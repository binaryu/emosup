package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"emosup/backend/internal/client"
	"emosup/backend/internal/model"
	"emosup/backend/internal/store"
)

func TestCreateScan(t *testing.T) {
	t.Parallel()

	openListServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/fs/list":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 200,
				"data": map[string]any{
					"content": []map[string]any{
						{"name": "Demo.S01E01.mkv", "is_dir": false, "size": 100},
						{"name": "Demo.EP03.mkv", "is_dir": false, "size": 120},
						{"name": "notes.txt", "is_dir": false, "size": 20},
					},
				},
			})
		case "/api/fs/get":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 200,
				"data": map[string]any{
					"raw_url": "https://openlist.example/raw/demo",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer openListServer.Close()

	emosServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/video/tree" {
			http.NotFound(w, r)
			return
		}

		_ = json.NewEncoder(w).Encode(client.EmosVideoTree{
			VideoType: "tv",
			ItemType:  "vl",
			ItemID:    10,
			TMDBID:    12345,
			Title:     "Demo",
			Seasons: []client.EmosSeason{
				{
					SeasonNumber: 1,
					Episodes: []client.EmosEpisode{
						{ItemType: "ve", ItemID: 101, EpisodeNumber: 1, EpisodeTitle: "Pilot"},
						{ItemType: "ve", ItemID: 102, EpisodeNumber: 2, EpisodeTitle: "Second"},
					},
				},
			},
		})
	}))
	defer emosServer.Close()

	dataDir := t.TempDir()
	fileStore := store.NewFileStore(dataDir)
	if err := fileStore.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}

	cfg := model.DefaultAppConfig()
	cfg.OpenList.BaseURL = openListServer.URL
	cfg.Emos.BaseURL = emosServer.URL
	cfg.Emos.Token = "demo-token"
	if err := fileStore.SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	openListService := NewOpenListService(fileStore, client.NewHTTPOpenListClient())
	emosService := NewEmosService(fileStore, client.NewHTTPEmosClient())
	matchService := NewMatchService()
	scanService := NewScanService(fileStore, openListService, nil, emosService, matchService)

	scan, err := scanService.CreateScan(context.Background(), CreateScanRequest{
		Path:      "/TV/Demo/Season 1",
		TMDBID:    12345,
		VideoType: "tv",
	})
	if err != nil {
		t.Fatalf("create scan: %v", err)
	}

	if scan.TotalCount != 2 {
		t.Fatalf("expected 2 video items, got %d", scan.TotalCount)
	}
	if scan.MatchedCount != 1 {
		t.Fatalf("expected 1 matched item, got %d", scan.MatchedCount)
	}
	if scan.UnmatchedCount != 1 {
		t.Fatalf("expected 1 unmatched item, got %d", scan.UnmatchedCount)
	}
	if scan.Items[0].RawURL == "" {
		t.Fatalf("expected raw url to be filled")
	}

	// Persisted in SQLite (not JSON files).
	reloaded, err := fileStore.GetScan(scan.ID)
	if err != nil {
		t.Fatalf("expected scan to persist in sqlite: %v", err)
	}
	if reloaded.TotalCount != scan.TotalCount {
		t.Fatalf("reloaded total=%d want %d", reloaded.TotalCount, scan.TotalCount)
	}
}

func TestCreateScanFailedEmosTreeDoesNotLeaveEmptyScan(t *testing.T) {
	t.Parallel()

	openListServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/fs/list" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 200,
				"data": map[string]any{
					"content": []map[string]any{
						{"name": "Demo.S01E01.mkv", "is_dir": false, "size": 100},
					},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer openListServer.Close()

	// EMOS server returns empty array [] for unknown TMDB ID
	emosServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/video/tree" {
			_ = json.NewEncoder(w).Encode([]client.EmosVideoTree{})
			return
		}
		http.NotFound(w, r)
	}))
	defer emosServer.Close()

	dataDir := t.TempDir()
	fileStore := store.NewFileStore(dataDir)
	if err := fileStore.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}

	cfg := model.DefaultAppConfig()
	cfg.OpenList.BaseURL = openListServer.URL
	cfg.Emos.BaseURL = emosServer.URL
	cfg.Emos.Token = "demo-token"
	if err := fileStore.SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	openListService := NewOpenListService(fileStore, client.NewHTTPOpenListClient())
	emosService := NewEmosService(fileStore, client.NewHTTPEmosClient())
	matchService := NewMatchService()
	scanService := NewScanService(fileStore, openListService, nil, emosService, matchService)

	_, err := scanService.CreateScan(context.Background(), CreateScanRequest{
		Path:      "/TV/Demo/Season 1",
		TMDBID:    99999,
		VideoType: "tv",
	})
	if err == nil {
		t.Fatal("expected error when emos tree is empty, got nil")
	}

	// Verify no empty scan session was left in store
	scans, err := fileStore.ListScans()
	if err != nil {
		t.Fatalf("list scans: %v", err)
	}
	if len(scans) != 0 {
		t.Fatalf("expected 0 scans in store after failure, got %d", len(scans))
	}
}
