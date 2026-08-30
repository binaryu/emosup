package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"emosup/backend/internal/model"
	"emosup/backend/internal/utils"

	_ "modernc.org/sqlite"
)

// FileStore is the persistence layer (SQLite under the hood).
// The name is kept for call-site compatibility after the JSON→SQLite refactor.
type FileStore struct {
	root    string
	db      *sql.DB
	writeMu sync.Mutex
}

func NewFileStore(root string) *FileStore {
	return &FileStore{root: root}
}

func (s *FileStore) Root() string {
	return s.root
}

func (s *FileStore) DB() *sql.DB {
	return s.db
}

type writeTx struct {
	*sql.Tx
	unlockOnce sync.Once
	unlock     func()
}

func (s *FileStore) beginWrite(ctx context.Context) (*writeTx, error) {
	s.writeMu.Lock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.writeMu.Unlock()
		return nil, err
	}
	return &writeTx{Tx: tx, unlock: s.writeMu.Unlock}, nil
}

func (tx *writeTx) Commit() error {
	err := tx.Tx.Commit()
	tx.unlockOnce.Do(tx.unlock)
	return err
}

func (tx *writeTx) Rollback() error {
	err := tx.Tx.Rollback()
	tx.unlockOnce.Do(tx.unlock)
	return err
}

func (s *FileStore) Init() error {
	dirs := []string{
		s.root,
		filepath.Join(s.root, "downloads"),
	}
	for _, dir := range dirs {
		if err := utils.EnsureDir(dir); err != nil {
			return err
		}
	}

	dbPath := filepath.Join(s.root, "emosup.db")
	// WAL + busy timeout: concurrent progress writers won't block readers as hard as JSON files.
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite still has a single writer, but a long-lived writer transaction must
	// not freeze unrelated reads (login, config, task list). WAL allows readers to
	// see the last committed snapshot while the writer connection is busy.
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(8)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return fmt.Errorf("ping sqlite: %w", err)
	}
	s.db = db

	if err := s.migrate(); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	// Clean up stale or failed scans that have no items so they don't pollute the UI
	_, _ = s.db.Exec(`DELETE FROM scans WHERE total_count = 0 OR status = 'failed' OR id NOT IN (SELECT DISTINCT scan_session_id FROM scan_items)`)
	if err := s.ensureConfig(); err != nil {
		return err
	}

	cfg, err := s.LoadConfig()
	if err != nil {
		return err
	}
	if dir := strings.TrimSpace(cfg.Download.Dir); dir != "" {
		if err := utils.EnsureDir(dir); err != nil {
			return fmt.Errorf("ensure download dir %s: %w", dir, err)
		}
	}
	return nil
}

