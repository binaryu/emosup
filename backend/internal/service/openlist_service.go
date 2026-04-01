package service

import (
	"context"
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

func (s *OpenListService) Browse(ctx context.Context, path string) ([]client.OpenListEntry, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return nil, err
	}

	entries, err := s.client.List(ctx, client.OpenListAccess{
		BaseURL: cfg.OpenList.BaseURL,
		Token:   cfg.OpenList.Token,
	}, path)
	if err != nil {
		return nil, err
	}

	return entries, nil
}

func (s *OpenListService) ListVideoFiles(ctx context.Context, path string) ([]client.OpenListEntry, error) {
	entries, err := s.Browse(ctx, path)
	if err != nil {
		return nil, err
	}

	videos := make([]client.OpenListEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir || !IsVideoFile(entry.Name) {
			continue
		}
		videos = append(videos, entry)
	}

	return videos, nil
}

func (s *OpenListService) GetRawLink(ctx context.Context, path string) (string, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return "", err
	}

	rawURL, err := s.client.GetRawLink(ctx, client.OpenListAccess{
		BaseURL: cfg.OpenList.BaseURL,
		Token:   cfg.OpenList.Token,
	}, path)
	if err != nil {
		return "", err
	}

	return client.ResolveMaybeRelativeURL(cfg.OpenList.BaseURL, rawURL), nil
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
