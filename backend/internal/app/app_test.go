package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindFrontendDistDirUsesBuiltDirectoryFromEnv(t *testing.T) {
	t.Setenv("EMOSUP_FRONTEND_DIST", t.TempDir())

	distDir := os.Getenv("EMOSUP_FRONTEND_DIST")
	if err := os.WriteFile(filepath.Join(distDir, "index.html"), []byte("<!doctype html>"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.Mkdir(filepath.Join(distDir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}

	got := findFrontendDistDir()
	if got != distDir {
		t.Fatalf("findFrontendDistDir() = %q, want %q", got, distDir)
	}
}

func TestFindFrontendDistDirRejectsUnbuiltDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EMOSUP_FRONTEND_DIST", dir)

	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html>"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	got := findFrontendDistDir()
	if got != "" {
		t.Fatalf("findFrontendDistDir() = %q, want empty string", got)
	}
}

func TestResolveDataRootUsesRepoBackendDataFromWorkingDirectory(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("EMOSUP_DATA_DIR", "")

	if err := os.MkdirAll(filepath.Join(repoRoot, "backend"), 0o755); err != nil {
		t.Fatalf("mkdir backend: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "frontend"), 0o755); err != nil {
		t.Fatalf("mkdir frontend: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "backend", "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "frontend", "package.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(previousWD); chdirErr != nil {
			t.Fatalf("restore wd: %v", chdirErr)
		}
	})

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	got := resolveDataRoot()
	want := filepath.Join(repoRoot, "backend", "data")
	if got != want {
		t.Fatalf("resolveDataRoot() = %q, want %q", got, want)
	}
}