func (s *FileStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *FileStore) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS config (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  data TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS scans (
  id TEXT PRIMARY KEY,
  source TEXT NOT NULL DEFAULT '',
  path TEXT NOT NULL DEFAULT '',
  tmdb_id INTEGER NOT NULL DEFAULT 0,
  video_type TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  total_count INTEGER NOT NULL DEFAULT 0,
  matched_count INTEGER NOT NULL DEFAULT 0,
  unmatched_count INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS scan_items (
  id TEXT PRIMARY KEY,
  scan_session_id TEXT NOT NULL,
  openlist_path TEXT NOT NULL DEFAULT '',
  file_name TEXT NOT NULL DEFAULT '',
  file_size INTEGER NOT NULL DEFAULT 0,
  raw_url TEXT NOT NULL DEFAULT '',
  is_video INTEGER NOT NULL DEFAULT 1,
  has_media INTEGER, -- NULL / 0 / 1
  parsed_json TEXT NOT NULL DEFAULT '{}',
  match_status TEXT NOT NULL DEFAULT '',
  match_reason TEXT NOT NULL DEFAULT '',
  match_candidates_json TEXT NOT NULL DEFAULT '[]',
  selected_item_type TEXT NOT NULL DEFAULT '',
  selected_item_id INTEGER NOT NULL DEFAULT 0,
  selected_title TEXT NOT NULL DEFAULT '',
  confirmed INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (scan_session_id) REFERENCES scans(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_scan_items_scan ON scan_items(scan_session_id);

CREATE TABLE IF NOT EXISTS tasks (
  id TEXT PRIMARY KEY,
  scan_session_id TEXT NOT NULL DEFAULT '',
  scan_item_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  paused INTEGER NOT NULL DEFAULT 0,
  retry_count INTEGER NOT NULL DEFAULT 0,
  keep_local_file INTEGER NOT NULL DEFAULT 0,
  source_type TEXT NOT NULL DEFAULT '',
  source_path TEXT NOT NULL DEFAULT '',
  source_raw_url TEXT NOT NULL DEFAULT '',
  source_file_name TEXT NOT NULL DEFAULT '',
  source_file_size INTEGER NOT NULL DEFAULT 0,
  parsed_season INTEGER,
  parsed_episode INTEGER,
  parsed_is_special INTEGER NOT NULL DEFAULT 0,
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
);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_created ON tasks(created_at DESC);

CREATE TABLE IF NOT EXISTS task_logs (
  id TEXT PRIMARY KEY,
  task_id TEXT NOT NULL,
  level TEXT NOT NULL DEFAULT 'info',
  message TEXT NOT NULL DEFAULT '',
  time TEXT NOT NULL,
  FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_task_logs_task ON task_logs(task_id, time);
`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	if err := s.migrateTaskColumns(); err != nil {
		return err
	}
	if err := s.dropLegacyColumns(); err != nil {
		return err
	}
	return nil
}

// dropLegacyColumns removes columns from old schemas that are no longer used.
func (s *FileStore) dropLegacyColumns() error {
	for _, column := range []string{"dl_aria2_gid"} {
		exists, err := s.columnExists("tasks", column)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		if _, err := s.db.Exec("ALTER TABLE tasks DROP COLUMN " + column); err != nil {
			return err
		}
	}
	return nil
}

func (s *FileStore) migrateTaskColumns() error {
	columns := []struct {
		name string
		ddl  string
	}{
		{name: "ul_upload_type", ddl: "ALTER TABLE tasks ADD COLUMN ul_upload_type TEXT NOT NULL DEFAULT ''"},
		{name: "ul_multipart_size_min", ddl: "ALTER TABLE tasks ADD COLUMN ul_multipart_size_min INTEGER NOT NULL DEFAULT 0"},
		{name: "ul_multipart_size_max", ddl: "ALTER TABLE tasks ADD COLUMN ul_multipart_size_max INTEGER NOT NULL DEFAULT 0"},
		{name: "ul_multipart_presigns", ddl: "ALTER TABLE tasks ADD COLUMN ul_multipart_presigns TEXT NOT NULL DEFAULT '[]'"},
		{name: "ul_multipart_parts", ddl: "ALTER TABLE tasks ADD COLUMN ul_multipart_parts TEXT NOT NULL DEFAULT '[]'"},
		{name: "keep_local_file", ddl: "ALTER TABLE tasks ADD COLUMN keep_local_file INTEGER NOT NULL DEFAULT 0"},
	}
	for _, column := range columns {
		exists, err := s.columnExists("tasks", column.name)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := s.db.Exec(column.ddl); err != nil {
			return err
		}
	}
	return nil
}

func (s *FileStore) columnExists(table, column string) (bool, error) {
	rows, err := s.db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull, pk int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

// ---------- config ----------

func (s *FileStore) LoadConfig() (model.AppConfig, error) {
	var raw string
	err := s.db.QueryRow(`SELECT data FROM config WHERE id = 1`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return s.defaultConfig(), nil
	}
	if err != nil {
		return model.AppConfig{}, err
	}
	var cfg model.AppConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return model.AppConfig{}, err
	}
	// Legacy migration: pre-v1.2.3 configs stored the cache dir under the
	// (removed) aria2 section as download_dir. Persist the migrated shape once
	// so the stored config no longer carries the dead section.
	if strings.Contains(raw, `"aria2"`) && strings.TrimSpace(cfg.Download.Dir) == "" {
		var legacy struct {
			Aria2 struct {
				DownloadDir string `json:"download_dir"`
			} `json:"aria2"`
		}
		if json.Unmarshal([]byte(raw), &legacy) == nil && legacy.Aria2.DownloadDir != "" {
			cfg.Download.Dir = legacy.Aria2.DownloadDir
		}
		cfg = s.normalizeConfig(cfg)
		if saveErr := s.SaveConfig(cfg); saveErr != nil {
			log.Printf("persist migrated config failed: %v", saveErr)
		}
		return cfg, nil
	}
	return s.normalizeConfig(cfg), nil
}

func (s *FileStore) SaveConfig(cfg model.AppConfig) error {
	cfg = s.normalizeConfig(cfg)
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err = s.db.Exec(`INSERT INTO config(id, data) VALUES (1, ?)
		ON CONFLICT(id) DO UPDATE SET data = excluded.data`, string(raw))
	return err
}

func (s *FileStore) ensureConfig() error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM config WHERE id = 1`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	return s.SaveConfig(s.defaultConfig())
}

func (s *FileStore) defaultConfig() model.AppConfig {
	cfg := model.DefaultAppConfig()
	cfg.Download.Dir = filepath.Join(s.root, "downloads")
	return model.NormalizeAppConfig(cfg)
}

func (s *FileStore) normalizeConfig(cfg model.AppConfig) model.AppConfig {
	cfg = model.NormalizeAppConfig(cfg)
	dir := strings.TrimSpace(cfg.Download.Dir)
	defaultPlaceholder := model.DefaultAppConfig().Download.Dir
	if dir == "" || dir == defaultPlaceholder || dir == "./data/downloads" || dir == "data/downloads" {
		cfg.Download.Dir = filepath.Join(s.root, "downloads")
	} else if !filepath.IsAbs(dir) {
		cfg.Download.Dir = filepath.Join(s.root, dir)
	}
	if abs, err := filepath.Abs(cfg.Download.Dir); err == nil {
		cfg.Download.Dir = abs
	}

	// Local browse root: keep empty (means fall back) or resolve to absolute path.
	localRoot := strings.TrimSpace(cfg.Local.Root)
	if localRoot != "" {
		if !filepath.IsAbs(localRoot) {
			localRoot = filepath.Join(s.root, localRoot)
		}
		if abs, err := filepath.Abs(localRoot); err == nil {
			cfg.Local.Root = abs
		} else {
			cfg.Local.Root = localRoot
		}
	} else {
		cfg.Local.Root = ""
	}
	return cfg
}

// ---------- scans ----------

func (s *FileStore) SaveScan(scan model.ScanSession) error {
	tx, err := s.beginWrite(context.Background())
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(`
INSERT INTO scans(id, source, path, tmdb_id, video_type, status, total_count, matched_count, unmatched_count, created_at, updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET
  source=excluded.source, path=excluded.path, tmdb_id=excluded.tmdb_id, video_type=excluded.video_type,
  status=excluded.status, total_count=excluded.total_count, matched_count=excluded.matched_count,
  unmatched_count=excluded.unmatched_count, updated_at=excluded.updated_at`,
		scan.ID, scan.Source, scan.Path, scan.TMDBID, scan.VideoType, string(scan.Status),
		scan.TotalCount, scan.MatchedCount, scan.UnmatchedCount,
		formatTime(scan.CreatedAt), formatTime(scan.UpdatedAt),
	)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM scan_items WHERE scan_session_id = ?`, scan.ID); err != nil {
		return err
	}
	for _, item := range scan.Items {
		if err := insertScanItem(context.Background(), tx.Tx, item); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func insertScanItem(ctx context.Context, tx *sql.Tx, item model.ScanItem) error {
	parsed, _ := json.Marshal(item.Parsed)
	cands, _ := json.Marshal(item.MatchCandidates)
	if cands == nil {
		cands = []byte("[]")
	}
	var hasMedia any
	if item.HasMedia != nil {
		if *item.HasMedia {
			hasMedia = 1
		} else {
			hasMedia = 0
		}
	}
	_, err := tx.ExecContext(ctx, `
INSERT INTO scan_items(
  id, scan_session_id, openlist_path, file_name, file_size, raw_url, is_video, has_media,
  parsed_json, match_status, match_reason, match_candidates_json,
  selected_item_type, selected_item_id, selected_title, confirmed, created_at, updated_at
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		item.ID, item.ScanSessionID, item.OpenListPath, item.FileName, item.FileSize, item.RawURL,
		boolToInt(item.IsVideo), hasMedia, string(parsed), string(item.MatchStatus), item.MatchReason, string(cands),
		item.SelectedItemType, item.SelectedItemID, item.SelectedTitle, boolToInt(item.Confirmed),
		formatTime(item.CreatedAt), formatTime(item.UpdatedAt),
	)
	return err
}

func (s *FileStore) GetScan(id string) (model.ScanSession, error) {
	scan, err := s.scanMeta(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.ScanSession{}, os.ErrNotExist
		}
		return model.ScanSession{}, err
	}
	items, err := s.listScanItems(id)
	if err != nil {
		return model.ScanSession{}, err
	}
	scan.Items = items
	return scan, nil
}

func (s *FileStore) scanMeta(id string) (model.ScanSession, error) {
	var scan model.ScanSession
	var status, created, updated string
	err := s.db.QueryRow(`
SELECT id, source, path, tmdb_id, video_type, status, total_count, matched_count, unmatched_count, created_at, updated_at
FROM scans WHERE id = ?`, id).Scan(
		&scan.ID, &scan.Source, &scan.Path, &scan.TMDBID, &scan.VideoType, &status,
		&scan.TotalCount, &scan.MatchedCount, &scan.UnmatchedCount, &created, &updated,
	)
	if err != nil {
		return model.ScanSession{}, err
	}
	scan.Status = model.ScanSessionStatus(status)
	scan.CreatedAt = parseTime(created)
	scan.UpdatedAt = parseTime(updated)
	return scan, nil
}

func (s *FileStore) listScanItems(scanID string) ([]model.ScanItem, error) {
	rows, err := s.db.Query(`
SELECT id, scan_session_id, openlist_path, file_name, file_size, raw_url, is_video, has_media,
       parsed_json, match_status, match_reason, match_candidates_json,
       selected_item_type, selected_item_id, selected_title, confirmed, created_at, updated_at
FROM scan_items WHERE scan_session_id = ? ORDER BY file_name ASC`, scanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.ScanItem, 0)
	for rows.Next() {
		item, err := scanItemFromRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type scannable interface {
	Scan(dest ...any) error
}

func scanItemFromRow(row scannable) (model.ScanItem, error) {
	var item model.ScanItem
	var isVideo, confirmed int
	var hasMedia sql.NullInt64
	var parsedJSON, candsJSON, matchStatus, created, updated string
	err := row.Scan(
		&item.ID, &item.ScanSessionID, &item.OpenListPath, &item.FileName, &item.FileSize, &item.RawURL,
		&isVideo, &hasMedia, &parsedJSON, &matchStatus, &item.MatchReason, &candsJSON,
		&item.SelectedItemType, &item.SelectedItemID, &item.SelectedTitle, &confirmed, &created, &updated,
	)
	if err != nil {
		return model.ScanItem{}, err
	}
	item.IsVideo = isVideo != 0
	item.Confirmed = confirmed != 0
	item.MatchStatus = model.MatchStatus(matchStatus)
	item.CreatedAt = parseTime(created)
	item.UpdatedAt = parseTime(updated)
	if hasMedia.Valid {
		v := hasMedia.Int64 != 0
		item.HasMedia = &v
	}
	_ = json.Unmarshal([]byte(parsedJSON), &item.Parsed)
	if candsJSON != "" {
		_ = json.Unmarshal([]byte(candsJSON), &item.MatchCandidates)
	}
	if item.MatchCandidates == nil {
		item.MatchCandidates = []model.MatchCandidate{}
	}
	return item, nil
}

func (s *FileStore) UpdateScanItem(scanID, itemID string, updater func(*model.ScanItem) error) (model.ScanSession, error) {
	tx, err := s.beginWrite(context.Background())
	if err != nil {
		return model.ScanSession{}, err
	}
	defer func() { _ = tx.Rollback() }()

	item, err := getScanItemTx(context.Background(), tx.Tx, scanID, itemID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.ScanSession{}, os.ErrNotExist
		}
		return model.ScanSession{}, err
	}
	if err := updater(&item); err != nil {
		return model.ScanSession{}, err
	}
	item.UpdatedAt = time.Now()
	if err := replaceScanItemTx(context.Background(), tx.Tx, item); err != nil {
		return model.ScanSession{}, err
	}
	if err := refreshScanCountsTx(context.Background(), tx.Tx, scanID); err != nil {
		return model.ScanSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.ScanSession{}, err
	}
	return s.GetScan(scanID)
}

func getScanItemTx(ctx context.Context, tx *sql.Tx, scanID, itemID string) (model.ScanItem, error) {
	row := tx.QueryRowContext(ctx, `
SELECT id, scan_session_id, openlist_path, file_name, file_size, raw_url, is_video, has_media,
       parsed_json, match_status, match_reason, match_candidates_json,
       selected_item_type, selected_item_id, selected_title, confirmed, created_at, updated_at
FROM scan_items WHERE scan_session_id = ? AND id = ?`, scanID, itemID)
	return scanItemFromRow(row)
}

func replaceScanItemTx(ctx context.Context, tx *sql.Tx, item model.ScanItem) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM scan_items WHERE id = ?`, item.ID); err != nil {
		return err
	}
	return insertScanItem(ctx, tx, item)
}

func refreshScanCountsTx(ctx context.Context, tx *sql.Tx, scanID string) error {
	var total, matched int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM scan_items WHERE scan_session_id = ?`, scanID).Scan(&total); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM scan_items WHERE scan_session_id = ? AND match_status = ?`,
		scanID, string(model.MatchStatusMatched)).Scan(&matched); err != nil {
		return err
	}
	unmatched := total - matched
	_, err := tx.ExecContext(ctx, `UPDATE scans SET total_count=?, matched_count=?, unmatched_count=?, updated_at=? WHERE id=?`,
		total, matched, unmatched, formatTime(time.Now()), scanID)
	return err
}

func (s *FileStore) DeleteScan(id string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.Exec(`DELETE FROM scans WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return os.ErrNotExist
	}
	return nil
}

func (s *FileStore) DeleteScanItem(scanID, itemID string) (model.ScanSession, error) {
	return s.DeleteScanItems(scanID, []string{itemID})
}

// DeleteScanItems removes the given scan items in a single transaction and
// returns the refreshed scan session.
func (s *FileStore) DeleteScanItems(scanID string, itemIDs []string) (model.ScanSession, error) {
	if len(itemIDs) == 0 {
		return model.ScanSession{}, errors.New("no scan items to delete")
	}
	tx, err := s.beginWrite(context.Background())
	if err != nil {
		return model.ScanSession{}, err
	}
	defer func() { _ = tx.Rollback() }()

	placeholders := make([]string, len(itemIDs))
	args := make([]any, 0, len(itemIDs)+1)
	args = append(args, scanID)
	for i, id := range itemIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	res, err := tx.Exec(`DELETE FROM scan_items WHERE scan_session_id = ? AND id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return model.ScanSession{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.ScanSession{}, os.ErrNotExist
	}
	if err := refreshScanCountsTx(context.Background(), tx.Tx, scanID); err != nil {
		return model.ScanSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.ScanSession{}, err
	}
	return s.GetScan(scanID)
}

func (s *FileStore) ListScans() ([]model.ScanSession, error) {
	rows, err := s.db.Query(`
SELECT id, source, path, tmdb_id, video_type, status, total_count, matched_count, unmatched_count, created_at, updated_at
FROM scans WHERE status = 'completed' AND total_count > 0 ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scans := make([]model.ScanSession, 0)
	for rows.Next() {
		var scan model.ScanSession
		var status, created, updated string
		if err := rows.Scan(
			&scan.ID, &scan.Source, &scan.Path, &scan.TMDBID, &scan.VideoType, &status,
			&scan.TotalCount, &scan.MatchedCount, &scan.UnmatchedCount, &created, &updated,
		); err != nil {
			return nil, err
		}
		scan.Status = model.ScanSessionStatus(status)
		scan.CreatedAt = parseTime(created)
		scan.UpdatedAt = parseTime(updated)
		items, err := s.listScanItems(scan.ID)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			continue
		}
		scan.Items = items
		scans = append(scans, scan)
	}
	return scans, rows.Err()
}

// ---------- tasks ----------

func (s *FileStore) SaveTask(task model.Task) error {
	return s.upsertTask(task)
}

func (s *FileStore) upsertTask(task model.Task) error {
	var finished any
	if task.FinishedAt != nil {
		finished = formatTime(*task.FinishedAt)
	}
	var lastErr any
	if task.Result.LastErrorAt != nil {
		lastErr = formatTime(*task.Result.LastErrorAt)
	}
	var season, episode any
	if task.Parsed.Season != nil {
		season = *task.Parsed.Season
	}
	if task.Parsed.Episode != nil {
		episode = *task.Parsed.Episode
	}
	presignsJSON, _ := json.Marshal(task.Upload.MultipartPresigns)
	partsJSON, _ := json.Marshal(task.Upload.MultipartParts)
	if len(presignsJSON) == 0 {
		presignsJSON = []byte("[]")
	}
	if len(partsJSON) == 0 {
		partsJSON = []byte("[]")
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`
INSERT INTO tasks(
  id, scan_session_id, scan_item_id, status, paused, retry_count, keep_local_file,
  source_type, source_path, source_raw_url, source_file_name, source_file_size,
  parsed_season, parsed_episode, parsed_is_special,
  target_tmdb_id, target_item_type, target_item_id, target_title,
  dl_save_dir, dl_local_path, dl_status, dl_total_bytes, dl_completed_bytes, dl_progress, dl_speed,
  ul_storage, ul_file_id, ul_upload_url, ul_upload_type, ul_multipart_size_min, ul_multipart_size_max,
  ul_multipart_presigns, ul_multipart_parts, ul_media_id, ul_total_bytes, ul_uploaded_bytes, ul_progress, ul_speed,
  ul_status, ul_save_retry_count, ul_last_save_error,
  result_error_message, result_error_stage, result_error_code, result_last_error_at,
  created_at, updated_at, finished_at
) VALUES (
  ?,?,?,?,?,?,?,
  ?,?,?,?,?,
  ?,?,?,
  ?,?,?,?,
  ?,?,?,?,?,?,?,
  ?,?,?,?,?,?,
  ?,?,?,?,?,?,?,
  ?,?,?,
  ?,?,?,?,
  ?,?,?
)
ON CONFLICT(id) DO UPDATE SET
  scan_session_id=excluded.scan_session_id, scan_item_id=excluded.scan_item_id, status=excluded.status,
  paused=excluded.paused, retry_count=excluded.retry_count, keep_local_file=excluded.keep_local_file,
  source_type=excluded.source_type, source_path=excluded.source_path, source_raw_url=excluded.source_raw_url,
  source_file_name=excluded.source_file_name, source_file_size=excluded.source_file_size,
  parsed_season=excluded.parsed_season, parsed_episode=excluded.parsed_episode, parsed_is_special=excluded.parsed_is_special,
  target_tmdb_id=excluded.target_tmdb_id, target_item_type=excluded.target_item_type,
  target_item_id=excluded.target_item_id, target_title=excluded.target_title,
  dl_save_dir=excluded.dl_save_dir, dl_local_path=excluded.dl_local_path,
  dl_status=excluded.dl_status, dl_total_bytes=excluded.dl_total_bytes, dl_completed_bytes=excluded.dl_completed_bytes,
  dl_progress=excluded.dl_progress, dl_speed=excluded.dl_speed,
  ul_storage=excluded.ul_storage, ul_file_id=excluded.ul_file_id, ul_upload_url=excluded.ul_upload_url,
  ul_upload_type=excluded.ul_upload_type, ul_multipart_size_min=excluded.ul_multipart_size_min,
  ul_multipart_size_max=excluded.ul_multipart_size_max, ul_multipart_presigns=excluded.ul_multipart_presigns,
  ul_multipart_parts=excluded.ul_multipart_parts,
  ul_media_id=excluded.ul_media_id, ul_total_bytes=excluded.ul_total_bytes, ul_uploaded_bytes=excluded.ul_uploaded_bytes,
  ul_progress=excluded.ul_progress, ul_speed=excluded.ul_speed, ul_status=excluded.ul_status,
  ul_save_retry_count=excluded.ul_save_retry_count, ul_last_save_error=excluded.ul_last_save_error,
  result_error_message=excluded.result_error_message, result_error_stage=excluded.result_error_stage,
  result_error_code=excluded.result_error_code, result_last_error_at=excluded.result_last_error_at,
  updated_at=excluded.updated_at, finished_at=excluded.finished_at`,
		task.ID, task.ScanSessionID, task.ScanItemID, string(task.Status), boolToInt(task.Paused), task.RetryCount, boolToInt(task.KeepLocalFile),
		task.Source.Type, task.Source.Path, task.Source.RawURL, task.Source.FileName, task.Source.FileSize,
		season, episode, boolToInt(task.Parsed.IsSpecial),
		task.Target.TMDBID, task.Target.ItemType, task.Target.ItemID, task.Target.Title,
		task.Download.SaveDir, task.Download.LocalPath, task.Download.Status,
		task.Download.TotalBytes, task.Download.CompletedBytes, task.Download.Progress, task.Download.Speed,
		task.Upload.Storage, task.Upload.FileID, task.Upload.UploadURL, task.Upload.UploadType,
		task.Upload.MultipartSizeMin, task.Upload.MultipartSizeMax, string(presignsJSON), string(partsJSON),
		task.Upload.MediaID,
		task.Upload.TotalBytes, task.Upload.UploadedBytes, task.Upload.Progress, task.Upload.Speed,
		task.Upload.Status, task.Upload.SaveRetryCount, task.Upload.LastSaveError,
		task.Result.ErrorMessage, task.Result.ErrorStage, task.Result.ErrorCode, lastErr,
		formatTime(task.CreatedAt), formatTime(task.UpdatedAt), finished,
	)
	return err
}

func (s *FileStore) GetTask(id string) (model.Task, error) {
	task, err := s.taskFromQuery(`SELECT `+taskColumns+` FROM tasks WHERE id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Task{}, os.ErrNotExist
	}
	return task, err
}

const taskColumns = `
  id, scan_session_id, scan_item_id, status, paused, retry_count, keep_local_file,
  source_type, source_path, source_raw_url, source_file_name, source_file_size,
  parsed_season, parsed_episode, parsed_is_special,
  target_tmdb_id, target_item_type, target_item_id, target_title,
  dl_save_dir, dl_local_path, dl_status, dl_total_bytes, dl_completed_bytes, dl_progress, dl_speed,
  ul_storage, ul_file_id, ul_upload_url, ul_upload_type, ul_multipart_size_min, ul_multipart_size_max,
  ul_multipart_presigns, ul_multipart_parts, ul_media_id, ul_total_bytes, ul_uploaded_bytes, ul_progress, ul_speed,
  ul_status, ul_save_retry_count, ul_last_save_error,
  result_error_message, result_error_stage, result_error_code, result_last_error_at,
  created_at, updated_at, finished_at`

func (s *FileStore) taskFromQuery(query string, args ...any) (model.Task, error) {
	row := s.db.QueryRow(query, args...)
	return scanTask(row)
}

func scanTask(row scannable) (model.Task, error) {
	var t model.Task
	var status, created, updated string
	var paused, special int
	var keepLocal int
	var season, episode sql.NullInt64
	var lastErr, finished sql.NullString
	var presignsJSON, partsJSON string

	err := row.Scan(
		&t.ID, &t.ScanSessionID, &t.ScanItemID, &status, &paused, &t.RetryCount, &keepLocal,
		&t.Source.Type, &t.Source.Path, &t.Source.RawURL, &t.Source.FileName, &t.Source.FileSize,
		&season, &episode, &special,
		&t.Target.TMDBID, &t.Target.ItemType, &t.Target.ItemID, &t.Target.Title,
		&t.Download.SaveDir, &t.Download.LocalPath, &t.Download.Status,
		&t.Download.TotalBytes, &t.Download.CompletedBytes, &t.Download.Progress, &t.Download.Speed,
		&t.Upload.Storage, &t.Upload.FileID, &t.Upload.UploadURL, &t.Upload.UploadType,
		&t.Upload.MultipartSizeMin, &t.Upload.MultipartSizeMax, &presignsJSON, &partsJSON,
		&t.Upload.MediaID,
		&t.Upload.TotalBytes, &t.Upload.UploadedBytes, &t.Upload.Progress, &t.Upload.Speed,
		&t.Upload.Status, &t.Upload.SaveRetryCount, &t.Upload.LastSaveError,
		&t.Result.ErrorMessage, &t.Result.ErrorStage, &t.Result.ErrorCode, &lastErr,
		&created, &updated, &finished,
	)
	if err != nil {
		return model.Task{}, err
	}
	t.Status = model.TaskStatus(status)
	t.Paused = paused != 0
	t.KeepLocalFile = keepLocal != 0
	t.Parsed.IsSpecial = special != 0
	if season.Valid {
		v := int(season.Int64)
		t.Parsed.Season = &v
	}
	if episode.Valid {
		v := int(episode.Int64)
		t.Parsed.Episode = &v
	}
	if err := json.Unmarshal([]byte(presignsJSON), &t.Upload.MultipartPresigns); err != nil {
		return model.Task{}, fmt.Errorf("scan multipart presigns: %w", err)
	}
	if err := json.Unmarshal([]byte(partsJSON), &t.Upload.MultipartParts); err != nil {
		return model.Task{}, fmt.Errorf("scan multipart parts: %w", err)
	}
	t.CreatedAt = parseTime(created)
	t.UpdatedAt = parseTime(updated)
	if lastErr.Valid && lastErr.String != "" {
		tm := parseTime(lastErr.String)
		t.Result.LastErrorAt = &tm
	}
	if finished.Valid && finished.String != "" {
		tm := parseTime(finished.String)
		t.FinishedAt = &tm
	}
	return t, nil
}

func (s *FileStore) UpdateTask(id string, updater func(*model.Task) error) (model.Task, error) {
	// Transactional read-modify-write: much cheaper than rewriting whole JSON files.
	tx, err := s.beginWrite(context.Background())
	if err != nil {
		return model.Task{}, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRow(`SELECT `+taskColumns+` FROM tasks WHERE id = ?`, id)
	task, err := scanTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Task{}, os.ErrNotExist
		}
		return model.Task{}, err
	}
	if err := updater(&task); err != nil {
		return model.Task{}, err
	}
	if err := upsertTaskTx(context.Background(), tx.Tx, task); err != nil {
		return model.Task{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func upsertTaskTx(ctx context.Context, tx *sql.Tx, task model.Task) error {
	var finished any
	if task.FinishedAt != nil {
		finished = formatTime(*task.FinishedAt)
	}
	var lastErr any
	if task.Result.LastErrorAt != nil {
		lastErr = formatTime(*task.Result.LastErrorAt)
	}
	var season, episode any
	if task.Parsed.Season != nil {
		season = *task.Parsed.Season
	}
	if task.Parsed.Episode != nil {
		episode = *task.Parsed.Episode
	}
	presignsJSON, _ := json.Marshal(task.Upload.MultipartPresigns)
	partsJSON, _ := json.Marshal(task.Upload.MultipartParts)
	if len(presignsJSON) == 0 {
		presignsJSON = []byte("[]")
	}
	if len(partsJSON) == 0 {
		partsJSON = []byte("[]")
	}

	_, err := tx.ExecContext(ctx, `
INSERT INTO tasks(
  id, scan_session_id, scan_item_id, status, paused, retry_count, keep_local_file,
  source_type, source_path, source_raw_url, source_file_name, source_file_size,
  parsed_season, parsed_episode, parsed_is_special,
  target_tmdb_id, target_item_type, target_item_id, target_title,
  dl_save_dir, dl_local_path, dl_status, dl_total_bytes, dl_completed_bytes, dl_progress, dl_speed,
  ul_storage, ul_file_id, ul_upload_url, ul_upload_type, ul_multipart_size_min, ul_multipart_size_max,
  ul_multipart_presigns, ul_multipart_parts, ul_media_id, ul_total_bytes, ul_uploaded_bytes, ul_progress, ul_speed,
  ul_status, ul_save_retry_count, ul_last_save_error,
  result_error_message, result_error_stage, result_error_code, result_last_error_at,
  created_at, updated_at, finished_at
) VALUES (
  ?,?,?,?,?,?,?,
  ?,?,?,?,?,
  ?,?,?,
  ?,?,?,?,
  ?,?,?,?,?,?,?,
  ?,?,?,?,?,?,
  ?,?,?,?,?,?,?,
  ?,?,?,
  ?,?,?,?,
  ?,?,?
)
ON CONFLICT(id) DO UPDATE SET
  scan_session_id=excluded.scan_session_id, scan_item_id=excluded.scan_item_id, status=excluded.status,
  paused=excluded.paused, retry_count=excluded.retry_count, keep_local_file=excluded.keep_local_file,
  source_type=excluded.source_type, source_path=excluded.source_path, source_raw_url=excluded.source_raw_url,
  source_file_name=excluded.source_file_name, source_file_size=excluded.source_file_size,
  parsed_season=excluded.parsed_season, parsed_episode=excluded.parsed_episode, parsed_is_special=excluded.parsed_is_special,
  target_tmdb_id=excluded.target_tmdb_id, target_item_type=excluded.target_item_type,
  target_item_id=excluded.target_item_id, target_title=excluded.target_title,
  dl_save_dir=excluded.dl_save_dir, dl_local_path=excluded.dl_local_path,
  dl_status=excluded.dl_status, dl_total_bytes=excluded.dl_total_bytes, dl_completed_bytes=excluded.dl_completed_bytes,
  dl_progress=excluded.dl_progress, dl_speed=excluded.dl_speed,
  ul_storage=excluded.ul_storage, ul_file_id=excluded.ul_file_id, ul_upload_url=excluded.ul_upload_url,
  ul_upload_type=excluded.ul_upload_type, ul_multipart_size_min=excluded.ul_multipart_size_min,
  ul_multipart_size_max=excluded.ul_multipart_size_max, ul_multipart_presigns=excluded.ul_multipart_presigns,
  ul_multipart_parts=excluded.ul_multipart_parts,
  ul_media_id=excluded.ul_media_id, ul_total_bytes=excluded.ul_total_bytes, ul_uploaded_bytes=excluded.ul_uploaded_bytes,
  ul_progress=excluded.ul_progress, ul_speed=excluded.ul_speed, ul_status=excluded.ul_status,
  ul_save_retry_count=excluded.ul_save_retry_count, ul_last_save_error=excluded.ul_last_save_error,
  result_error_message=excluded.result_error_message, result_error_stage=excluded.result_error_stage,
  result_error_code=excluded.result_error_code, result_last_error_at=excluded.result_last_error_at,
  updated_at=excluded.updated_at, finished_at=excluded.finished_at`,
		task.ID, task.ScanSessionID, task.ScanItemID, string(task.Status), boolToInt(task.Paused), task.RetryCount, boolToInt(task.KeepLocalFile),
		task.Source.Type, task.Source.Path, task.Source.RawURL, task.Source.FileName, task.Source.FileSize,
		season, episode, boolToInt(task.Parsed.IsSpecial),
		task.Target.TMDBID, task.Target.ItemType, task.Target.ItemID, task.Target.Title,
		task.Download.SaveDir, task.Download.LocalPath, task.Download.Status,
		task.Download.TotalBytes, task.Download.CompletedBytes, task.Download.Progress, task.Download.Speed,
		task.Upload.Storage, task.Upload.FileID, task.Upload.UploadURL, task.Upload.UploadType,
		task.Upload.MultipartSizeMin, task.Upload.MultipartSizeMax, string(presignsJSON), string(partsJSON),
		task.Upload.MediaID,
		task.Upload.TotalBytes, task.Upload.UploadedBytes, task.Upload.Progress, task.Upload.Speed,
		task.Upload.Status, task.Upload.SaveRetryCount, task.Upload.LastSaveError,
		task.Result.ErrorMessage, task.Result.ErrorStage, task.Result.ErrorCode, lastErr,
		formatTime(task.CreatedAt), formatTime(task.UpdatedAt), finished,
	)
	return err
}

func (s *FileStore) ListTasks() ([]model.Task, error) {
	rows, err := s.db.Query(`SELECT ` + taskColumns + ` FROM tasks ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]model.Task, 0)
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *FileStore) DeleteTask(id string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return os.ErrNotExist
	}
	// logs cascade via FK
	return nil
}

