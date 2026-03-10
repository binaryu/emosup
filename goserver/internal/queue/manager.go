package queue

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"emosup/goserver/internal/domain"
	"emosup/goserver/internal/events"
	"emosup/goserver/internal/service"
	"emosup/goserver/internal/store"
)

type Manager struct {
	store         *store.MemoryTaskStore
	bus           *events.Bus
	aria2Service  *service.Aria2Service
	uploadService *service.UploadService
	mu            sync.Mutex
	running       bool
	cancelFn      context.CancelFunc
	workerCtx     context.Context
}

func NewManager(taskStore *store.MemoryTaskStore, bus *events.Bus) *Manager {
	return &Manager{
		store:         taskStore,
		bus:           bus,
		aria2Service:  service.NewAria2Service(),
		uploadService: service.NewUploadService(),
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

		if strings.ToLower(task.MatchStatus) != "ok" && task.MatchStatus != "" {
			finish := time.Now()
			task.Status = domain.TaskStatusFailed
			task.Stage = domain.TaskStageIdle
			task.LastError = fmt.Sprintf("匹配失败/冲突 status=%s msg=%s", task.MatchStatus, task.MatchText)
			task.StatusText = "任务failed"
			task.FinishedAt = &finish
			m.store.Update(task)
			m.logf("[ERROR] 匹配失败：%s | %s", task.Name, task.LastError)
			m.publish(domain.Event{Type: domain.EventTaskFinished, TaskID: task.ID, Task: &task, Timestamp: finish})
			m.publishSnapshot()
			continue
		}
		if task.ServerHasMedia != nil && *task.ServerHasMedia && !task.ForceUpload {
			finish := time.Now()
			task.Status = domain.TaskStatusSkipped
			task.Stage = domain.TaskStageIdle
			task.LastError = "预检查：已有资源，跳过"
			task.StatusText = "任务skipped"
			task.FinishedAt = &finish
			m.store.Update(task)
			m.logf("[WARN] 预检查跳过：%s", task.Name)
			m.publish(domain.Event{Type: domain.EventTaskFinished, TaskID: task.ID, Task: &task, Timestamp: finish})
			m.publishSnapshot()
			continue
		}

		source := strings.ToLower(task.Source)
		cachePath := task.LocalPath
		if source == "local" {
			if cachePath == "" {
				finish := time.Now()
				task.Status = domain.TaskStatusFailed
				task.Stage = domain.TaskStageIdle
				task.LastError = "本地文件不存在"
				task.StatusText = "任务failed"
				task.FinishedAt = &finish
				m.store.Update(task)
				m.logf("[ERROR] 本地文件不存在：%s", task.Name)
				m.publish(domain.Event{Type: domain.EventTaskFinished, TaskID: task.ID, Task: &task, Timestamp: finish})
				m.publishSnapshot()
				continue
			}
			task.StatusText = "本地直传：" + filepath.Base(cachePath)
			m.store.Update(task)
			m.publishSnapshot()
		} else if task.OLPath != "" && task.Aria2RPCURL != "" {
			_ = m.aria2Service.CheckVersion(task.Aria2RPCURL, task.Aria2RPCSecret)
			task.Stage = domain.TaskStageDownload
			task.StatusText = "下载中：" + task.Name
			m.store.Update(task)
			m.publishSnapshot()
			downloadURL := strings.TrimRight(task.OpenListBaseURL, "/") + "/d/" + strings.TrimLeft(task.OLPath, "/")
			cachePath = filepath.Join(task.CacheDir, task.Name)
			err := m.aria2Service.DownloadAndMonitor(ctx, task.Aria2RPCURL, task.Aria2RPCSecret, downloadURL, cachePath, task.DownloadThreads, func(progress service.Aria2Progress) {
				m.store.UpdateDownloadProgress(task.ID, domain.Progress{
					Percent: progress.Percent,
					Speed:   progress.Speed,
					ETA:     progress.ETA,
					Done:    progress.Done,
				}, fmt.Sprintf("下载中 %.1f%% | %s | %s", progress.Percent, progress.Speed, task.Name))
				m.publishSnapshot()
			})
			if err != nil && ctx.Err() == nil {
				finish := time.Now()
				task.Status = domain.TaskStatusFailed
				task.Stage = domain.TaskStageIdle
				task.LastError = err.Error()
				task.StatusText = "任务failed"
				task.FinishedAt = &finish
				m.store.Update(task)
				m.logf("[ERROR] 下载失败：%s | %v", task.Name, err)
				m.publish(domain.Event{Type: domain.EventTaskFinished, TaskID: task.ID, Task: &task, Timestamp: finish})
				m.publishSnapshot()
				continue
			}
		}

		uploadOK := false
		saveOK := false
		if cachePath != "" && task.EmosAPIBase != "" && task.EmosToken != "" {
			task.Stage = domain.TaskStageUpload
			task.StatusText = "上传中：" + filepath.Base(cachePath)
			m.store.Update(task)
			m.publishSnapshot()
			token, err := m.uploadService.GetToken(task.EmosAPIBase, task.EmosToken, cachePath, task.Storage)
			if err != nil {
				finish := time.Now()
				task.Status = domain.TaskStatusFailed
				task.Stage = domain.TaskStageIdle
				task.LastError = err.Error()
				task.StatusText = "任务failed"
				task.FinishedAt = &finish
				m.store.Update(task)
				m.logf("[ERROR] 获取上传令牌失败：%s | %v", task.Name, err)
				m.publish(domain.Event{Type: domain.EventTaskFinished, TaskID: task.ID, Task: &task, Timestamp: finish})
				m.publishSnapshot()
				continue
			}
			err = m.uploadService.UploadStreamChunked(ctx, cachePath, token.UploadURL, task.ChunkSizeMB, func(progress service.UploadProgress) {
				m.store.UpdateUploadProgress(task.ID, domain.Progress{
					Percent: progress.Percent,
					Speed:   progress.Speed,
					ETA:     progress.ETA,
					Done:    progress.Done,
				}, fmt.Sprintf("上传中 %.1f%% | %s", progress.Percent, progress.Speed))
				m.publishSnapshot()
			})
			if err != nil && ctx.Err() == nil {
				finish := time.Now()
				task.Status = domain.TaskStatusFailed
				task.Stage = domain.TaskStageIdle
				task.LastError = err.Error()
				task.StatusText = "任务failed"
				task.FinishedAt = &finish
				m.store.Update(task)
				m.logf("[ERROR] 上传失败：%s | %v", task.Name, err)
				m.publish(domain.Event{Type: domain.EventTaskFinished, TaskID: task.ID, Task: &task, Timestamp: finish})
				m.publishSnapshot()
				m.logf("[WARN] 未完全成功：保留缓存用于续传/重试 -> %s", cachePath)
				continue
			}
			uploadOK = true
			if task.ServerItemType != "" && task.ServerItemID != nil {
				err = m.uploadService.SaveUpload(task.EmosAPIBase, task.EmosToken, task.ServerItemType, *task.ServerItemID, token.FileID)
				if err != nil {
					finish := time.Now()
					task.Status = domain.TaskStatusFailed
					task.Stage = domain.TaskStageIdle
					task.LastError = err.Error()
					task.StatusText = "任务failed"
					task.FinishedAt = &finish
					m.store.Update(task)
					m.logf("[ERROR] 保存上传结果失败：%s | %v", task.Name, err)
					m.publish(domain.Event{Type: domain.EventTaskFinished, TaskID: task.ID, Task: &task, Timestamp: finish})
					m.publishSnapshot()
					m.logf("[WARN] 未完全成功：保留缓存用于续传/重试 -> %s", cachePath)
					continue
				}
				saveOK = true
			}
		}

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
		case <-time.After(500 * time.Millisecond):
		}

		finish := time.Now()
		task.Status = domain.TaskStatusSuccess
		task.Stage = domain.TaskStageIdle
		task.StatusText = "任务完成"
		task.FinishedAt = &finish
		task.Download = domain.Progress{Percent: 100, Speed: "done", ETA: "N/A", Done: true}
		task.Upload = domain.Progress{Percent: 100, Speed: "done", ETA: "N/A", Done: true}
		m.store.Update(task)
		if uploadOK && saveOK {
			if source == "local" {
				m.logf("[INFO] 本地文件上传完成（保留源文件）")
			} else {
				_ = service.SafeRemove(cachePath)
				_ = service.SafeRemove(cachePath + ".aria2")
				m.logf("[INFO] 已删除缓存文件(.aria2 也清理)")
			}
		}
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
