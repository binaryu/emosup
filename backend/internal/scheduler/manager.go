package scheduler

import (
	"context"
	"log"
	"sync"
	"syscall"
	"time"

	"emosup/backend/internal/eventbus"
	"emosup/backend/internal/model"
	"emosup/backend/internal/service"
)

type RuntimeStatus struct {
	SchedulerRunning bool     `json:"scheduler_running"`
	CurrentTaskIDs   []string `json:"current_task_ids"`
	CurrentStage     string   `json:"current_stage"`
	MaxConcurrency   int      `json:"max_concurrency"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
}

type Manager struct {
	taskService      *service.TaskService
	downloadExecutor *service.DownloadExecutor
	uploadExecutor   *service.UploadExecutor
	pollInterval     time.Duration
	maxConcurrency   int
	eventBus         *eventbus.Bus
	tuner            *service.Tuner

	mu           sync.RWMutex
	running      bool
	activeTasks  map[string]string // taskID -> stage
	startedAt    *time.Time
	lastRecovery RecoverySummary
}

func NewManager(
	taskService *service.TaskService,
	downloadExecutor *service.DownloadExecutor,
	uploadExecutor *service.UploadExecutor,
	pollInterval time.Duration,
	maxConcurrency int,
	eventBus *eventbus.Bus,
	tuner *service.Tuner,
) *Manager {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	if maxConcurrency <= 0 {
		maxConcurrency = 1
	}

	return &Manager{
		taskService:      taskService,
		downloadExecutor: downloadExecutor,
		uploadExecutor:   uploadExecutor,
		pollInterval:     pollInterval,
		maxConcurrency:   maxConcurrency,
		eventBus:         eventBus,
		tuner:            tuner,
		activeTasks:      make(map[string]string),
	}
}

// downloadCeiling returns how many download tasks may run in parallel: the
// user cap, raised by the auto-tuner based on measured bandwidth. The number
// actually started is further gated by free disk space in tick(), so raising
// this can never fill the disk.
func (m *Manager) downloadCeiling() int {
	ceiling := m.maxConcurrency
	if m.tuner != nil {
		if snap := m.tuner.Snapshot(); snap.Enabled {
			ceiling = max(ceiling, snap.DownloadConcurrency)
		}
	}
	return ceiling
}

// uploadCeiling returns how many upload tasks may run in parallel: the user
// cap, raised by the auto-tuner. Uploads are bandwidth-bound (they free disk
// as they complete), so raising them is safe.
func (m *Manager) uploadCeiling() int {
	ceiling := m.maxConcurrency
	if m.tuner != nil {
		if snap := m.tuner.Snapshot(); snap.Enabled {
			ceiling = max(ceiling, snap.UploadConcurrency)
		}
	}
	return ceiling
}

// downloadDiskHeadroom keeps this much free space untouched by downloads.
const downloadDiskHeadroom = int64(5e9)

// taskDiskBytes returns the expected on-disk footprint of a task; 0 when the
// size is unknown (the per-task download guard still applies then).
func taskDiskBytes(task model.Task) int64 {
	if task.Download.TotalBytes > 0 {
		return task.Download.TotalBytes
	}
	if task.Download.CompletedBytes > 0 {
		return task.Download.CompletedBytes
	}
	return task.Source.FileSize
}

// diskAllowsDownload reports whether free disk can hold the next file on top
// of what is already committed, keeping downloadDiskHeadroom free.
func diskAllowsDownload(free, committed, next int64) bool {
	return free >= committed+next+downloadDiskHeadroom
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m *Manager) Start(ctx context.Context) {
	m.setRunning(true)
	defer m.setRunning(false)

	log.Printf("scheduler started with poll interval %s", m.pollInterval)
	summary, err := m.recover(ctx)
	if err != nil {
		log.Printf("scheduler recovery failed: %v", err)
	} else {
		m.setRecoverySummary(summary)
		log.Printf("scheduler recovery summary: %+v", summary)
	}

	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("scheduler stopped")
			return
		case <-ticker.C:
			if err := m.tick(ctx); err != nil {
				log.Printf("scheduler tick failed: %v", err)
			}
		}
	}
}

func (m *Manager) RuntimeStatus() RuntimeStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	taskIDs := make([]string, 0, len(m.activeTasks))
	for id := range m.activeTasks {
		taskIDs = append(taskIDs, id)
	}

	return RuntimeStatus{
		SchedulerRunning: m.running,
		CurrentTaskIDs:   taskIDs,
		MaxConcurrency:   m.maxConcurrency,
		StartedAt:        m.startedAt,
	}
}

func (m *Manager) RecoverySummary() RecoverySummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastRecovery
}

func (m *Manager) tick(ctx context.Context) error {
	// Dynamically read max concurrency from config so changes take effect without restart
	cfg, err := m.taskService.LoadConfig(ctx)
	if err != nil {
		log.Printf("scheduler failed to load config: %v", err)
		return err
	}
	maxConc := cfg.Worker.MaxConcurrency
	if maxConc <= 0 {
		maxConc = 1
	}

	m.mu.Lock()
	m.maxConcurrency = maxConc
	activeIDs := make(map[string]struct{}, len(m.activeTasks))
	dlActive, ulActive := 0, 0
	for id, stage := range m.activeTasks {
		activeIDs[id] = struct{}{}
		switch stage {
		case "download":
			dlActive++
		default:
			ulActive++
		}
	}
	m.mu.Unlock()

	// 1) Uploads first, from their own pool (bandwidth-bound, no disk risk):
	// completed downloads are uploaded promptly instead of piling up on disk.
	ulCeiling := m.uploadCeiling()
	for ulActive < ulCeiling {
		task, found, err := m.taskService.GetNextUploadTask(ctx, activeIDs)
		if err != nil {
			return err
		}
		if !found {
			break
		}
		log.Printf("scheduler picked upload task: %s status=%s", task.ID, task.Status)
		m.startTask(ctx, task)
		activeIDs[task.ID] = struct{}{}
		ulActive++
	}

	// 2) Downloads: capped at the user max raised by the auto-tuner, AND
	// gated by free disk space using real task sizes, so concurrent downloads
	// can never fill the disk.
	dlCeiling := m.downloadCeiling()
	if dlActive >= dlCeiling {
		return nil
	}
	committed, err := m.diskCommittedBytes(ctx, activeIDs)
	if err != nil {
		return err
	}
	free := m.freeDiskBytes(cfg.Download.Dir)
	diskKnown := free >= 0
	for dlActive < dlCeiling {
		task, found, err := m.taskService.GetNextDownloadTask(ctx, activeIDs)
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		next := taskDiskBytes(task)
		if diskKnown && !diskAllowsDownload(free, committed, next) {
			// Disk cannot hold the next file — stop admitting downloads this
			// tick; uploads (step 1) keep draining parked files.
			return nil
		}
		log.Printf("scheduler picked download task: %s status=%s", task.ID, task.Status)
		m.startTask(ctx, task)
		activeIDs[task.ID] = struct{}{}
		committed += next
		dlActive++
	}

	return nil
}

// diskCommittedBytes sums the on-disk footprint of downloads in flight and
// files that are downloaded but not yet uploaded (parked awaiting an upload
// slot). Queued-but-not-started tasks are not counted; the tick adds their
// size as it admits them.
func (m *Manager) diskCommittedBytes(ctx context.Context, activeIDs map[string]struct{}) (int64, error) {
	tasks, err := m.taskService.ListAllTasks(ctx)
	if err != nil {
		return 0, err
	}
	var committed int64
	for _, task := range tasks {
		_, active := activeIDs[task.ID]
		switch {
		case active && (task.Status == model.TaskStatusQueued ||
			task.Status == model.TaskStatusDownloading ||
			task.Status == model.TaskStatusDownloadCompleted):
			committed += taskDiskBytes(task)
		case active && (task.Status == model.TaskStatusUploadPending ||
			task.Status == model.TaskStatusUploading ||
			task.Status == model.TaskStatusSaving):
			// Uploading/saving tasks still hold the file on disk until the
			// upload finishes and deletes it.
			committed += taskDiskBytes(task)
		case !active && task.Status == model.TaskStatusUploadPending:
			committed += taskDiskBytes(task)
		}
	}
	return committed, nil
}

// freeDiskBytes returns free bytes in dir; -1 when it cannot be measured
// (empty dir or statfs error), which disables the download admission gate.
func (m *Manager) freeDiskBytes(dir string) int64 {
	if dir == "" {
		return -1
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		log.Printf("scheduler disk check failed: dir=%s err=%v", dir, err)
		return -1
	}
	return int64(stat.Bavail) * int64(stat.Bsize)
}

func (m *Manager) recover(ctx context.Context) (RecoverySummary, error) {
	log.Println("recovery started")
	defer log.Println("recovery completed")

	tasks, err := m.taskService.ListAllTasks(ctx)
	if err != nil {
		return RecoverySummary{}, err
	}

	summary := RecoverySummary{Total: len(tasks)}
	var resumeTask *model.Task

	for _, task := range tasks {
		log.Printf("task recovery begin: task=%s status=%s", task.ID, task.Status)
		switch task.Status {
		case model.TaskStatusQueued:
			summary.Queued++
		case model.TaskStatusDownloading:
			shouldResume, recoverErr := m.downloadExecutor.RecoverTask(ctx, task)
			if recoverErr != nil {
				summary.DownloadingFailed++
				return summary, recoverErr
			}
			if shouldResume {
				summary.DownloadingRecovered++
				if resumeTask == nil {
					taskCopy := task
					resumeTask = &taskCopy
				} else {
					if _, err := m.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "recovery", "download_failed", "multiple downloading tasks detected during recovery"); err != nil {
						return summary, err
					}
					summary.DownloadingFailed++
				}
			}
		case model.TaskStatusDownloadCompleted:
			if _, err := m.downloadExecutor.RecoverTask(ctx, task); err != nil {
				return summary, err
			}
			summary.DownloadCompleted++
		case model.TaskStatusUploading:
			if _, err := m.taskService.RecoverInterruptedUpload(ctx, task.ID); err != nil {
				return summary, err
			}
			summary.UploadingFailed++
		case model.TaskStatusSaving:
			recoveredTask, err := m.taskService.RecoverSavingTask(ctx, task.ID)
			if err != nil {
				summary.SavingFailed++
				return summary, err
			}
			summary.SavingResumed++
			if resumeTask == nil {
				taskCopy := recoveredTask
				resumeTask = &taskCopy
			}
		case model.TaskStatusUploadPending:
			summary.UploadPending++
			if resumeTask == nil {
				taskCopy := task
				resumeTask = &taskCopy
			}
		}
		log.Printf("task recovery result: task=%s status=%s", task.ID, task.Status)
	}

	if resumeTask != nil {
		log.Printf("scheduler resumed task: %s status=%s", resumeTask.ID, resumeTask.Status)
		m.startTask(ctx, *resumeTask)
	}
	return summary, nil
}

func (m *Manager) startTask(ctx context.Context, task model.Task) {
	stage := stageFromStatus(task.Status)
	m.setActiveTask(task.ID, stage)

	go func() {
		defer m.clearActiveTask(task.ID)
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("task execution panic recovered: task=%s stage=%s panic=%v", task.ID, stage, recovered)
				switch stage {
				case "download":
					_, _ = m.taskService.MarkDownloadFailedWithDetails(context.Background(), task.ID, "system", "scheduler_panic", "scheduler panic while executing download task")
				default:
					_, _ = m.taskService.MarkUploadFailedWithDetails(context.Background(), task.ID, "system", "scheduler_panic", "scheduler panic while executing upload task")
				}
			}
		}()

		var err error
		switch task.Status {
		case model.TaskStatusQueued, model.TaskStatusDownloading, model.TaskStatusDownloadCompleted:
			err = m.downloadExecutor.Execute(ctx, task.ID)
		case model.TaskStatusUploadPending, model.TaskStatusSaving:
			err = m.uploadExecutor.Execute(ctx, task.ID)
		default:
			return
		}

		if err != nil && ctx.Err() == nil {
			log.Printf("task execution failed: task=%s stage=%s err=%v", task.ID, stage, err)
		}
	}()
}

func (m *Manager) setActiveTask(taskID string, stage string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeTasks == nil {
		m.activeTasks = make(map[string]string)
	}
	m.activeTasks[taskID] = stage
	if m.startedAt == nil {
		now := time.Now()
		m.startedAt = &now
	}
	if m.eventBus != nil {
		m.eventBus.Publish(eventbus.TaskEvent{TaskID: taskID, Status: stage})
	}
}

func (m *Manager) clearActiveTask(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.activeTasks, taskID)
	if len(m.activeTasks) == 0 {
		m.startedAt = nil
	}
	if m.eventBus != nil {
		m.eventBus.Publish(eventbus.TaskEvent{TaskID: taskID, Status: "done"})
	}
}

func (m *Manager) setRecoverySummary(summary RecoverySummary) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastRecovery = summary
}

func (m *Manager) setRunning(running bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = running
}

func stageFromStatus(status model.TaskStatus) string {
	switch status {
	case model.TaskStatusQueued, model.TaskStatusDownloading, model.TaskStatusDownloadCompleted:
		return "download"
	case model.TaskStatusUploadPending, model.TaskStatusUploading:
		return "uploading"
	case model.TaskStatusSaving:
		return "saving"
	default:
		return ""
	}
}