// ---------- task logs ----------

func (s *FileStore) SaveTaskLog(log model.TaskLog) error {
	tx, err := s.beginWrite(context.Background())
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM task_logs WHERE task_id = ?`, log.TaskID); err != nil {
		return err
	}
	for _, item := range log.Items {
		if err := insertLogTx(context.Background(), tx.Tx, log.TaskID, item); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func insertLogTx(ctx context.Context, tx *sql.Tx, taskID string, item model.TaskLogItem) error {
	id := item.ID
	if id == "" {
		id = utils.NewID("log")
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO task_logs(id, task_id, level, message, time) VALUES (?,?,?,?,?)`,
		id, taskID, item.Level, item.Message, formatTime(item.Time))
	return err
}

func (s *FileStore) GetTaskLog(taskID string) (model.TaskLog, error) {
	rows, err := s.db.Query(`
SELECT id, level, message, time FROM task_logs WHERE task_id = ? ORDER BY time ASC, id ASC`, taskID)
	if err != nil {
		return model.TaskLog{}, err
	}
	defer rows.Close()

	items := make([]model.TaskLogItem, 0)
	for rows.Next() {
		var item model.TaskLogItem
		var ts string
		if err := rows.Scan(&item.ID, &item.Level, &item.Message, &ts); err != nil {
			return model.TaskLog{}, err
		}
		item.Time = parseTime(ts)
		items = append(items, item)
	}
	return model.TaskLog{TaskID: taskID, Items: items}, rows.Err()
}

func (s *FileStore) AppendTaskLog(taskID string, item model.TaskLogItem) error {
	if item.ID == "" {
		item.ID = utils.NewID("log")
	}
	if item.Time.IsZero() {
		item.Time = time.Now()
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT INTO task_logs(id, task_id, level, message, time) VALUES (?,?,?,?,?)`,
		item.ID, taskID, item.Level, item.Message, formatTime(item.Time))
	return err
}

// ---------- helpers ----------

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}
