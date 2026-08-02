package service

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"emosup/backend/internal/store"
	"emosup/backend/internal/version"
)

// GitHubRelease mirrors the fields of the GitHub "latest release" API we need.
type GitHubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

type UpgradeCheck struct {
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	HasUpdate   bool   `json:"has_update"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
}

type UpgradeResult struct {
	Version string `json:"version"`
}

type UpgradeService struct {
	mu      sync.Mutex
	running bool
	store   *store.FileStore
}

func NewUpgradeService(store *store.FileStore) *UpgradeService {
	return &UpgradeService{store: store}
}

func (s *UpgradeService) repo() string {
	if repo := strings.TrimSpace(os.Getenv("EMOSUP_REPO")); repo != "" {
		return repo
	}
	return "binaryu/emosup"
}

// proxyPrefix mirrors the install.sh semantics: empty/"0" → no proxy,
// "1" → gh-proxy.com, otherwise treat the value as a URL prefix.
func proxyPrefix() string {
	p := strings.TrimSpace(os.Getenv("EMOSUP_PROXY"))
	switch {
	case p == "" || p == "0":
		return ""
	case p == "1":
		return "https://gh-proxy.com/"
	default:
		if strings.HasSuffix(p, "/") {
			return p
		}
		return p + "/"
	}
}

func (s *UpgradeService) githubAPIURL(path string) string {
	if prefix := proxyPrefix(); prefix != "" {
		return prefix + "https://api.github.com" + path
	}
	return "https://api.github.com" + path
}

func (s *UpgradeService) githubDownloadURL(path string) string {
	if prefix := proxyPrefix(); prefix != "" {
		return prefix + "https://github.com" + path
	}
	return "https://github.com" + path
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

// Check queries GitHub for the latest release and compares it with the running build.
func (s *UpgradeService) Check(ctx context.Context) (*UpgradeCheck, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		s.githubAPIURL(fmt.Sprintf("/repos/%s/releases/latest", s.repo())), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "emosup-upgrade/"+version.Version)

	resp, err := newHTTPClient(15*time.Second).Do(req)
	if err != nil {
		return nil, fmt.Errorf("检查更新失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("检查更新失败: GitHub 返回 %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&release); err != nil {
		return nil, fmt.Errorf("解析更新信息失败: %w", err)
	}

	latest := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	current := version.Version
	hasUpdate := latest != "" && (current == "" || current == "dev" || compareVersions(latest, current) > 0)

	return &UpgradeCheck{
		Current:     current,
		Latest:      latest,
		HasUpdate:   hasUpdate,
		Name:        release.Name,
		Body:        release.Body,
		PublishedAt: release.PublishedAt,
	}, nil
}

// compareVersions compares dot-separated numeric versions; returns <0, 0, >0.
func compareVersions(a, b string) int {
	pa := strings.Split(strings.TrimPrefix(strings.TrimSpace(a), "v"), ".")
	pb := strings.Split(strings.TrimPrefix(strings.TrimSpace(b), "v"), ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// Run downloads the release for the current platform, verifies its checksum,
// swaps program files (preserving data/ and emosup.env) and schedules a restart.
func (s *UpgradeService) Run(ctx context.Context) (*UpgradeResult, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil, errors.New("升级已在进行中，请稍候")
	}
	s.running = true
	defer func() { s.running = false }()
	s.mu.Unlock()

	if runtime.GOOS != "linux" {
		return nil, errors.New("自动升级仅支持 Linux 部署")
	}
	if isDockerContainer() {
		return nil, errors.New("Docker 部署请勿在面板内升级：请在宿主机执行 docker compose pull && docker compose up -d")
	}

	installDir, _, err := detectInstallLayout()
	if err != nil {
		return nil, err
	}

	check, err := s.Check(ctx)
	if err != nil {
		return nil, err
	}
	if !check.HasUpdate {
		return nil, fmt.Errorf("当前已是最新版本 (%s)", check.Latest)
	}

	tmpDir, err := os.MkdirTemp("", "emosup-upgrade-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	arch := runtime.GOARCH
	asset := fmt.Sprintf("emosup-linux-%s.tar.gz", arch)
	urlPath := fmt.Sprintf("/%s/releases/download/v%s/%s", s.repo(), check.Latest, asset)

	if err := downloadFile(ctx, s.githubDownloadURL(urlPath), filepath.Join(tmpDir, asset), 15*time.Minute); err != nil {
		return nil, fmt.Errorf("下载升级包失败: %w", err)
	}
	if err := downloadFile(ctx, s.githubDownloadURL(urlPath+".sha256"), filepath.Join(tmpDir, asset+".sha256"), 30*time.Second); err != nil {
		return nil, fmt.Errorf("下载校验文件失败: %w", err)
	}
	if err := verifySHA256(filepath.Join(tmpDir, asset), filepath.Join(tmpDir, asset+".sha256")); err != nil {
		return nil, err
	}

	extracted, err := extractTarGz(filepath.Join(tmpDir, asset), tmpDir)
	if err != nil {
		return nil, fmt.Errorf("解压升级包失败: %w", err)
	}

	if err := swapInstall(installDir, extracted, check.Latest, s.frontendDir(installDir)); err != nil {
		return nil, err
	}

	if err := scheduleRestart(installDir, os.Getpid()); err != nil {
		return nil, fmt.Errorf("升级文件已替换，但启动重启脚本失败: %w", err)
	}

	log.Printf("upgrade: %s → v%s installed at %s, restart scheduled", version.Version, check.Latest, installDir)
	return &UpgradeResult{Version: check.Latest}, nil
}

func (s *UpgradeService) frontendDir(installDir string) string {
	if env := strings.TrimSpace(os.Getenv("EMOSUP_FRONTEND_DIST")); env != "" {
		return env
	}
	return filepath.Join(installDir, "frontend")
}

func isDockerContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if os.Getenv("EMOSUP_FRONTEND_DIST") == "/app/frontend" {
		return true
	}
	return false
}

// detectInstallLayout verifies this is a release-style install and returns the
// install directory and the resolved executable path.
func detectInstallLayout() (installDir, exe string, err error) {
	exe, err = os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("无法定位可执行文件: %w", err)
	}
	if resolved, rerr := filepath.EvalSymlinks(exe); rerr == nil {
		exe = resolved
	}
	installDir = filepath.Dir(exe)

	if !fileExists(filepath.Join(installDir, "emosup-server")) {
		return "", "", errors.New("未检测到 release 安装布局（缺少 emosup-server），仅支持 install.sh 部署的目录自动升级")
	}
	frontend := filepath.Join(installDir, "frontend")
	hasEnv := fileExists(filepath.Join(installDir, "emosup.env"))
	if !isFrontendBuildDir(frontend) && !hasEnv {
		return "", "", errors.New("未检测到 release 安装布局（缺少 frontend/ 或 emosup.env），开发环境请手动升级")
	}
	return installDir, exe, nil
}

func isFrontendBuildDir(dir string) bool {
	index, err1 := os.Stat(filepath.Join(dir, "index.html"))
	assets, err2 := os.Stat(filepath.Join(dir, "assets"))
	return err1 == nil && err2 == nil && !index.IsDir() && assets.IsDir()
}

func downloadFile(ctx context.Context, url, dest string, timeout time.Duration) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "emosup-upgrade/"+version.Version)

	resp, err := newHTTPClient(timeout).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d (%s)", resp.StatusCode, url)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func verifySHA256(filePath, shaPath string) error {
	wantBytes, err := os.ReadFile(shaPath)
	if err != nil {
		return fmt.Errorf("读取校验文件失败: %w", err)
	}
	fields := strings.Fields(string(wantBytes))
	if len(fields) == 0 {
		return errors.New("校验文件格式错误")
	}
	want := strings.ToLower(strings.TrimSpace(fields[0]))

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	got := hex.EncodeToString(hash.Sum(nil))
	if got != want {
		return fmt.Errorf("SHA256 校验失败：got %s want %s", got, want)
	}
	return nil
}

// extractTarGz extracts into parentDir and returns the root of the extracted
// directory tree (the tarball contains a top-level emosup-linux-* folder).
func extractTarGz(archivePath, parentDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	root := ""
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		name := filepath.Clean(hdr.Name)
		if name == "." || strings.HasPrefix(name, ".."+string(filepath.Separator)) || filepath.IsAbs(name) {
			return "", fmt.Errorf("非法路径: %s", hdr.Name)
		}
		target := filepath.Join(parentDir, name)
		if !strings.HasPrefix(target, parentDir+string(filepath.Separator)) {
			return "", fmt.Errorf("非法路径: %s", hdr.Name)
		}
		if root == "" {
			root = filepath.Join(parentDir, strings.SplitN(name, string(filepath.Separator), 2)[0])
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return "", err
			}
			out.Close()
		}
	}
	if root == "" {
		return "", errors.New("升级包内容为空")
	}
	return root, nil
}

// swapInstall replaces program files while preserving data/ and emosup.env.
func swapInstall(installDir, extracted, newVersion, frontendDir string) error {
	// 1. Move data aside (same filesystem → instant, no cross-device copy).
	dataPath := filepath.Join(installDir, "data")
	dataBackup := filepath.Join(installDir, ".upgrade-data-bak")
	if dirExists(dataPath) {
		if err := os.Rename(dataPath, dataBackup); err != nil {
			return fmt.Errorf("备份 data/ 失败: %w", err)
		}
	}

	// 2. Wipe everything except the data backup and the env file.
	entries, err := os.ReadDir(installDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == ".upgrade-data-bak" || name == "emosup.env" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(installDir, name)); err != nil {
			return fmt.Errorf("清理安装目录失败: %w", err)
		}
	}

	// 3. Copy the new tree in.
	if err := copyTree(extracted, installDir); err != nil {
		return fmt.Errorf("写入新版本文件失败: %w", err)
	}

	// 4. Restore data (replace the empty data/ shipped in the tarball).
	_ = os.RemoveAll(dataPath)
	if dirExists(dataBackup) {
		if err := os.Rename(dataBackup, dataPath); err != nil {
			return fmt.Errorf("恢复 data/ 失败: %w", err)
		}
	}

	// 5. Custom frontend dir (e.g. EMOSUP_FRONTEND_DIST) gets the new assets too.
	if strings.TrimSpace(frontendDir) != "" && filepath.Clean(frontendDir) != filepath.Join(installDir, "frontend") {
		srcFrontend := filepath.Join(installDir, "frontend")
		if isFrontendBuildDir(srcFrontend) {
			if err := os.MkdirAll(frontendDir, 0o755); err != nil {
				return err
			}
			if err := replaceDirContents(frontendDir, srcFrontend); err != nil {
				return fmt.Errorf("更新自定义前端目录失败: %w", err)
			}
		}
	}

	// 6. Record the version.
	if err := os.WriteFile(filepath.Join(installDir, "VERSION"), []byte("v"+newVersion+"\n"), 0o644); err != nil {
		log.Printf("upgrade: 写入 VERSION 失败: %v", err)
	}

	if err := os.Chmod(filepath.Join(installDir, "emosup-server"), 0o755); err != nil {
		return fmt.Errorf("设置可执行权限失败: %w", err)
	}
	return nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if d.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		info, err := in.Stat()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}

func replaceDirContents(dir, src string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return copyTree(src, dir)
}

// scheduleRestart spawns a detached script that waits for this process to exit
// and then restarts via systemd (preferred) or re-execs the binary directly.
func scheduleRestart(installDir string, pid int) error {
	script := `#!/bin/sh
PID="$1"; DIR="$2"
i=0
while kill -0 "$PID" 2>/dev/null && [ "$i" -lt 60 ]; do sleep 1; i=$((i+1)); done
kill -9 "$PID" 2>/dev/null || true
if command -v systemctl >/dev/null 2>&1 && systemctl cat emosup >/dev/null 2>&1; then
  systemctl restart emosup >/dev/null 2>&1
else
  cd "$DIR" || exit 1
  nohup ./emosup-server >/dev/null 2>&1 &
fi
`
	scriptPath := filepath.Join(os.TempDir(), fmt.Sprintf("emosup-restart-%d.sh", pid))
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		return err
	}

	cmd := exec.Command("setsid", "-f", "/bin/sh", scriptPath, strconv.Itoa(pid), installDir)
	if err := cmd.Start(); err != nil {
		// setsid may be unavailable; fall back to a detached background process.
		cmd = exec.Command("/bin/sh", "-c",
			fmt.Sprintf("nohup /bin/sh %s %d %q >/dev/null 2>&1 &", scriptPath, pid, installDir))
		if err2 := cmd.Start(); err2 != nil {
			return err
		}
	}
	return nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
