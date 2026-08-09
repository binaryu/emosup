package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"emosup/backend/internal/model"
	"emosup/backend/internal/store"
)

// CacheEntry describes one file in the download cache directory.
type CacheEntry struct {
	Path          string    `json:"path"`
	Name          string    `json:"name"`
	Size          int64     `json:"size"`
	ModifiedAt    time.Time `json:"modified_at"`
	IsTemp        bool      `json:"is_temp"`
	Referenced    bool      `json:"referenced"`
	TaskID        string    `json:"task_id,omitempty"`
	TaskStatus    string    `json:"task_status,omitempty"`
	TaskFileName  string    `json:"task_file_name,omitempty"`
	KeepLocalFile bool      `json:"keep_local_file"`
}

// CacheListResult is the full cache inventory response.
type CacheListResult struct {
	Dir            string       `json:"dir"`
	Entries        []CacheEntry `json:"entries"`
	TotalSize      int64        `json:"total_size"`
	OrphanCount    int          `json:"orphan_count"`
	ActiveRefCount int          `json:"active_ref_count"`
}

type CacheService struct {
	store *store.FileStore
}

func NewCacheService(store *store.FileStore) *CacheService {
	return &CacheService{store: store}
}

// cacheDir resolves the absolute download cache directory (same rules the
// download executor uses when writing files).
func (s *CacheService) cacheDir() (string, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return "", err
	}
	dir := strings.TrimSpace(cfg.Download.Dir)
	if dir == "" {
		return "", errors.New("下载缓存目录未配置")
	}
	return absPath(dir), nil
}

// List scans the download cache directory and marks every file with its
// referencing task (if any).
func (s *CacheService) List(_ context.Context) (CacheListResult, error) {
	dir, err := s.cacheDir()
	if err != nil {
		return CacheListResult{}, err
	}

	tasks, err := s.store.ListTasks()
	if err != nil {
		return CacheListResult{}, err
	}
	// Index tasks by normalized absolute local path.
	refByPath := make(map[string]model.Task)
	for _, task := range tasks {
		path := absPath(toContainerPath(strings.TrimSpace(task.Download.LocalPath)))
		if path == "" {
			continue
		}
		// A later task sharing the path wins (file names are unique per task).
		refByPath[path] = task
	}

	info, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return CacheListResult{Dir: dir, Entries: []CacheEntry{}}, nil
		}
		return CacheListResult{}, err
	}
	if !info.IsDir() {
		return CacheListResult{}, fmt.Errorf("下载缓存路径不是目录: %s", dir)
	}

	var entries []CacheEntry
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			// Skip the default BT seeding folder so torrents are never listed.
			if path != dir && strings.EqualFold(d.Name(), "BT") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		fileInfo, err := d.Info()
		if err != nil {
			return nil
		}

		entry := CacheEntry{
			Path:       path,
			Name:       d.Name(),
			Size:       fileInfo.Size(),
			ModifiedAt: fileInfo.ModTime(),
			IsTemp:     strings.HasSuffix(d.Name(), ".partmulti"),
		}
		if task, ok := refByPath[absPath(path)]; ok {
			entry.Referenced = true
			entry.TaskID = task.ID
			entry.TaskStatus = string(task.Status)
			entry.TaskFileName = task.Source.FileName
			entry.KeepLocalFile = task.KeepLocalFile
		}
		entries = append(entries, entry)
		return nil
	})

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Referenced != entries[j].Referenced {
			return !entries[i].Referenced // orphans first
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	result := CacheListResult{Dir: dir, Entries: entries}
	activeStatuses := map[model.TaskStatus]bool{
		model.TaskStatusDownloading: true,
		model.TaskStatusUploading:   true,
		model.TaskStatusSaving:      true,
	}
	for i := range entries {
		result.TotalSize += entries[i].Size
		if !entries[i].Referenced {
			result.OrphanCount++
		} else if activeStatuses[model.TaskStatus(entries[i].TaskStatus)] {
			result.ActiveRefCount++
		}
	}
	return result, nil
}

// Delete removes the given cache files. Files referenced by tasks that are
// currently running (downloading/uploading/saving) are refused; everything
// else (orphans, completed/canceled/failed/queued task files) can be removed.
func (s *CacheService) Delete(_ context.Context, paths []string) (deleted []string, failed map[string]string, err error) {
	dir, err := s.cacheDir()
	if err != nil {
		return nil, nil, err
	}
	absDir := absPath(dir)

	tasks, err := s.store.ListTasks()
	if err != nil {
		return nil, nil, err
	}
	activePaths := make(map[string]bool)
	for _, task := range tasks {
		switch task.Status {
		case model.TaskStatusDownloading, model.TaskStatusUploading, model.TaskStatusSaving:
			activePaths[absPath(toContainerPath(strings.TrimSpace(task.Download.LocalPath)))] = true
		}
	}

	deleted = make([]string, 0, len(paths))
	failed = make(map[string]string)
	for _, raw := range paths {
		path := absPath(strings.TrimSpace(raw))
		rel, relErr := filepath.Rel(absDir, path)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			failed[path] = "路径不在下载缓存目录内"
			continue
		}
		if activePaths[path] {
			failed[path] = "任务正在使用中（下载/上传），请先取消任务"
			continue
		}
		if err := os.Remove(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				deleted = append(deleted, path)
			} else {
				failed[path] = err.Error()
			}
			continue
		}
		deleted = append(deleted, path)
		// Best-effort cleanup of now-empty parent dirs.
		parent := filepath.Dir(path)
		for parent != absDir && strings.HasPrefix(parent+string(filepath.Separator), absDir+string(filepath.Separator)) {
			if os.Remove(parent) != nil {
				break
			}
			parent = filepath.Dir(parent)
		}
	}
	return deleted, failed, nil
}
