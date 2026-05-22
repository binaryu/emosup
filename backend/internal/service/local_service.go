package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"emosup/backend/internal/store"
)

type LocalEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

type LocalService struct {
	store *store.FileStore
}

func NewLocalService(store *store.FileStore) *LocalService {
	return &LocalService{store: store}
}

func (s *LocalService) GetFileInfo(_ context.Context, relPath string) (LocalEntry, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return LocalEntry{}, err
	}

	downloadDir := strings.TrimSpace(cfg.Aria2.DownloadDir)
	if downloadDir == "" {
		downloadDir = "."
	}

	absBase, err := filepath.Abs(downloadDir)
	if err != nil {
		return LocalEntry{}, err
	}

	cleanRel := filepath.Clean(strings.TrimPrefix(filepath.Clean(relPath), "/"))
	fullPath := filepath.Join(absBase, cleanRel)

	// Prevent directory traversal
	if !strings.HasPrefix(filepath.Clean(fullPath)+string(filepath.Separator), filepath.Clean(absBase)+string(filepath.Separator)) && filepath.Clean(fullPath) != filepath.Clean(absBase) {
		return LocalEntry{}, errors.New("access denied")
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return LocalEntry{}, err
	}

	if info.IsDir() {
		return LocalEntry{}, errors.New("path is a directory, not a file")
	}

	return LocalEntry{
		Name:  info.Name(),
		Path:  "/" + cleanRel,
		IsDir: false,
		Size:  info.Size(),
	}, nil
}

func (s *LocalService) Browse(_ context.Context, relPath string) (string, []LocalEntry, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return "", nil, err
	}

	downloadDir := strings.TrimSpace(cfg.Aria2.DownloadDir)
	if downloadDir == "" {
		downloadDir = "."
	}

	absBase, err := filepath.Abs(downloadDir)
	if err != nil {
		return "", nil, err
	}

	cleanRel := filepath.Clean(strings.TrimPrefix(filepath.Clean(relPath), "/"))
	if cleanRel == "." {
		cleanRel = ""
	}
	fullPath := filepath.Join(absBase, cleanRel)

	// Prevent directory traversal
	if !strings.HasPrefix(filepath.Clean(fullPath)+string(filepath.Separator), filepath.Clean(absBase)+string(filepath.Separator)) && filepath.Clean(fullPath) != filepath.Clean(absBase) {
		return "", nil, errors.New("access denied")
	}

	dirEntries, err := os.ReadDir(fullPath)
	if err != nil {
		return "", nil, err
	}

	result := make([]LocalEntry, 0, len(dirEntries))
	for _, entry := range dirEntries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		entryPath := filepath.Join(cleanRel, entry.Name())
		if cleanRel == "" {
			entryPath = entry.Name()
		}

		result = append(result, LocalEntry{
			Name:  entry.Name(),
			Path:  "/" + filepath.ToSlash(entryPath),
			IsDir: entry.IsDir(),
			Size:  info.Size(),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	displayPath := "/" + cleanRel
	if cleanRel == "" {
		displayPath = "/"
	}

	return displayPath, result, nil
}
