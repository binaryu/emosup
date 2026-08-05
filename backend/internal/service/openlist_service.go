package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"emosup/backend/internal/client"
	"emosup/backend/internal/model"
	"emosup/backend/internal/store"
)

type OpenListService struct {
	store  *store.FileStore
	client client.OpenListClient
}

func NewOpenListService(store *store.FileStore, openListClient client.OpenListClient) *OpenListService {
	return &OpenListService{
		store:  store,
		client: openListClient,
	}
}

func (s *OpenListService) buildAccess(cfg model.AppConfig) client.OpenListAccess {
	return client.OpenListAccess{
		BaseURL:  cfg.OpenList.BaseURL,
		Username: cfg.OpenList.Username,
		Password: cfg.OpenList.Password,
		Token:    cfg.OpenList.Token,
	}
}

func (s *OpenListService) ensureToken(ctx context.Context, access *client.OpenListAccess) error {
	if access.Token != "" {
		return nil
	}
	if access.Username == "" || access.Password == "" {
		return nil // No credentials to try
	}

	token, err := s.client.Login(ctx, *access)
	if err != nil {
		return err
	}
	access.Token = token
	return nil
}

func (s *OpenListService) Browse(ctx context.Context, path string) ([]client.OpenListEntry, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return nil, err
	}

	access := s.buildAccess(cfg)
	// Auto-login if username/password provided but no token
	if err := s.ensureToken(ctx, &access); err != nil {
		return nil, err
	}

	entries, err := s.client.List(ctx, access, path)
	if err != nil {
		return nil, err
	}

	return entries, nil
}

func (s *OpenListService) GetFileInfo(ctx context.Context, path string) ([]client.OpenListEntry, error) {
	// List parent directory and find the matching file by name
	parentDir := filepath.Dir(path)
	if parentDir == "" || parentDir == "." {
		parentDir = "/"
	}

	entries, err := s.Browse(ctx, parentDir)
	if err != nil {
		return nil, err
	}

	baseName := filepath.Base(path)
	for _, entry := range entries {
		if !entry.IsDir && (entry.Name == baseName || strings.EqualFold(entry.Path, path)) {
			return []client.OpenListEntry{entry}, nil
		}
	}

	return nil, fmt.Errorf("file not found: %s", path)
}

func (s *OpenListService) ListVideoFiles(ctx context.Context, path string) ([]client.OpenListEntry, error) {
	return s.listVideoFilesRecursive(ctx, path)
}

func (s *OpenListService) listVideoFilesRecursive(ctx context.Context, path string) ([]client.OpenListEntry, error) {
	entries, err := s.Browse(ctx, path)
	if err != nil {
		return nil, err
	}

	videos := make([]client.OpenListEntry, 0)
	for _, entry := range entries {
		if entry.IsDir {
			subVideos, subErr := s.listVideoFilesRecursive(ctx, entry.Path)
			if subErr != nil {
				continue // skip inaccessible subdirs
			}
			videos = append(videos, subVideos...)
			continue
		}
		if IsVideoFile(entry.Name) {
			videos = append(videos, entry)
		}
	}

	return videos, nil
}

func (s *OpenListService) GetRawLink(ctx context.Context, path string) (string, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return "", err
	}

	access := s.buildAccess(cfg)
	if err := s.ensureToken(ctx, &access); err != nil {
		return "", err
	}

	return s.GetRawLinkWithAccess(ctx, access, path)
}

// GetRawLinkWithAccess fetches a raw link with a pre-built access so callers
// that process many files (e.g. scans) avoid reloading config per file.
func (s *OpenListService) GetRawLinkWithAccess(ctx context.Context, access client.OpenListAccess, path string) (string, error) {
	rawURL, err := s.client.GetRawLink(ctx, access, path)
	if err != nil {
		return "", err
	}

	return client.ResolveMaybeRelativeURL(access.BaseURL, rawURL), nil
}

func IsVideoFile(name string) bool {
	extension := strings.ToLower(filepath.Ext(name))
	switch extension {
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".m4v", ".ts", ".mpg", ".mpeg", ".webm":
		return true
	default:
		return false
	}
}

func BuildInvalidScanItem(scanID string, entry client.OpenListEntry, reason string) model.ScanItem {
	return model.ScanItem{
		ScanSessionID: scanID,
		OpenListPath:  entry.Path,
		FileName:      entry.Name,
		FileSize:      entry.Size,
		IsVideo:       true,
		MatchStatus:   model.MatchStatusInvalid,
		MatchReason:   reason,
	}
}
