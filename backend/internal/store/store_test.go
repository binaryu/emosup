package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"emosup/backend/internal/model"
)

func TestInitCreatesDBAndDownloadDir(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	s := NewFileStore(root)
	if err := s.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer s.Close()

	if _, err := os.Stat(filepath.Join(root, "emosup.db")); err != nil {
		t.Fatalf("db missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "downloads")); err != nil {
		t.Fatalf("downloads missing: %v", err)
	}

	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	want, _ := filepath.Abs(filepath.Join(root, "downloads"))
	if cfg.Download.Dir != want {
		t.Fatalf("download_dir=%q want %q", cfg.Download.Dir, want)
	}
}

func TestTaskProgressUpdateAndList(t *testing.T) {
	s := NewFileStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	now := time.Now()
	task := model.Task{
		ID:        "task_1",
		Status:    model.TaskStatusDownloading,
		Source:    model.TaskSource{FileName: "a.mkv", FileSize: 1000},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.SaveTask(task); err != nil {
		t.Fatal(err)
	}

	updated, err := s.UpdateTask("task_1", func(task *model.Task) error {
		task.Download.Progress = 42.5
		task.Download.Speed = 123456
		task.Download.CompletedBytes = 425
		task.Download.TotalBytes = 1000
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Download.Progress != 42.5 || updated.Download.Speed != 123456 {
		t.Fatalf("progress not updated: %+v", updated.Download)
	}

	got, err := s.GetTask("task_1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Download.Progress != 42.5 {
		t.Fatalf("get progress=%v", got.Download.Progress)
	}

	list, err := s.ListTasks()
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}
}

func TestDBReadsDoNotBlockOnHeldWriteTransaction(t *testing.T) {
	s := NewFileStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.DB().Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	// Hold the SQLite write transaction open. This simulates a stuck/long task
	// writer: reads must still complete from the WAL snapshot on another conn.
	if _, err := tx.Exec(`UPDATE config SET data = data WHERE id = 1`); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var raw string
	if err := s.DB().QueryRowContext(ctx, `SELECT data FROM config WHERE id = 1`).Scan(&raw); err != nil {
		t.Fatalf("read while write tx held: %v", err)
	}
	if raw == "" {
		t.Fatal("expected config row to be readable")
	}
}

func TestScanItemsCRUD(t *testing.T) {
	s := NewFileStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	now := time.Now()
	season, ep := 1, 2
	scan := model.ScanSession{
		ID: "scan_1", Path: "/tv", TMDBID: 9, Status: model.ScanSessionStatusCompleted,
		Items: []model.ScanItem{{
			ID: "item_1", ScanSessionID: "scan_1", FileName: "S01E02.mkv",
			IsVideo: true, MatchStatus: model.MatchStatusMatched,
			Parsed:    model.ParsedEpisodeInfo{Season: &season, Episode: &ep},
			CreatedAt: now, UpdatedAt: now,
		}},
		TotalCount: 1, MatchedCount: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.SaveScan(scan); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetScan("scan_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || got.Items[0].FileName != "S01E02.mkv" {
		t.Fatalf("items=%+v", got.Items)
	}
	if got.Items[0].Parsed.Episode == nil || *got.Items[0].Parsed.Episode != 2 {
		t.Fatalf("parsed=%+v", got.Items[0].Parsed)
	}

	_, err = s.UpdateScanItem("scan_1", "item_1", func(item *model.ScanItem) error {
		item.Confirmed = true
		item.SelectedTitle = "Ep 2"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetScan("scan_1")
	if !got.Items[0].Confirmed || got.Items[0].SelectedTitle != "Ep 2" {
		t.Fatalf("update failed: %+v", got.Items[0])
	}

	if err := s.AppendTaskLog("missing", model.TaskLogItem{Level: "info", Message: "x", Time: now}); err == nil {
		// may fail due to FK if task missing - that's ok for sqlite FK
	}

	// log against real task
	_ = s.SaveTask(model.Task{ID: "tlog", Status: model.TaskStatusQueued, CreatedAt: now, UpdatedAt: now})
	if err := s.AppendTaskLog("tlog", model.TaskLogItem{Level: "info", Message: "hello", Time: now}); err != nil {
		t.Fatal(err)
	}
	log, err := s.GetTaskLog("tlog")
	if err != nil || len(log.Items) != 1 || log.Items[0].Message != "hello" {
		t.Fatalf("log=%+v err=%v", log, err)
	}
}
