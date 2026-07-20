package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"emosup/backend/internal/store"
	"emosup/backend/internal/utils"
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

// Root returns the absolute path used for local media browsing.
//
// Priority:
//  1. EMOSUP_LOCAL_ROOT (explicit override, Docker: /media)
//  2. config.aria2.download_dir (same as download cache by default)
//  3. {dataRoot}/downloads
//
// Binary runs: dataRoot is typically <repo>/backend/data → .../backend/data/downloads
// Docker runs: set EMOSUP_LOCAL_ROOT=/media and mount host media there.
func (s *LocalService) Root() string {
	downloadDir := ""
	dataRoot := ""
	if s.store != nil {
		dataRoot = s.store.Root()
		if cfg, err := s.store.LoadConfig(); err == nil {
			downloadDir = strings.TrimSpace(cfg.Aria2.DownloadDir)
		}
	}
	return ResolveLocalMediaRoot(downloadDir, dataRoot)
}

// ResolveLocalMediaRoot is shared by task creation and local browse.
func ResolveLocalMediaRoot(downloadDir, dataRoot string) string {
	if root := strings.TrimSpace(os.Getenv("EMOSUP_LOCAL_ROOT")); root != "" {
		return absPath(root)
	}
	if dir := strings.TrimSpace(downloadDir); dir != "" {
		return absPath(dir)
	}
	if root := strings.TrimSpace(dataRoot); root != "" {
		return absPath(filepath.Join(root, "downloads"))
	}
	// Last resort for bare binary without store: cwd/data/downloads
	return absPath(filepath.Join("data", "downloads"))
}

func absPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return filepath.Clean(path)
}

func (s *LocalService) resolve(relPath string) (absBase, fullPath, cleanRel string, err error) {
	absBase = s.Root()
	if absBase == "" {
		return "", "", "", errors.New("local media root is not configured")
	}

	// Auto-create root so first browse / binary start never fails with ENOENT.
	if err := utils.EnsureDir(absBase); err != nil {
		return "", "", "", fmt.Errorf("ensure local root %s: %w", absBase, err)
	}

	cleanRel = filepath.Clean(strings.TrimPrefix(filepath.Clean(relPath), "/"))
	if cleanRel == "." {
		cleanRel = ""
	}
	fullPath = filepath.Join(absBase, cleanRel)

	// Prevent directory traversal
	baseWithSep := filepath.Clean(absBase) + string(filepath.Separator)
	cleanedFull := filepath.Clean(fullPath)
	if cleanedFull != filepath.Clean(absBase) && !strings.HasPrefix(cleanedFull+string(filepath.Separator), baseWithSep) {
		return "", "", "", errors.New("access denied")
	}
	return absBase, fullPath, cleanRel, nil
}

func (s *LocalService) IsDir(_ context.Context, relPath string) bool {
	_, fullPath, _, err := s.resolve(relPath)
	if err != nil {
		return false
	}
	info, err := os.Stat(fullPath)
	return err == nil && info.IsDir()
}

func (s *LocalService) GetFileInfo(_ context.Context, relPath string) (LocalEntry, error) {
	_, fullPath, cleanRel, err := s.resolve(relPath)
	if err != nil {
		return LocalEntry{}, err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return LocalEntry{}, err
	}
	if info.IsDir() {
		return LocalEntry{}, errors.New("path is a directory, not a file")
	}

	display := "/"
	if cleanRel != "" {
		display = "/" + filepath.ToSlash(cleanRel)
	}

	return LocalEntry{
		Name:  info.Name(),
		Path:  display,
		IsDir: false,
		Size:  info.Size(),
	}, nil
}

func (s *LocalService) Browse(_ context.Context, relPath string) (string, []LocalEntry, error) {
	_, fullPath, cleanRel, err := s.resolve(relPath)
	if err != nil {
		return "", nil, err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return "", nil, err
	}
	if !info.IsDir() {
		return "", nil, fmt.Errorf("not a directory: %s", relPath)
	}

	dirEntries, err := os.ReadDir(fullPath)
	if err != nil {
		return "", nil, err
	}

	result := make([]LocalEntry, 0, len(dirEntries))
	for _, entry := range dirEntries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}

		entryPath := entry.Name()
		if cleanRel != "" {
			entryPath = filepath.Join(cleanRel, entry.Name())
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

	displayPath := "/"
	if cleanRel != "" {
		displayPath = "/" + filepath.ToSlash(cleanRel)
	}

	return displayPath, result, nil
}
