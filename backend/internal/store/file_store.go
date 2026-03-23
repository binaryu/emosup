package store

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"emosup/backend/internal/model"
	"emosup/backend/internal/utils"
)

type FileStore struct {
	root string
	mu   sync.RWMutex
}

func NewFileStore(root string) *FileStore {
	return &FileStore{root: root}
}

func (s *FileStore) Init() error {
	dirs := []string{
		s.root,
		filepath.Join(s.root, "scans"),
		filepath.Join(s.root, "tasks"),
		filepath.Join(s.root, "logs"),
	}

	for _, dir := range dirs {
		if err := utils.EnsureDir(dir); err != nil {
			return err
		}
	}

	return nil
}

func (s *FileStore) LoadConfig() (model.AppConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.root, "config.json")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return model.DefaultAppConfig(), nil
	}

	var cfg model.AppConfig
	if err := utils.ReadJSON(path, &cfg); err != nil {
		return model.AppConfig{}, err
	}

	return model.NormalizeAppConfig(cfg), nil
}

func (s *FileStore) SaveConfig(cfg model.AppConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return utils.AtomicWriteJSON(filepath.Join(s.root, "config.json"), model.NormalizeAppConfig(cfg))
}

func (s *FileStore) SaveScan(scan model.ScanSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return utils.AtomicWriteJSON(filepath.Join(s.root, "scans", "scan_"+scan.ID+".json"), scan)
}

func (s *FileStore) GetScan(id string) (model.ScanSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var scan model.ScanSession
	err := utils.ReadJSON(filepath.Join(s.root, "scans", "scan_"+id+".json"), &scan)
	return scan, err
}

func (s *FileStore) UpdateScanItem(scanID, itemID string, updater func(*model.ScanItem) error) (model.ScanSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.root, "scans", "scan_"+scanID+".json")
	var scan model.ScanSession
	if err := utils.ReadJSON(path, &scan); err != nil {
		return model.ScanSession{}, err
	}

	found := false
	for index := range scan.Items {
		if scan.Items[index].ID != itemID {
			continue
		}

		if err := updater(&scan.Items[index]); err != nil {
			return model.ScanSession{}, err
		}
		found = true
		break
	}

	if !found {
		return model.ScanSession{}, os.ErrNotExist
	}

	scan.MatchedCount, scan.UnmatchedCount = recalculateScanCounts(scan.Items)
	scan.TotalCount = len(scan.Items)
	scan.UpdatedAt = time.Now()

	if err := utils.AtomicWriteJSON(path, scan); err != nil {
		return model.ScanSession{}, err
	}

	return scan, nil
}

func (s *FileStore) ListScans() ([]model.ScanSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pattern := filepath.Join(s.root, "scans", "scan_*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	scans := make([]model.ScanSession, 0, len(files))
	for _, file := range files {
		var scan model.ScanSession
		if err := utils.ReadJSON(file, &scan); err != nil {
			return nil, err
		}
		scans = append(scans, scan)
	}

	sort.Slice(scans, func(i, j int) bool {
		return scans[i].CreatedAt.After(scans[j].CreatedAt)
	})

	return scans, nil
}

func (s *FileStore) SaveTask(task model.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return utils.AtomicWriteJSON(filepath.Join(s.root, "tasks", "task_"+task.ID+".json"), task)
}

func (s *FileStore) GetTask(id string) (model.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var task model.Task
	err := utils.ReadJSON(filepath.Join(s.root, "tasks", "task_"+id+".json"), &task)
	return task, err
}

func (s *FileStore) UpdateTask(id string, updater func(*model.Task) error) (model.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.root, "tasks", "task_"+id+".json")
	var task model.Task
	if err := utils.ReadJSON(path, &task); err != nil {
		return model.Task{}, err
	}

	if err := updater(&task); err != nil {
		return model.Task{}, err
	}

	if err := utils.AtomicWriteJSON(path, task); err != nil {
		return model.Task{}, err
	}

	return task, nil
}

func (s *FileStore) ListTasks() ([]model.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pattern := filepath.Join(s.root, "tasks", "task_*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	tasks := make([]model.Task, 0, len(files))
	for _, file := range files {
		var task model.Task
		if err := utils.ReadJSON(file, &task); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	return tasks, nil
}

func (s *FileStore) SaveTaskLog(log model.TaskLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return utils.AtomicWriteJSON(filepath.Join(s.root, "logs", "task_"+log.TaskID+".json"), log)
}

func (s *FileStore) GetTaskLog(taskID string) (model.TaskLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var taskLog model.TaskLog
	err := utils.ReadJSON(filepath.Join(s.root, "logs", "task_"+taskID+".json"), &taskLog)
	if errors.Is(err, os.ErrNotExist) {
		return model.TaskLog{TaskID: taskID, Items: []model.TaskLogItem{}}, nil
	}

	return taskLog, err
}

func (s *FileStore) AppendTaskLog(taskID string, item model.TaskLogItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.root, "logs", "task_"+taskID+".json")
	taskLog := model.TaskLog{
		TaskID: taskID,
		Items:  []model.TaskLogItem{},
	}

	if err := utils.ReadJSON(path, &taskLog); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	taskLog.TaskID = taskID
	taskLog.Items = append(taskLog.Items, item)
	return utils.AtomicWriteJSON(path, taskLog)
}

func recalculateScanCounts(items []model.ScanItem) (matched int, unmatched int) {
	for _, item := range items {
		if item.MatchStatus == model.MatchStatusMatched {
			matched++
			continue
		}
		unmatched++
	}

	return matched, unmatched
}
