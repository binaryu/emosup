package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"emosup/backend/internal/model"
	"emosup/backend/internal/store"
	"emosup/backend/internal/utils"
)

func newCacheTestStore(t *testing.T) (*store.FileStore, string) {
	t.Helper()
	root := t.TempDir()
	fileStore := store.NewFileStore(root)
	if err := fileStore.Init(); err != nil {
		t.Fatalf("store init failed: %v", err)
	}
	downloadDir := filepath.Join(root, "downloads")
	if err := utils.EnsureDir(downloadDir); err != nil {
		t.Fatalf("mkdir downloads: %v", err)
	}

	cfg, err := fileStore.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Download.Dir = downloadDir
	if err := fileStore.SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	return fileStore, downloadDir
}

func TestCacheListAndDelete(t *testing.T) {
	fileStore, downloadDir := newCacheTestStore(t)
	svc := NewCacheService(fileStore)

	// Orphan file: not referenced by any task.
	orphan := filepath.Join(downloadDir, "orphan__old.mkv")
	if err := os.WriteFile(orphan, []byte("orphan-data"), 0o644); err != nil {
		t.Fatalf("write orphan: %v", err)
	}

	// Referenced file: owned by a completed keep-local task.
	task := model.Task{
		ID:            utils.NewID("task"),
		Status:        model.TaskStatusCompleted,
		KeepLocalFile: true,
		Source:        model.TaskSource{FileName: "kept.mkv"},
		Download:      model.TaskDownload{LocalPath: filepath.Join(downloadDir, "task1__kept.mkv")},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := fileStore.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}
	kept := filepath.Join(downloadDir, "task1__kept.mkv")
	if err := os.WriteFile(kept, []byte("kept-data"), 0o644); err != nil {
		t.Fatalf("write kept: %v", err)
	}

	// Active task: must be refused on delete.
	activeTask := model.Task{
		ID:        utils.NewID("task"),
		Status:    model.TaskStatusUploading,
		Source:    model.TaskSource{FileName: "active.mkv"},
		Download:  model.TaskDownload{LocalPath: filepath.Join(downloadDir, "task2__active.mkv")},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := fileStore.SaveTask(activeTask); err != nil {
		t.Fatalf("save active task: %v", err)
	}
	active := filepath.Join(downloadDir, "task2__active.mkv")
	if err := os.WriteFile(active, []byte("active-data"), 0o644); err != nil {
		t.Fatalf("write active: %v", err)
	}

	// Temp part file.
	part := filepath.Join(downloadDir, "task1__kept.mkv.partmulti")
	if err := os.WriteFile(part, []byte("part"), 0o644); err != nil {
		t.Fatalf("write part: %v", err)
	}

	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if list.OrphanCount != 2 { // orphan file + partmulti temp
		t.Fatalf("orphan count = %d, want 2", list.OrphanCount)
	}
	if list.ActiveRefCount != 1 {
		t.Fatalf("active ref count = %d, want 1", list.ActiveRefCount)
	}

	byName := make(map[string]CacheEntry)
	for _, e := range list.Entries {
		byName[e.Name] = e
	}
	if e := byName["task1__kept.mkv"]; !e.Referenced || !e.KeepLocalFile || e.TaskID != task.ID {
		t.Fatalf("kept file entry wrong: %+v", e)
	}
	if e := byName["task2__active.mkv"]; !e.Referenced || e.TaskStatus != string(model.TaskStatusUploading) {
		t.Fatalf("active file entry wrong: %+v", e)
	}
	if e := byName["orphan__old.mkv"]; e.Referenced {
		t.Fatalf("orphan file should not be referenced: %+v", e)
	}
	if e := byName["task1__kept.mkv.partmulti"]; !e.IsTemp || e.Referenced {
		t.Fatalf("temp part entry wrong: %+v", e)
	}

	// Delete outside cache dir must fail.
	outside := filepath.Join(t.TempDir(), "evil.mkv")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	deleted, failed, err := svc.Delete(context.Background(), []string{outside})
	if err != nil {
		t.Fatalf("delete outside: %v", err)
	}
	if len(deleted) != 0 || len(failed) != 1 {
		t.Fatalf("outside delete: deleted=%v failed=%v", deleted, failed)
	}

	// Delete active-referenced file must be refused.
	deleted, failed, err = svc.Delete(context.Background(), []string{active})
	if err != nil {
		t.Fatalf("delete active: %v", err)
	}
	if len(deleted) != 0 || len(failed) != 1 {
		t.Fatalf("active delete: deleted=%v failed=%v", deleted, failed)
	}
	if !strings.Contains(failed[active], "使用中") {
		t.Fatalf("active delete reason = %q, want 使用中", failed[active])
	}

	// Delete orphan + completed-keep + temp files.
	deleted, failed, err = svc.Delete(context.Background(), []string{orphan, kept, part})
	if err != nil {
		t.Fatalf("delete batch: %v", err)
	}
	if len(deleted) != 3 || len(failed) != 0 {
		t.Fatalf("batch delete: deleted=%v failed=%v", deleted, failed)
	}
	for _, p := range []string{orphan, kept, part} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("file still exists after delete: %s", p)
		}
	}
}
