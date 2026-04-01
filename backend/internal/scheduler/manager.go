package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"emosup/backend/internal/model"
	"emosup/backend/internal/service"
)

type RuntimeStatus struct {
	SchedulerRunning bool       `json:"scheduler_running"`
	CurrentTaskID    string     `json:"current_task_id"`
	CurrentStage     string     `json:"current_stage"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
}

type Manager struct {
	taskService      *service.TaskService
	downloadExecutor *service.DownloadExecutor
	uploadExecutor   *service.UploadExecutor
	pollInterval     time.Duration

	mu            sync.RWMutex
	running       bool
	currentTaskID string
	currentStage  string
	startedAt     *time.Time
	lastRecovery  RecoverySummary
}

func NewManager(
	taskService *service.TaskService,
	downloadExecutor *service.DownloadExecutor,
	uploadExecutor *service.UploadExecutor,
	pollInterval time.Duration,
) *Manager {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	return &Manager{
		taskService:      taskService,
		downloadExecutor: downloadExecutor,
		uploadExecutor:   uploadExecutor,
		pollInterval:     pollInterval,
	}
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
	runtime := RuntimeStatus{
		SchedulerRunning: m.running,
		CurrentTaskID:    m.currentTaskID,
		CurrentStage:     m.currentStage,
		StartedAt:        m.startedAt,
	}
	m.mu.RUnlock()

	if runtime.CurrentTaskID != "" {
		if task, err := m.taskService.GetTask(context.Background(), runtime.CurrentTaskID); err == nil {
			runtime.CurrentStage = stageFromStatus(task.Status)
		}
	}

	return runtime
}

func (m *Manager) RecoverySummary() RecoverySummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastRecovery
}

func (m *Manager) tick(ctx context.Context) error {
	if m.CurrentTaskID() != "" {
		return nil
	}

	task, found, err := m.taskService.GetNextRunnableTask(ctx)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	log.Printf("scheduler picked task: %s status=%s", task.ID, task.Status)
	m.startTask(ctx, task)
	return nil
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
	m.setCurrentTask(task.ID, stage)

	go func() {
		defer m.clearCurrentTask()
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

func (m *Manager) CurrentTaskID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentTaskID
}

func (m *Manager) setCurrentTask(taskID string, stage string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.currentTaskID = taskID
	m.currentStage = stage
	m.startedAt = &now
}

func (m *Manager) clearCurrentTask() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.currentTaskID = ""
	m.currentStage = ""
	m.startedAt = nil
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
