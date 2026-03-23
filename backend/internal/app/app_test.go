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
