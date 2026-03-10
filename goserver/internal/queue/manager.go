package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"emosup/goserver/internal/domain"
	"emosup/goserver/internal/events"
	"emosup/goserver/internal/store"
)

type Manager struct {
	store   *store.MemoryTaskStore
	bus     *events.Bus
	mu      sync.Mutex
	running bool
}

type EnqueueRequest struct {
	Name string `json:"name"`
}

func NewManager(taskStore *store.MemoryTaskStore, bus *events.Bus) *Manager {
	return &Manager{
		store: taskStore,
		bus:   bus,
	}
}

func (m *Manager) Enqueue(req EnqueueRequest) domain.Task {
	now := time.Now()
	task := domain.Task{
		ID:          fmt.Sprintf("task-%d", now.UnixNano()),
		Name:        req.Name,
		Status:      domain.TaskStatusPending,
		Stage:       domain.TaskStageQueued,
		StatusText:  "等待执行",
		CurrentFile: req.Name,
		Download:    domain.Progress{Speed: "0 MB/s", ETA: "N/A"},
		Upload:      domain.Progress{Speed: "0 MB/s", ETA: "N/A"},
		CreatedAt:   now,
	}
	m.store.Enqueue(task)
	m.publish(domain.Event{Type: domain.EventTaskQueued, TaskID: task.ID, Task: &task, Timestamp: now})
	m.publishSnapshot()
	m.EnsureWorker(context.Background())
	return task
}

func (m *Manager) EnsureWorker(ctx context.Context) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.store.SetWorkerRunning(true)
	m.mu.Unlock()

	go m.workerLoop(ctx)
}

func (m *Manager) workerLoop(ctx context.Context) {
	defer func() {
		m.mu.Lock()
		m.running = false
		m.store.SetWorkerRunning(false)
		m.mu.Unlock()
		m.publishSnapshot()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		task, ok := m.store.Dequeue()
		if !ok {
			return
		}

		now := time.Now()
		task.Status = domain.TaskStatusRunning
		task.Stage = domain.TaskStageRunning
		task.StatusText = "开始处理"
		task.StartedAt = &now
		m.store.Update(task)
		m.publish(domain.Event{Type: domain.EventTaskStarted, TaskID: task.ID, Task: &task, Timestamp: now})
		m.publishSnapshot()

		time.Sleep(2 * time.Second)

		finish := time.Now()
		task.Status = domain.TaskStatusSuccess
		task.Stage = domain.TaskStageIdle
		task.StatusText = "任务完成"
		task.FinishedAt = &finish
		task.Download = domain.Progress{Percent: 100, Speed: "done", ETA: "N/A", Done: true}
		task.Upload = domain.Progress{Percent: 100, Speed: "done", ETA: "N/A", Done: true}
		m.store.Update(task)
		m.publish(domain.Event{Type: domain.EventTaskFinished, TaskID: task.ID, Task: &task, Timestamp: finish})
		m.publishSnapshot()
	}
}

func (m *Manager) Snapshot() domain.Snapshot {
	return m.store.Snapshot()
}

func (m *Manager) publish(event domain.Event) {
	m.bus.Publish(event)
}

func (m *Manager) publishSnapshot() {
	snapshot := m.store.Snapshot()
	m.bus.Publish(domain.Event{
		Type:      domain.EventSnapshot,
		Snapshot:  &snapshot,
		Timestamp: time.Now(),
	})
}
