package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"emosup/backend/internal/store"
	"emosup/backend/internal/utils"
)

type LocalEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

type LocalService struct {
	store *store.FileStore
}

func NewLocalService(store *store.FileStore) *LocalService {
	return &LocalService{store: store}
}

// Root returns the absolute path used for local media browsing.
//
// Priority:
//  1. EMOSUP_LOCAL_ROOT (env override, e.g. Docker /media)
//  2. config.local.root (settings UI / config file)
//  3. config.aria2.download_dir
//  4. {dataRoot}/downloads
func (s *LocalService) Root() string {
	localRoot, downloadDir, dataRoot := "", "", ""
	if s.store != nil {
		dataRoot = s.store.Root()
		if cfg, err := s.store.LoadConfig(); err == nil {
			localRoot = strings.TrimSpace(cfg.Local.Root)
			downloadDir = strings.TrimSpace(cfg.Aria2.DownloadDir)
		}
	}
	return ResolveLocalMediaRoot(localRoot, downloadDir, dataRoot)
}

// ResolveLocalMediaRoot is shared by task creation and local browse.
func ResolveLocalMediaRoot(localRoot, downloadDir, dataRoot string) string {
	if root := strings.TrimSpace(os.Getenv("EMOSUP_LOCAL_ROOT")); root != "" {
		return absPath(root)
	}
	if dir := strings.TrimSpace(localRoot); dir != "" {
		// Relative paths are anchored to data root when possible.
		if !filepath.IsAbs(dir) && strings.TrimSpace(dataRoot) != "" {
			return absPath(filepath.Join(dataRoot, dir))
		}
		return absPath(dir)
	}
	if dir := strings.TrimSpace(downloadDir); dir != "" {
		return absPath(dir)
	}
	if root := strings.TrimSpace(dataRoot); root != "" {
		return absPath(filepath.Join(root, "downloads"))
	}
	return absPath(filepath.Join("data", "downloads"))
}

func absPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return filepath.Clean(path)
}

func (s *LocalService) resolve(relPath string) (absBase, fullPath, cleanRel string, err error) {
	absBase = s.Root()
	if absBase == "" {
		return "", "", "", errors.New("local media root is not configured")
	}

	if err := s.ensureRootAccessible(absBase); err != nil {
		return "", "", "", err
	}

	cleanRel = filepath.Clean(strings.TrimPrefix(filepath.Clean(relPath), "/"))
	if cleanRel == "." {
		cleanRel = ""
	}
	fullPath = filepath.Join(absBase, cleanRel)

	// Prevent directory traversal
	cleanedFull := filepath.Clean(fullPath)
	if rel, err := filepath.Rel(filepath.Clean(absBase), cleanedFull); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", "", errors.New("access denied")
	}
	return absBase, fullPath, cleanRel, nil
}

// ensureRootAccessible creates default data/downloads if missing; custom paths must already exist.
func (s *LocalService) ensureRootAccessible(absBase string) error {
	info, err := os.Stat(absBase)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("本地目录不是文件夹: %s", absBase)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// Auto-create only when root sits under the app data directory (default downloads).
	dataRoot := ""
	if s.store != nil {
		dataRoot = s.store.Root()
	}
	if dataRoot != "" {
		dataAbs := absPath(dataRoot)
		if absBase == dataAbs || strings.HasPrefix(absBase+string(filepath.Separator), dataAbs+string(filepath.Separator)) {
			if mkErr := utils.EnsureDir(absBase); mkErr != nil {
				return fmt.Errorf("创建本地目录失败 %s: %w", absBase, mkErr)
			}
			return nil
		}
	}

	return fmt.Errorf("本地目录不存在: %s（请在「系统配置 → 本地媒体」填写已存在的绝对路径）", absBase)
}

func (s *LocalService) IsDir(_ context.Context, relPath string) bool {
	_, fullPath, _, err := s.resolve(relPath)
	if err != nil {
		return false
	}
	info, err := os.Stat(fullPath)
	return err == nil && info.IsDir()
}

func (s *LocalService) GetFileInfo(_ context.Context, relPath string) (LocalEntry, error) {
	_, fullPath, cleanRel, err := s.resolve(relPath)
	if err != nil {
		return LocalEntry{}, err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return LocalEntry{}, err
	}
	if info.IsDir() {
		return LocalEntry{}, errors.New("path is a directory, not a file")
	}

	display := "/"
	if cleanRel != "" {
		display = "/" + filepath.ToSlash(cleanRel)
	}

	return LocalEntry{
		Name:  info.Name(),
		Path:  display,
		IsDir: false,
		Size:  info.Size(),
	}, nil
}

func (s *LocalService) Browse(_ context.Context, relPath string) (string, []LocalEntry, error) {
	_, fullPath, cleanRel, err := s.resolve(relPath)
	if err != nil {
		return "", nil, err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return "", nil, err
	}
	if !info.IsDir() {
		return "", nil, fmt.Errorf("not a directory: %s", relPath)
	}

	dirEntries, err := os.ReadDir(fullPath)
	if err != nil {
		return "", nil, err
	}

	result := make([]LocalEntry, 0, len(dirEntries))
	for _, entry := range dirEntries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}

		entryPath := entry.Name()
		if cleanRel != "" {
			entryPath = filepath.Join(cleanRel, entry.Name())
		}

		result = append(result, LocalEntry{
			Name:  entry.Name(),
			Path:  "/" + filepath.ToSlash(entryPath),
			IsDir: entry.IsDir(),
			Size:  info.Size(),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	displayPath := "/"
	if cleanRel != "" {
		displayPath = "/" + filepath.ToSlash(cleanRel)
	}

	return displayPath, result, nil
}
