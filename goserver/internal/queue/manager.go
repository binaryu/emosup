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
	store     *store.MemoryTaskStore
	bus       *events.Bus
	mu        sync.Mutex
	running   bool
	cancelFn  context.CancelFunc
	workerCtx context.Context
}

func NewManager(taskStore *store.MemoryTaskStore, bus *events.Bus) *Manager {
	return &Manager{
		store: taskStore,
		bus:   bus,
	}
}

func (m *Manager) Enqueue(req EnqueueRequest) any {
	if len(req.Files) > 0 {
		return m.enqueueBatch(req)
	}
	return m.enqueueSimple(req)
}

func (m *Manager) enqueueSimple(req EnqueueRequest) domain.Task {
	now := time.Now()
	task := domain.Task{
		ID:          buildTaskID(now, 0),
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

func (m *Manager) enqueueBatch(req EnqueueRequest) QueueAddResult {
	now := time.Now()
	queuedIDs := make([]string, 0)
	added := 0
	skipped := 0
	for idx, item := range req.Files {
		if !item.Selected {
			skipped++
			continue
		}
		id := buildTaskID(now, idx)
		task := item.toTask(id, req, now)
		m.store.Enqueue(task)
		m.logf("[INFO] 已加入队列：%s", task.Name)
		m.publish(domain.Event{Type: domain.EventTaskQueued, TaskID: task.ID, Task: &task, Timestamp: now})
		queuedIDs = append(queuedIDs, id)
		added++
	}
	m.publishSnapshot()
	m.EnsureWorker(context.Background())
	return QueueAddResult{Added: added, Skipped: skipped, QueuedIDs: queuedIDs}
}

func (m *Manager) EnsureWorker(parent context.Context) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	m.workerCtx = ctx
	m.cancelFn = cancel
	m.running = true
	m.store.SetWorkerRunning(true)
	m.mu.Unlock()

	go m.workerLoop(ctx)
}

func (m *Manager) CancelCurrent() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running || m.store.Snapshot().CurrentTaskID == "" {
		m.logf("[INFO] cancel ignored: no running task")
		return map[string]string{"status": "no_task"}
	}
	m.store.SetCancelRequested(true)
	if m.cancelFn != nil {
		m.cancelFn()
	}
	m.logf("[WARN] cancel requested")
	m.publishSnapshot()
	return map[string]string{"status": "cancelling"}
}

func (m *Manager) workerLoop(ctx context.Context) {
	defer func() {
		m.mu.Lock()
		m.running = false
		m.cancelFn = nil
		m.workerCtx = nil
		m.store.SetWorkerRunning(false)
		m.store.SetCancelRequested(false)
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
