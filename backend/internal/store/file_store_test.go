package store

import (
	"path/filepath"
	"testing"
)

func TestInitCreatesDefaultConfigWithDataScopedDownloadDir(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "data")
	fileStore := NewFileStore(root)

	if err := fileStore.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}

	cfg, err := fileStore.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	wantDownloadDir := filepath.Join(root, "downloads")
	if cfg.Aria2.DownloadDir != wantDownloadDir {
		t.Fatalf("download_dir = %q, want %q", cfg.Aria2.DownloadDir, wantDownloadDir)
	}
}
