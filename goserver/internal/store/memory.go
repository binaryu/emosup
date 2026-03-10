package store

import (
	"sync"
	"time"

	"emosup/goserver/internal/domain"
)

type MemoryTaskStore struct {
	mu              sync.RWMutex
	tasks           map[string]domain.Task
	order           []string
	queue           []string
	currentTaskID   string
	workerRunning   bool
	cancelRequested bool
	stage           domain.TaskStage
	currentFile     string
	statusText      string
	lastError       string
	download        domain.Progress
	upload          domain.Progress
	totalFiles      int
	completedFiles  int
	logs            []string
}

func NewMemoryTaskStore() *MemoryTaskStore {
	return &MemoryTaskStore{
		tasks:      make(map[string]domain.Task),
		order:      make([]string, 0),
		queue:      make([]string, 0),
		logs:       make([]string, 0),
		stage:      domain.TaskStageIdle,
		statusText: "空闲",
		download:   domain.Progress{Speed: "0 MB/s", ETA: "N/A"},
		upload:     domain.Progress{Speed: "0 MB/s", ETA: "N/A"},
	}
}

func (s *MemoryTaskStore) Enqueue(task domain.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks[task.ID] = task
	s.order = append(s.order, task.ID)
	s.queue = append(s.queue, task.ID)
	s.totalFiles++
	if !s.workerRunning && len(s.queue) > 0 {
		s.stage = domain.TaskStageQueued
	}
}

func (s *MemoryTaskStore) Dequeue() (domain.Task, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.queue) == 0 {
		return domain.Task{}, false
	}

	id := s.queue[0]
	s.queue = s.queue[1:]
	task := s.tasks[id]
	s.currentTaskID = id
	s.currentFile = task.Name
	s.statusText = "开始处理：" + task.Name
	s.stage = domain.TaskStageRunning
	s.lastError = ""
	s.cancelRequested = false
	s.download = domain.Progress{Speed: "0 MB/s", ETA: "N/A"}
	s.upload = domain.Progress{Speed: "0 MB/s", ETA: "N/A"}
	return task, true
}

func (s *MemoryTaskStore) Update(task domain.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks[task.ID] = task
	if task.ID == s.currentTaskID {
		s.currentFile = task.CurrentFile
		s.statusText = task.StatusText
		s.lastError = task.LastError
		s.stage = task.Stage
		s.download = task.Download
		s.upload = task.Upload
	}
	if task.Status == domain.TaskStatusSuccess || task.Status == domain.TaskStatusFailed || task.Status == domain.TaskStatusCancelled || task.Status == domain.TaskStatusSkipped {
		if s.currentTaskID == task.ID {
			s.currentTaskID = ""
			s.currentFile = ""
			s.stage = domain.TaskStageIdle
			s.statusText = "任务" + string(task.Status)
			s.lastError = task.LastError
			s.download = domain.Progress{Speed: "0 MB/s", ETA: "N/A"}
			s.upload = domain.Progress{Speed: "0 MB/s", ETA: "N/A"}
		}
		s.completedFiles++
	}
}

func (s *MemoryTaskStore) UpdateDownloadProgress(taskID string, progress domain.Progress, statusText string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task, ok := s.tasks[taskID]; ok {
		task.Download = progress
		task.StatusText = statusText
		task.Stage = domain.TaskStageDownload
		s.tasks[taskID] = task
	}
	if s.currentTaskID == taskID {
		s.download = progress
		s.statusText = statusText
		s.stage = domain.TaskStageDownload
	}
}

func (s *MemoryTaskStore) SetWorkerRunning(running bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !running {
		s.currentTaskID = ""
		s.currentFile = ""
		s.stage = domain.TaskStageIdle
		if len(s.queue) == 0 {
			s.statusText = "空闲"
		}
	}
	s.workerRunning = running
}

func (s *MemoryTaskStore) SetCancelRequested(cancel bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancelRequested = cancel
}

func (s *MemoryTaskStore) CancelRequested() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cancelRequested
}

func (s *MemoryTaskStore) AppendLog(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, line)
	if len(s.logs) > 300 {
		s.logs = s.logs[len(s.logs)-300:]
	}
}

func (s *MemoryTaskStore) LogsTail(n int) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if n <= 0 || len(s.logs) <= n {
		return append([]string(nil), s.logs...)
	}
	return append([]string(nil), s.logs[len(s.logs)-n:]...)
}

func (s *MemoryTaskStore) Snapshot() domain.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]domain.Task, 0, len(s.order))
	recent := make([]domain.Task, 0, 20)
	counts := domain.TaskCounts{}
	for _, id := range s.order {
		if task, ok := s.tasks[id]; ok {
			tasks = append(tasks, task)
			switch task.Status {
			case domain.TaskStatusPending:
				counts.Pending++
			case domain.TaskStatusRunning:
				counts.Running++
			case domain.TaskStatusSuccess:
				counts.Success++
			case domain.TaskStatusFailed:
				counts.Failed++
			case domain.TaskStatusSkipped:
				counts.Skipped++
			case domain.TaskStatusCancelled:
				counts.Cancelled++
			}
		}
	}
	start := 0
	if len(tasks) > 20 {
		start = len(tasks) - 20
	}
	recent = append(recent, tasks[start:]...)

	return domain.Snapshot{
		WorkerRunning:   s.workerRunning,
		CancelRequested: s.cancelRequested,
		Stage:           s.stage,
		TotalFiles:      s.totalFiles,
		CompletedFiles:  s.completedFiles,
		QueueSize:       len(s.queue),
		CurrentTaskID:   s.currentTaskID,
		CurrentFile:     s.currentFile,
		StatusText:      s.statusText,
		LastError:       s.lastError,
		Download:        s.download,
		Upload:          s.upload,
		RecentTasks:     recent,
		Counts:          counts,
		Tasks:           tasks,
		UpdatedAt:       time.Now(),
	}
}
