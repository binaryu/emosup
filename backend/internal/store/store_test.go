package store

import (
	"context"
	"database/sql"
	"fmt"
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

func TestDeleteScanItemsBatch(t *testing.T) {
	s := NewFileStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	now := time.Now()
	items := make([]model.ScanItem, 0, 3)
	for i := 1; i <= 3; i++ {
		id := fmt.Sprintf("item_%d", i)
		status := model.MatchStatusUnmatched
		if i == 1 {
			status = model.MatchStatusMatched
		}
		items = append(items, model.ScanItem{
			ID: id, ScanSessionID: "scan_b", FileName: id, IsVideo: true,
			MatchStatus: status, CreatedAt: now, UpdatedAt: now,
		})
	}
	if err := s.SaveScan(model.ScanSession{
		ID: "scan_b", Path: "/tv", TMDBID: 9, Status: model.ScanSessionStatusCompleted,
		Items: items, TotalCount: 3, MatchedCount: 1, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	scan, err := s.DeleteScanItems("scan_b", []string{"item_1", "item_3"})
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Items) != 1 || scan.Items[0].ID != "item_2" {
		t.Fatalf("items after batch delete: %+v", scan.Items)
	}
	if scan.TotalCount != 1 || scan.MatchedCount != 0 || scan.UnmatchedCount != 1 {
		t.Fatalf("counts after batch delete: total=%d matched=%d unmatched=%d",
			scan.TotalCount, scan.MatchedCount, scan.UnmatchedCount)
	}

	// Deleting the last item empties the scan.
	scan, err = s.DeleteScanItems("scan_b", []string{"item_2"})
	if err != nil {
		t.Fatal(err)
	}
	if scan.TotalCount != 0 || len(scan.Items) != 0 {
		t.Fatalf("expected empty scan, got total=%d items=%d", scan.TotalCount, len(scan.Items))
	}

	// Unknown ids error out.
	if _, err := s.DeleteScanItems("scan_b", []string{"nope"}); err == nil {
		t.Fatal("expected error deleting unknown items")
	}
	if _, err := s.DeleteScanItems("scan_b", nil); err == nil {
		t.Fatal("expected error deleting empty id list")
	}
}

func TestMigrateAddsKeepLocalFileColumn(t *testing.T) {
	root := t.TempDir()
	// Create a legacy DB with the old tasks schema (no keep_local_file).
	dbPath := filepath.Join(root, "emosup.db")
	legacy, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(`CREATE TABLE tasks (
	  id TEXT PRIMARY KEY,
	  scan_session_id TEXT NOT NULL DEFAULT '',
	  scan_item_id TEXT NOT NULL DEFAULT '',
	  status TEXT NOT NULL DEFAULT '',
	  paused INTEGER NOT NULL DEFAULT 0,
	  retry_count INTEGER NOT NULL DEFAULT 0,
	  source_type TEXT NOT NULL DEFAULT '',
	  source_path TEXT NOT NULL DEFAULT '',
	  source_raw_url TEXT NOT NULL DEFAULT '',
	  source_file_name TEXT NOT NULL DEFAULT '',
	  source_file_size INTEGER NOT NULL DEFAULT 0,
	  parsed_season INTEGER, parsed_episode INTEGER, parsed_is_special INTEGER NOT NULL DEFAULT 0,
	  target_tmdb_id INTEGER NOT NULL DEFAULT 0,
	  target_item_type TEXT NOT NULL DEFAULT '',
	  target_item_id INTEGER NOT NULL DEFAULT 0,
	  target_title TEXT NOT NULL DEFAULT '',
	  dl_save_dir TEXT NOT NULL DEFAULT '',
	  dl_local_path TEXT NOT NULL DEFAULT '',
	  dl_status TEXT NOT NULL DEFAULT '',
	  dl_total_bytes INTEGER NOT NULL DEFAULT 0,
	  dl_completed_bytes INTEGER NOT NULL DEFAULT 0,
	  dl_progress REAL NOT NULL DEFAULT 0,
	  dl_speed INTEGER NOT NULL DEFAULT 0,
	  ul_storage TEXT NOT NULL DEFAULT '',
	  ul_file_id TEXT NOT NULL DEFAULT '',
	  ul_upload_url TEXT NOT NULL DEFAULT '',
	  ul_upload_type TEXT NOT NULL DEFAULT '',
	  ul_multipart_size_min INTEGER NOT NULL DEFAULT 0,
	  ul_multipart_size_max INTEGER NOT NULL DEFAULT 0,
	  ul_multipart_presigns TEXT NOT NULL DEFAULT '[]',
	  ul_multipart_parts TEXT NOT NULL DEFAULT '[]',
	  ul_media_id TEXT NOT NULL DEFAULT '',
	  ul_total_bytes INTEGER NOT NULL DEFAULT 0,
	  ul_uploaded_bytes INTEGER NOT NULL DEFAULT 0,
	  ul_progress REAL NOT NULL DEFAULT 0,
	  ul_speed INTEGER NOT NULL DEFAULT 0,
	  ul_status TEXT NOT NULL DEFAULT '',
	  ul_save_retry_count INTEGER NOT NULL DEFAULT 0,
	  ul_last_save_error TEXT NOT NULL DEFAULT '',
	  result_error_message TEXT NOT NULL DEFAULT '',
	  result_error_stage TEXT NOT NULL DEFAULT '',
	  result_error_code TEXT NOT NULL DEFAULT '',
	  result_last_error_at TEXT,
	  created_at TEXT NOT NULL,
	  updated_at TEXT NOT NULL,
	  finished_at TEXT
	)`); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	s := NewFileStore(root)
	if err := s.Init(); err != nil {
		t.Fatalf("init with legacy db: %v", err)
	}
	defer s.Close()

	// Insert a legacy task row without keep_local_file, then verify it loads.
	if _, err := s.db.Exec(`INSERT INTO tasks(
	  id, status, source_file_name, created_at, updated_at
	) VALUES ('task_legacy', 'completed', 'old.mkv', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	task, err := s.GetTask("task_legacy")
	if err != nil {
		t.Fatal(err)
	}
	if task.KeepLocalFile {
		t.Fatal("legacy task should default keep_local_file=false")
	}

	// And a new task round-trips the flag.
	now := time.Now()
	saved := model.Task{
		ID:            "task_keep",
		Status:        model.TaskStatusCompleted,
		KeepLocalFile: true,
		Source:        model.TaskSource{FileName: "kept.mkv"},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.SaveTask(saved); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTask("task_keep")
	if err != nil {
		t.Fatal(err)
	}
	if !got.KeepLocalFile {
		t.Fatal("keep_local_file did not round-trip")
	}
}
