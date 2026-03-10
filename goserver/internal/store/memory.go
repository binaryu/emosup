package store

import (
	"sync"
	"time"

	"emosup/goserver/internal/domain"
)

type MemoryTaskStore struct {
	mu            sync.RWMutex
	tasks         map[string]domain.Task
	order         []string
	queue         []string
	currentTaskID string
	workerRunning bool
}

func NewMemoryTaskStore() *MemoryTaskStore {
	return &MemoryTaskStore{
		tasks: make(map[string]domain.Task),
		order: make([]string, 0),
		queue: make([]string, 0),
	}
}

func (s *MemoryTaskStore) Enqueue(task domain.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks[task.ID] = task
	s.order = append(s.order, task.ID)
	s.queue = append(s.queue, task.ID)
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
	return task, true
}

func (s *MemoryTaskStore) Update(task domain.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks[task.ID] = task
	if task.Status == domain.TaskStatusSuccess || task.Status == domain.TaskStatusFailed || task.Status == domain.TaskStatusCancelled {
		if s.currentTaskID == task.ID {
			s.currentTaskID = ""
		}
	}
}

func (s *MemoryTaskStore) SetWorkerRunning(running bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !running {
		s.currentTaskID = ""
	}
	s.workerRunning = running
}

func (s *MemoryTaskStore) WorkerRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.workerRunning
}

func (s *MemoryTaskStore) Snapshot() domain.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]domain.Task, 0, len(s.order))
	for _, id := range s.order {
		if task, ok := s.tasks[id]; ok {
			tasks = append(tasks, task)
		}
	}

	return domain.Snapshot{
		WorkerRunning: s.workerRunning,
		QueueSize:     len(s.queue),
		CurrentTaskID: s.currentTaskID,
		Tasks:         tasks,
		UpdatedAt:     time.Now(),
	}
}
