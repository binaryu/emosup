package store

import (
	"testing"

	"emosup/backend/internal/model"
)

// TestLegacyAria2ConfigMigratesToDownloadDir verifies old configs that stored
// the cache dir under the removed aria2 section are migrated on load.
func TestLegacyAria2ConfigMigratesToDownloadDir(t *testing.T) {
	root := t.TempDir()
	s := NewFileStore(root)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Simulate a pre-v1.2.3 config row with the aria2 section.
	raw := `{"server":{"host":"0.0.0.0","port":8080},"aria2":{"rpc_url":"http://127.0.0.1:6800/jsonrpc","download_dir":"/custom/cache","poll_interval_seconds":3,"connect_timeout_seconds":10}}`
	if _, err := s.db.Exec(`UPDATE config SET data = ? WHERE id = 1`, raw); err != nil {
		t.Fatal(err)
	}

	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Download.Dir == "" || cfg.Download.Dir == model.DefaultAppConfig().Download.Dir {
		t.Fatalf("download dir not migrated: %q", cfg.Download.Dir)
	}
}

// TestLegacyAria2GIDColumnDropped verifies the dl_aria2_gid column is removed
// from pre-existing databases.
func TestLegacyAria2GIDColumnDropped(t *testing.T) {
	root := t.TempDir()
	store := NewFileStore(root)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	// Re-add the legacy column as an old DB would have it, then re-init.
	if _, err := store.db.Exec(`ALTER TABLE tasks ADD COLUMN dl_aria2_gid TEXT NOT NULL DEFAULT ''`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store = NewFileStore(root)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	exists, err := store.columnExists("tasks", "dl_aria2_gid")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("dl_aria2_gid column should have been dropped")
	}
}
