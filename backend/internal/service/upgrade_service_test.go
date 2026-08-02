package service

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.2.0", "v1.2.1", -1},
		{"1.2.1", "v1.2.0", 1},
		{"v1.2.1", "v1.2.1", 0},
		{"v1.9.0", "v1.10.0", -1},
		{"v2.0.0", "v1.99.99", 1},
		{"v1.2.1-beta.1", "v1.2.1", -1},
		{"v1.2", "v1.2.0", 0},
	}
	for _, c := range cases {
		got := compareVersions(c.a, c.b)
		if (got < 0) != (c.want < 0) || (got > 0) != (c.want > 0) || (got == 0) != (c.want == 0) {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func makeTarGz(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pkg.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		content := files[name]
		hdr := &tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractTarGz(t *testing.T) {
	pkg := makeTarGz(t, map[string]string{
		"emosup-linux-amd64/emosup-server":       "new-binary",
		"emosup-linux-amd64/frontend/index.html": "ok",
		"emosup-linux-amd64/data/downloads/":     "",
	})
	// directories come implicitly; ensure one explicit dir entry
	root, err := extractTarGz(pkg, t.TempDir())
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !fileExists(filepath.Join(root, "emosup-server")) {
		t.Fatalf("missing emosup-server in %s", root)
	}
	if !strings.HasSuffix(root, "emosup-linux-amd64") {
		t.Fatalf("root = %q, want .../emosup-linux-amd64", root)
	}
}

func TestExtractTarGzRejectsTraversal(t *testing.T) {
	pkg := makeTarGz(t, map[string]string{
		"../evil": "x",
	})
	if _, err := extractTarGz(pkg, t.TempDir()); err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestVerifySHA256(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "pkg")
	content := "hello upgrade"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	sha := filepath.Join(dir, "pkg.sha256")
	if err := os.WriteFile(sha, []byte(fmt.Sprintf("%x  pkg\n", sum)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifySHA256(file, sha); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := os.WriteFile(file, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifySHA256(file, sha); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func TestSwapInstallPreservesDataAndEnv(t *testing.T) {
	installDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(installDir, "data", "downloads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "data", "emosup.db"), []byte("db"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "emosup.env"), []byte("EMOSUP_PORT=8080\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "emosup-server"), []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "stale.txt"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}

	extracted := filepath.Join(t.TempDir(), "emosup-linux-amd64")
	if err := os.MkdirAll(filepath.Join(extracted, "frontend", "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extracted, "emosup-server"), []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extracted, "frontend", "index.html"), []byte("html"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := swapInstall(installDir, extracted, "1.2.1", filepath.Join(installDir, "frontend")); err != nil {
		t.Fatalf("swapInstall: %v", err)
	}

	bin, err := os.ReadFile(filepath.Join(installDir, "emosup-server"))
	if err != nil || string(bin) != "new" {
		t.Fatalf("binary not replaced: %q err=%v", bin, err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "data", "emosup.db")); err != nil {
		t.Fatalf("data/ lost: %v", err)
	}
	env, err := os.ReadFile(filepath.Join(installDir, "emosup.env"))
	if err != nil || string(env) != "EMOSUP_PORT=8080\n" {
		t.Fatalf("emosup.env lost: %q err=%v", env, err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "stale.txt")); !os.IsNotExist(err) {
		t.Fatal("stale file should have been removed")
	}
	if _, err := os.Stat(filepath.Join(installDir, "frontend", "index.html")); err != nil {
		t.Fatalf("frontend not copied: %v", err)
	}
	ver, err := os.ReadFile(filepath.Join(installDir, "VERSION"))
	if err != nil || strings.TrimSpace(string(ver)) != "v1.2.1" {
		t.Fatalf("VERSION = %q err=%v", ver, err)
	}
}

func TestParseSystemdUnitFromCgroup(t *testing.T) {
	cases := []struct {
		cgroup string
		want   string
	}{
		{"0::/system.slice/emosup.service\n", "emosup.service"},
		{"0::/system.slice/fn_emosup.service\n", "fn_emosup.service"},
		{"12:freezer:/system.slice/docker-abc.scope\n0::/system.slice/emosup.service\n", "emosup.service"},
		{"0::/\n", ""},
		{"0::/user.slice/user-1000.slice/session-1.scope\n", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := parseSystemdUnitFromCgroup(c.cgroup); got != c.want {
			t.Errorf("parseSystemdUnitFromCgroup(%q) = %q, want %q", c.cgroup, got, c.want)
		}
	}
}

func TestBuildRestartScript(t *testing.T) {
	script := buildRestartScript("/tmp/emosup-upgrade.log")
	for _, want := range []string{
		"LOG='/tmp/emosup-upgrade.log'",
		"systemctl restart \"$UNIT\"",
		"systemctl start \"$UNIT\"",
		"nohup ./emosup-server",
		"restart script finished",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("restart script missing %q", want)
		}
	}
}
