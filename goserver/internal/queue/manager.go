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
	m.logf("[INFO] 已加入队列：%s", task.Name)
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
			m.logf("[WARN] worker loop cancelled")
			return
		default:
		}

		task, ok := m.store.Dequeue()
		if !ok {
			m.logf("[INFO] worker loop exit: queue empty")
			return
		}

		now := time.Now()
		task.Status = domain.TaskStatusRunning
		task.Stage = domain.TaskStageRunning
		task.StatusText = "开始处理：" + task.Name
		task.CurrentFile = task.Name
		task.StartedAt = &now
		m.store.Update(task)
		m.logf("[INFO] 开始处理队列任务：%s", task.Name)
		m.publish(domain.Event{Type: domain.EventTaskStarted, TaskID: task.ID, Task: &task, Timestamp: now})
		m.publishSnapshot()

		select {
		case <-ctx.Done():
			finish := time.Now()
			task.Status = domain.TaskStatusCancelled
			task.Stage = domain.TaskStageIdle
			task.LastError = "cancelled"
			task.StatusText = "任务cancelled"
			task.FinishedAt = &finish
			m.store.Update(task)
			m.logf("[WARN] 任务取消：%s", task.Name)
			m.publish(domain.Event{Type: domain.EventTaskFinished, TaskID: task.ID, Task: &task, Timestamp: finish})
			m.publishSnapshot()
			return
		case <-time.After(2 * time.Second):
		}

		finish := time.Now()
		task.Status = domain.TaskStatusSuccess
		task.Stage = domain.TaskStageIdle
		task.StatusText = "任务完成"
		task.FinishedAt = &finish
		task.Download = domain.Progress{Percent: 100, Speed: "done", ETA: "N/A", Done: true}
		task.Upload = domain.Progress{Percent: 100, Speed: "done", ETA: "N/A", Done: true}
		m.store.Update(task)
		m.logf("[SUCCESS] 任务完成：%s", task.Name)
		m.publish(domain.Event{Type: domain.EventTaskFinished, TaskID: task.ID, Task: &task, Timestamp: finish})
		m.publishSnapshot()
	}
}

func (m *Manager) Snapshot() domain.Snapshot {
	return m.store.Snapshot()
}

func (m *Manager) LogsTail(n int) []string {
	return m.store.LogsTail(n)
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

func (m *Manager) logf(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	m.store.AppendLog(line)
	m.bus.Publish(domain.Event{Type: domain.EventLog, Message: line, Timestamp: time.Now()})
}
