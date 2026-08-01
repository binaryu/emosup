package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"emosup/backend/internal/client"
	"emosup/backend/internal/model"
	"emosup/backend/internal/store"
	"emosup/backend/internal/utils"
)

type TaskService struct {
	store          *store.FileStore
	aria2Client    client.Aria2Client
	openListClient client.OpenListClient

	runMu     sync.Mutex
	runCancel map[string]*taskRunHandle
}

type taskRunHandle struct {
	cancel context.CancelFunc
}

type TaskServiceError struct {
	Code    int
	Message string
}

func (e *TaskServiceError) Error() string {
	return e.Message
}

type BatchCreateTasksRequest struct {
	ScanSessionID string   `json:"scan_session_id"`
	ItemIDs       []string `json:"item_ids"`
}

type BatchCreateTasksResult struct {
	Created []CreatedTaskItem `json:"created"`
	Failed  []FailedTaskItem  `json:"failed"`
}

type CreatedTaskItem struct {
	TaskID string `json:"task_id"`
	ItemID string `json:"item_id"`
}

type FailedTaskItem struct {
	ItemID string `json:"item_id"`
	Reason string `json:"reason"`
}

type ListTasksRequest struct {
	Status   string
	Page     int
	PageSize int
}

func NewTaskService(store *store.FileStore, aria2Client client.Aria2Client, openListClients ...client.OpenListClient) *TaskService {
	var openListClient client.OpenListClient
	if len(openListClients) > 0 {
		openListClient = openListClients[0]
	}

	return &TaskService{
		store:          store,
		aria2Client:    aria2Client,
		openListClient: openListClient,
	}
}

func (s *TaskService) RegisterTaskRun(taskID string, cancel context.CancelFunc) func() {
	handle := &taskRunHandle{cancel: cancel}
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if s.runCancel == nil {
		s.runCancel = make(map[string]*taskRunHandle)
	}
	s.runCancel[taskID] = handle
	return func() {
		s.runMu.Lock()
		defer s.runMu.Unlock()
		if current := s.runCancel[taskID]; current == handle {
			delete(s.runCancel, taskID)
		}
	}
}

func (s *TaskService) cancelTaskRun(taskID string) {
	s.runMu.Lock()
	handle := s.runCancel[taskID]
	s.runMu.Unlock()
	if handle != nil {
		handle.cancel()
	}
}

func (s *TaskService) BatchCreateTasks(ctx context.Context, req BatchCreateTasksRequest) (result BatchCreateTasksResult, err error) {
	started := time.Now()
	log.Printf("batch-create started: scan=%s items=%d", req.ScanSessionID, len(req.ItemIDs))
	defer func() {
		log.Printf("batch-create finished: scan=%s created=%d failed=%d elapsed=%s",
			req.ScanSessionID, len(result.Created), len(result.Failed), time.Since(started))
		if err != nil {
			log.Printf("batch-create error: scan=%s err=%v", req.ScanSessionID, err)
		}
	}()

	if strings.TrimSpace(req.ScanSessionID) == "" {
		return result, newTaskServiceError(http.StatusBadRequest, "scan_session_id is required")
	}
	if len(req.ItemIDs) == 0 {
		return result, newTaskServiceError(http.StatusBadRequest, "item_ids is required")
	}

	scan, err := s.store.GetScan(req.ScanSessionID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return result, newTaskServiceError(http.StatusNotFound, "scan session not found")
		}
		return result, err
	}

	cfg, err := s.store.LoadConfig()
	if err != nil {
		return result, err
	}

	result = BatchCreateTasksResult{
		Created: make([]CreatedTaskItem, 0, len(req.ItemIDs)),
		Failed:  make([]FailedTaskItem, 0),
	}
	seen := make(map[string]struct{}, len(req.ItemIDs))
	tasksToCreate := make([]model.Task, 0, len(req.ItemIDs))

	for _, rawItemID := range req.ItemIDs {
		itemID := strings.TrimSpace(rawItemID)
		if itemID == "" {
			result.Failed = append(result.Failed, FailedTaskItem{ItemID: rawItemID, Reason: "item id is empty"})
			continue
		}
		if _, ok := seen[itemID]; ok {
			result.Failed = append(result.Failed, FailedTaskItem{ItemID: itemID, Reason: "duplicate item id in request"})
			continue
		}
		seen[itemID] = struct{}{}

		scanItem, found := findScanItem(scan, itemID)
		if !found {
			result.Failed = append(result.Failed, FailedTaskItem{ItemID: itemID, Reason: "scan item not found"})
			continue
		}
		if reason := validateScanItemForTask(scan, scanItem); reason != "" {
			result.Failed = append(result.Failed, FailedTaskItem{ItemID: itemID, Reason: reason})
			continue
		}
		tasksToCreate = append(tasksToCreate, newTaskFromScan(scan, scanItem, cfg.Emos.Storage, cfg.Local.Root, cfg.Aria2.DownloadDir))
	}

	if len(tasksToCreate) > 0 {
		stored, storeErr := s.store.CreateTasksForScan(ctx, scan.ID, tasksToCreate)
		if storeErr != nil {
			if errors.Is(storeErr, os.ErrNotExist) {
				return result, newTaskServiceError(http.StatusNotFound, "scan session not found")
			}
			return result, storeErr
		}
		for _, task := range stored.Created {
			result.Created = append(result.Created, CreatedTaskItem{TaskID: task.ID, ItemID: task.ScanItemID})
		}
		for _, failure := range stored.Failed {
			result.Failed = append(result.Failed, FailedTaskItem{ItemID: failure.ItemID, Reason: failure.Reason})
		}
	}

	return result, nil
}

func (s *TaskService) ListAllTaskIDs(_ context.Context, statusFilter string) ([]string, error) {
	tasks, err := s.store.ListTasks()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if statusFilter != "" && string(task.Status) != statusFilter {
			continue
		}
		ids = append(ids, task.ID)
	}
	return ids, nil
}

func (s *TaskService) ListTasks(_ context.Context, req ListTasksRequest) (model.TaskListResult, error) {
	tasks, err := s.store.ListTasks()
	if err != nil {
		return model.TaskListResult{}, err
	}

	filtered := make([]model.Task, 0, len(tasks))
	for _, task := range tasks {
		if req.Status != "" && string(task.Status) != req.Status {
			continue
		}
		filtered = append(filtered, task)
	}

	// Sort: active tasks first, then by season/episode, then filename
	sort.Slice(filtered, func(i, j int) bool {
		pi := statusPriority(filtered[i].Status)
		pj := statusPriority(filtered[j].Status)
		if pi != pj {
			return pi < pj
		}
		// Same priority: sort by season then episode
		si := taskSeason(filtered[i])
		sj := taskSeason(filtered[j])
		if si != sj {
			return si < sj
		}
		ei := taskEpisode(filtered[i])
		ej := taskEpisode(filtered[j])
		if ei != ej {
			return ei < ej
		}
		return filtered[i].Source.FileName < filtered[j].Source.FileName
	})

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return model.TaskListResult{
		Items:    filtered[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *TaskService) ListAllTasks(_ context.Context) ([]model.Task, error) {
	tasks, err := s.store.ListTasks()
	if err != nil {
		return nil, err
	}
	// Sort by season/episode so scheduler picks in order
	sort.Slice(tasks, func(i, j int) bool {
		si := taskSeason(tasks[i])
		sj := taskSeason(tasks[j])
		if si != sj {
			return si < sj
		}
		ei := taskEpisode(tasks[i])
		ej := taskEpisode(tasks[j])
		if ei != ej {
			return ei < ej
		}
		return tasks[i].Source.FileName < tasks[j].Source.FileName
	})
	return tasks, nil
}

func (s *TaskService) GetNextQueuedTask(ctx context.Context) (model.Task, bool, error) {
	tasks, err := s.ListAllTasks(ctx)
	if err != nil {
		return model.Task{}, false, err
	}

	for _, task := range tasks {
		if task.Status == model.TaskStatusQueued {
			return task, true, nil
		}
	}

	return model.Task{}, false, nil
}

func (s *TaskService) GetNextRunnableTask(ctx context.Context, excludeIDs map[string]struct{}) (model.Task, bool, error) {
	tasks, err := s.ListAllTasks(ctx)
	if err != nil {
		return model.Task{}, false, err
	}

	// Safety: if any upload is active, don't start new downloads (disk space protection)
	hasUploading := false
	for _, t := range tasks {
		if t.Status == model.TaskStatusUploading || t.Status == model.TaskStatusSaving {
			hasUploading = true
			break
		}
	}

	priorities := []model.TaskStatus{
		model.TaskStatusSaving,
		model.TaskStatusUploadPending,
		model.TaskStatusQueued,
	}
	for _, status := range priorities {
		for _, task := range tasks {
			if task.Status != status {
				continue
			}
			if _, excluded := excludeIDs[task.ID]; excluded {
				continue
			}
			if task.Paused {
				continue
			}
			// Don't start new downloads while uploading (disk space protection)
			if hasUploading && status == model.TaskStatusQueued {
				continue
			}
			return task, true, nil
		}
	}

	return model.Task{}, false, nil
}

func (s *TaskService) BatchPauseTasks(ctx context.Context, ids []string) (paused, failed []string) {
	for _, id := range ids {
		if _, err := s.PauseTask(ctx, strings.TrimSpace(id)); err != nil {
			failed = append(failed, id)
		} else {
			paused = append(paused, id)
		}
	}
	return
}

func (s *TaskService) BatchResumeTasks(ctx context.Context, ids []string) (resumed, failed []string) {
	for _, id := range ids {
		if _, err := s.ResumeTask(ctx, strings.TrimSpace(id)); err != nil {
			failed = append(failed, id)
		} else {
			resumed = append(resumed, id)
		}
	}
	return
}

func (s *TaskService) PauseTask(ctx context.Context, id string) (model.Task, error) {
	now := time.Now()
	task, err := s.store.UpdateTask(id, func(task *model.Task) error {
		task.Paused = true
		task.UpdatedAt = now
		return nil
	})
	if err == nil {
		_ = s.appendTaskLog(id, "info", "task paused")
	}
	return task, err
}

func (s *TaskService) ResumeTask(ctx context.Context, id string) (model.Task, error) {
	now := time.Now()
	task, err := s.store.UpdateTask(id, func(task *model.Task) error {
		task.Paused = false
		task.UpdatedAt = now
		return nil
	})
	if err == nil {
		_ = s.appendTaskLog(id, "info", "task resumed")
	}
	return task, err
}

func (s *TaskService) BatchDeleteTasks(ctx context.Context, ids []string) (deleted []string, failed []string) {
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if err := s.DeleteTask(ctx, id); err != nil {
			failed = append(failed, id)
		} else {
			deleted = append(deleted, id)
		}
	}
	return deleted, failed
}

func (s *TaskService) DeleteTask(ctx context.Context, id string) error {
	task, err := s.GetTask(ctx, id)
	if err != nil {
		return err
	}

	// Don't allow deleting active tasks (safety)
	if task.Status == model.TaskStatusDownloading || task.Status == model.TaskStatusUploading || task.Status == model.TaskStatusSaving {
		return newTaskServiceError(http.StatusBadRequest, "cannot delete an active task, cancel it first")
	}

	// Remove downloaded file (best-effort, with cleanup log)
	s.removeTaskDownloadedFiles(task)

	err = s.store.DeleteTask(task.ID)
	if err != nil {
		return err
	}

	if err := s.appendTaskLog(task.ID, "info", "task deleted"); err != nil {
		// log file already removed, ignore
		_ = err
	}

	return nil
}

func (s *TaskService) removeTaskDownloadedFiles(task model.Task) {
	localPath := strings.TrimSpace(task.Download.LocalPath)
	if localPath == "" {
		return
	}

	// Remove main file
	if err := os.Remove(localPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("[delete] remove downloaded file failed: task=%s path=%s err=%v", task.ID, localPath, err)
	} else if err == nil {
		log.Printf("[delete] removed downloaded file: task=%s path=%s", task.ID, localPath)
	}

	// Remove any multi-thread temp file
	partPath := localPath + ".partmulti"
	if err := os.Remove(partPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("[delete] remove temp file failed: task=%s path=%s err=%v", task.ID, partPath, err)
	} else if err == nil {
		log.Printf("[delete] removed temp file: task=%s path=%s", task.ID, partPath)
	}
}

func (s *TaskService) LoadConfig(_ context.Context) (model.AppConfig, error) {
	return s.store.LoadConfig()
}

func (s *TaskService) GetTask(_ context.Context, id string) (model.Task, error) {
	task, err := s.store.GetTask(strings.TrimSpace(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Task{}, newTaskServiceError(http.StatusNotFound, "task not found")
		}
		return model.Task{}, err
	}
	return task, nil
}

func (s *TaskService) GetTaskLog(ctx context.Context, id string) (model.TaskLog, error) {
	task, err := s.GetTask(ctx, id)
	if err != nil {
		return model.TaskLog{}, err
	}

	taskLog, err := s.store.GetTaskLog(task.ID)
	if err != nil {
		return model.TaskLog{}, err
	}
	return taskLog, nil
}

func (s *TaskService) GetTaskStats(_ context.Context) (model.TaskStats, error) {
	tasks, err := s.store.ListTasks()
	if err != nil {
		return model.TaskStats{}, err
	}

	stats := model.TaskStats{}
	for _, task := range tasks {
		stats.Total++
		switch task.Status {
		case model.TaskStatusQueued:
			stats.Queued++
		case model.TaskStatusCanceled:
			stats.Canceled++
		case model.TaskStatusCompleted:
			stats.Completed++
		case model.TaskStatusDownloadFailed, model.TaskStatusUploadFailed:
			stats.Failed++
		}
	}

	return stats, nil
}

func (s *TaskService) PrepareTaskDownload(ctx context.Context, taskID string) (model.Task, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.Task{}, err
	}

	downloadDir := strings.TrimSpace(cfg.Aria2.DownloadDir)
	if downloadDir == "" {
		return model.Task{}, newTaskServiceError(http.StatusInternalServerError, "aria2 download_dir is required")
	}
	if err := utils.EnsureDir(downloadDir); err != nil {
		return model.Task{}, fmt.Errorf("create download dir: %w", err)
	}

	now := time.Now()
	task, err := s.store.UpdateTask(taskID, func(task *model.Task) error {
		if task.Status != model.TaskStatusQueued {
			return newTaskServiceError(http.StatusBadRequest, fmt.Sprintf("task status %q cannot start download", task.Status))
		}
		if strings.TrimSpace(task.Source.RawURL) == "" {
			return newTaskServiceError(http.StatusBadRequest, "task source.raw_url is required")
		}

		fileName := buildDownloadFileName(*task)
		task.Status = model.TaskStatusDownloading
		task.UpdatedAt = now
		task.FinishedAt = nil
		clearTaskError(task)
		task.Download.Aria2GID = ""
		task.Download.SaveDir = downloadDir
		task.Download.LocalPath = filepath.Join(toContainerPath(downloadDir), fileName)
		task.Download.Status = "starting"
		task.Download.TotalBytes = maxInt64(task.Download.TotalBytes, task.Source.FileSize)
		task.Download.CompletedBytes = 0
		task.Download.Progress = 0
		task.Download.Speed = 0
		return nil
	})
	if err != nil {
		return model.Task{}, err
	}

	if err := s.appendTaskLog(task.ID, "info", "task download started"); err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func (s *TaskService) AttachAria2GID(_ context.Context, taskID, gid string) (model.Task, error) {
	now := time.Now()
	task, err := s.store.UpdateTask(taskID, func(task *model.Task) error {
		if task.Status != model.TaskStatusDownloading {
			return newTaskServiceError(http.StatusBadRequest, fmt.Sprintf("task status %q cannot attach aria2 gid", task.Status))
		}
		task.Download.Aria2GID = strings.TrimSpace(gid)
		task.Download.Status = "active"
		task.UpdatedAt = now
		return nil
	})
	if err != nil {
		return model.Task{}, err
	}
	if err := s.appendTaskLog(task.ID, "info", "aria2 gid assigned"); err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func (s *TaskService) SyncDownloadStatus(_ context.Context, taskID string, ariaStatus client.Aria2Status) (model.Task, error) {
	now := time.Now()
	return s.store.UpdateTask(taskID, func(task *model.Task) error {
		task.UpdatedAt = now
		task.Download.Status = ariaStatus.Status
		task.Download.TotalBytes = maxInt64(task.Download.TotalBytes, ariaStatus.TotalLength)
		task.Download.CompletedBytes = ariaStatus.CompletedLength
		task.Download.Progress = calculateProgress(task.Download.CompletedBytes, task.Download.TotalBytes)
		task.Download.Speed = ariaStatus.DownloadSpeed
		if localPath := firstAria2FilePath(ariaStatus); localPath != "" {
			task.Download.LocalPath = localPath
		}
		if task.Download.Status == "complete" {
			task.Download.Progress = 100
		}
		return nil
	})
}

func (s *TaskService) MarkDownloadFailed(ctx context.Context, taskID, message string) (model.Task, error) {
	return s.MarkDownloadFailedWithDetails(ctx, taskID, "download", "download_failed", message)
}

func (s *TaskService) MarkDownloadFailedWithDetails(_ context.Context, taskID, stage, code, message string) (model.Task, error) {
	now := time.Now()
	trimmedMessage := strings.TrimSpace(message)
	task, err := s.store.UpdateTask(taskID, func(task *model.Task) error {
		task.Status = model.TaskStatusDownloadFailed
		applyTaskError(task, now, stage, code, trimmedMessage)
		task.Download.Status = "error"
		task.Download.Speed = 0
		task.UpdatedAt = now
		task.FinishedAt = &now
		return nil
	})
	if err != nil {
		return model.Task{}, err
	}
	if err := s.appendTaskLog(task.ID, "error", "download failed: "+trimmedMessage); err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func (s *TaskService) MarkDownloadCompleted(ctx context.Context, taskID string, ariaStatus client.Aria2Status) (model.Task, error) {
	now := time.Now()
	task, err := s.store.UpdateTask(taskID, func(task *model.Task) error {
		task.Download.Status = "complete"
		task.Download.TotalBytes = maxInt64(task.Download.TotalBytes, ariaStatus.TotalLength)
		task.Download.CompletedBytes = maxInt64(task.Download.TotalBytes, ariaStatus.CompletedLength)
		task.Download.Progress = 100
		task.Download.Speed = 0
		if localPath := firstAria2FilePath(ariaStatus); localPath != "" {
			task.Download.LocalPath = localPath
		}
		task.Status = model.TaskStatusDownloadCompleted
		task.UpdatedAt = now
		clearTaskError(task)
		return nil
	})
	if err != nil {
		return model.Task{}, err
	}

	if strings.TrimSpace(task.Download.LocalPath) == "" {
		return s.MarkDownloadFailedWithDetails(ctx, taskID, "download", "download_file_missing", "download completed but local file path is empty")
	}
	localPath := toContainerPath(task.Download.LocalPath)
	info, err := os.Stat(localPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s.MarkDownloadFailedWithDetails(ctx, taskID, "download", "download_file_missing", "download completed but local file does not exist")
		}
		return model.Task{}, err
	}
	if info.Size() <= 0 {
		return s.MarkDownloadFailedWithDetails(ctx, taskID, "download", "download_file_missing", "download completed but local file is empty")
	}
	if task.Source.FileSize > 0 && info.Size() < task.Source.FileSize {
		return s.MarkDownloadFailedWithDetails(ctx, taskID, "download", "download_file_missing", "download completed but local file size is smaller than expected")
	}

	if err := s.appendTaskLog(task.ID, "info", "download completed"); err != nil {
		return model.Task{}, err
	}
	return s.MoveTaskToUploadPending(ctx, taskID)
}

func (s *TaskService) MoveTaskToUploadPending(_ context.Context, taskID string) (model.Task, error) {
	now := time.Now()
	task, err := s.store.UpdateTask(taskID, func(task *model.Task) error {
		if task.Status != model.TaskStatusDownloadCompleted && task.Status != model.TaskStatusUploadPending {
			return newTaskServiceError(http.StatusBadRequest, fmt.Sprintf("task status %q cannot move to upload_pending", task.Status))
		}
		task.Status = model.TaskStatusUploadPending
		task.Download.Status = "complete"
		task.Download.Progress = 100
		task.Download.Speed = 0
		task.Upload.Status = "pending"
		task.Upload.Speed = 0
		task.Upload.TotalBytes = maxInt64(task.Upload.TotalBytes, task.Download.TotalBytes)
		task.UpdatedAt = now
		return nil
	})
	if err != nil {
		return model.Task{}, err
	}
	if err := s.appendTaskLog(task.ID, "info", "task moved to upload_pending"); err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func (s *TaskService) PrepareTaskUpload(_ context.Context, taskID string, totalBytes int64) (model.Task, error) {
	now := time.Now()
	task, err := s.store.UpdateTask(taskID, func(task *model.Task) error {
		if task.Status != model.TaskStatusUploadPending {
			return newTaskServiceError(http.StatusBadRequest, fmt.Sprintf("task status %q cannot start upload", task.Status))
		}
		if strings.TrimSpace(task.Download.LocalPath) == "" {
			return newTaskServiceError(http.StatusBadRequest, "task download.local_path is required for upload")
		}

		task.Status = model.TaskStatusUploading
		clearTaskError(task)
		task.UpdatedAt = now
		task.FinishedAt = nil
		task.Upload.Status = "uploading"
		task.Upload.MediaID = ""
		task.Upload.Speed = 0
		task.Upload.SaveRetryCount = 0
		task.Upload.LastSaveError = ""
		if !hasResumableUploadContext(*task) {
			task.Upload.FileID = ""
			task.Upload.UploadURL = ""
			task.Upload.UploadedBytes = 0
			task.Upload.Progress = 0
			task.Upload.UploadType = ""
			task.Upload.MultipartSizeMin = 0
			task.Upload.MultipartSizeMax = 0
			task.Upload.MultipartPresigns = nil
			task.Upload.MultipartParts = nil
		} else {
			task.Upload.Progress = 0
		}
		task.Upload.TotalBytes = maxInt64(maxInt64(task.Source.FileSize, task.Download.TotalBytes), totalBytes)
		return nil
	})
	if err != nil {
		return model.Task{}, err
	}
	if err := s.appendTaskLog(task.ID, "info", "upload started"); err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func (s *TaskService) SetUploadContext(_ context.Context, taskID string, result client.EmosUploadTokenResult) (model.Task, error) {
	now := time.Now()
	task, err := s.store.UpdateTask(taskID, func(task *model.Task) error {
		if task.Status != model.TaskStatusUploading {
			return newTaskServiceError(http.StatusBadRequest, fmt.Sprintf("task status %q cannot attach upload context", task.Status))
		}
		task.Upload.Storage = firstNonEmptyTaskString(result.Storage, task.Upload.Storage, "default")
		task.Upload.FileID = strings.TrimSpace(result.FileID)
		task.Upload.UploadURL = strings.TrimSpace(result.UploadURL)
		task.Upload.UploadType = firstNonEmptyTaskString(result.UploadType, task.Upload.UploadType, "onedrive")
		task.Upload.MultipartSizeMin = result.MultipartSizeMin
		task.Upload.MultipartSizeMax = result.MultipartSizeMax
		task.Upload.MultipartPresigns = nil
		task.Upload.MultipartParts = nil
		task.Upload.UploadedBytes = 0
		task.Upload.Progress = 0
		task.Upload.MediaID = ""
		task.Upload.Status = "uploading"
		task.UpdatedAt = now
		return nil
	})
	if err != nil {
		return model.Task{}, err
	}
	if err := s.appendTaskLog(task.ID, "info", "upload token acquired"); err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func (s *TaskService) SetMultipartPresigns(_ context.Context, taskID string, presigns []model.UploadMultipartPart) (model.Task, error) {
	now := time.Now()
	task, err := s.store.UpdateTask(taskID, func(task *model.Task) error {
		if task.Status != model.TaskStatusUploading {
			return newTaskServiceError(http.StatusBadRequest, fmt.Sprintf("task status %q cannot attach multipart presigns", task.Status))
		}
		if task.Upload.UploadType == "" {
			task.Upload.UploadType = "multipart"
		}
		task.Upload.MultipartPresigns = presigns
		task.Upload.Status = "uploading"
		task.UpdatedAt = now
		return nil
	})
	if err != nil {
		return model.Task{}, err
	}
	if err := s.appendTaskLog(task.ID, "info", fmt.Sprintf("multipart presigns acquired: %d parts", len(presigns))); err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func (s *TaskService) RecordMultipartPart(_ context.Context, taskID string, retryCount int, part model.UploadMultipartPart, progress client.EmosUploadProgress) (model.Task, error) {
	now := time.Now()
	return s.store.UpdateTask(taskID, func(task *model.Task) error {
		if task.Status != model.TaskStatusUploading {
			return newTaskServiceError(http.StatusBadRequest, fmt.Sprintf("task status %q cannot record multipart part", task.Status))
		}
		if task.RetryCount != retryCount {
			return newTaskServiceError(http.StatusConflict, fmt.Sprintf("task retry count %d cannot record multipart part from attempt %d", task.RetryCount, retryCount))
		}
		parts := make([]model.UploadMultipartPart, 0, len(task.Upload.MultipartParts)+1)
		replaced := false
		for _, existing := range task.Upload.MultipartParts {
			if existing.Number == part.Number {
				parts = append(parts, part)
				replaced = true
				continue
			}
			parts = append(parts, existing)
		}
		if !replaced {
			parts = append(parts, part)
		}
		task.Upload.MultipartParts = parts
		task.Upload.Status = "uploading"
		task.Upload.TotalBytes = maxInt64(task.Upload.TotalBytes, progress.TotalBytes)
		task.Upload.UploadedBytes = maxInt64(task.Upload.UploadedBytes, progress.UploadedBytes)
		task.Upload.Progress = calculateProgress(task.Upload.UploadedBytes, task.Upload.TotalBytes)
		task.Upload.Speed = progress.Speed
		task.UpdatedAt = now
		return nil
	})
}

func (s *TaskService) SyncUploadProgress(_ context.Context, taskID string, progress client.EmosUploadProgress) (model.Task, error) {
	now := time.Now()
	return s.store.UpdateTask(taskID, func(task *model.Task) error {
		task.UpdatedAt = now
		task.Status = model.TaskStatusUploading
		task.Upload.Status = "uploading"
		task.Upload.TotalBytes = maxInt64(task.Upload.TotalBytes, progress.TotalBytes)
		task.Upload.UploadedBytes = progress.UploadedBytes
		task.Upload.Progress = calculateProgress(task.Upload.UploadedBytes, task.Upload.TotalBytes)
		task.Upload.Speed = progress.Speed
		if task.Upload.UploadedBytes >= task.Upload.TotalBytes && task.Upload.TotalBytes > 0 {
			task.Upload.Progress = 100
		}
		return nil
	})
}

func (s *TaskService) MarkUploadSaving(_ context.Context, taskID string) (model.Task, error) {
	now := time.Now()
	task, err := s.store.UpdateTask(taskID, func(task *model.Task) error {
		if task.Status != model.TaskStatusUploading && task.Status != model.TaskStatusSaving {
			return newTaskServiceError(http.StatusBadRequest, fmt.Sprintf("task status %q cannot enter saving", task.Status))
		}
		task.Status = model.TaskStatusSaving
		task.Upload.Status = "saving"
		task.Upload.Progress = 100
		task.Upload.UploadedBytes = maxInt64(task.Upload.TotalBytes, task.Upload.UploadedBytes)
		task.Upload.Speed = 0
		task.UpdatedAt = now
		return nil
	})
	if err != nil {
		return model.Task{}, err
	}
	if err := s.appendTaskLog(task.ID, "info", "upload finished, waiting save"); err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func (s *TaskService) RecordSaveRetry(_ context.Context, taskID, message string) (model.Task, error) {
	now := time.Now()
	trimmedMessage := strings.TrimSpace(message)
	task, err := s.store.UpdateTask(taskID, func(task *model.Task) error {
		if task.Status != model.TaskStatusSaving {
			return newTaskServiceError(http.StatusBadRequest, fmt.Sprintf("task status %q cannot record save retry", task.Status))
		}
		task.Upload.Status = "saving"
		task.Upload.SaveRetryCount++
		task.Upload.LastSaveError = trimmedMessage
		task.Upload.Speed = 0
		task.UpdatedAt = now
		return nil
	})
	if err != nil {
		return model.Task{}, err
	}
	if err := s.appendTaskLog(task.ID, "warn", "save retrying: "+trimmedMessage); err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func (s *TaskService) MarkUploadCompleted(_ context.Context, taskID, mediaID string) (model.Task, error) {
	now := time.Now()
	task, err := s.store.UpdateTask(taskID, func(task *model.Task) error {
		task.Status = model.TaskStatusCompleted
		clearTaskError(task)
		task.UpdatedAt = now
		task.FinishedAt = &now
		task.Upload.Status = "completed"
		task.Upload.MediaID = strings.TrimSpace(mediaID)
		task.Upload.Progress = 100
		task.Upload.UploadedBytes = maxInt64(task.Upload.TotalBytes, task.Upload.UploadedBytes)
		task.Upload.Speed = 0
		task.Upload.LastSaveError = ""
		return nil
	})
	if err != nil {
		return model.Task{}, err
	}
	if err := s.appendTaskLog(task.ID, "info", "save completed"); err != nil {
		return model.Task{}, err
	}
	if err := s.appendTaskLog(task.ID, "info", "task completed"); err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func (s *TaskService) MarkUploadFailed(ctx context.Context, taskID, message string) (model.Task, error) {
	return s.MarkUploadFailedWithDetails(ctx, taskID, "upload", "upload_failed", message)
}

func (s *TaskService) MarkUploadFailedWithDetails(_ context.Context, taskID, stage, code, message string) (model.Task, error) {
	now := time.Now()
	trimmedMessage := strings.TrimSpace(message)
	task, err := s.store.UpdateTask(taskID, func(task *model.Task) error {
		task.Status = model.TaskStatusUploadFailed
		applyTaskError(task, now, stage, code, trimmedMessage)
		task.UpdatedAt = now
		task.FinishedAt = &now
		task.Upload.Status = "failed"
		task.Upload.Speed = 0
		if trimmedMessage != "" {
			task.Upload.LastSaveError = trimmedMessage
		}
		return nil
	})
	if err != nil {
		return model.Task{}, err
	}
	if err := s.appendTaskLog(task.ID, "error", "upload failed: "+trimmedMessage); err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func (s *TaskService) RecoverSavingTask(ctx context.Context, taskID string) (model.Task, error) {
	task, err := s.GetTask(ctx, taskID)
	if err != nil {
		return model.Task{}, err
	}

	if strings.TrimSpace(task.Upload.FileID) == "" || strings.TrimSpace(task.Target.ItemType) == "" || task.Target.ItemID <= 0 {
		return s.MarkUploadFailedWithDetails(ctx, taskID, "recovery", "recovery_context_missing", "save context missing during recovery")
	}

	now := time.Now()
	task, err = s.store.UpdateTask(taskID, func(task *model.Task) error {
		task.Status = model.TaskStatusSaving
		task.Upload.Status = "saving"
		task.Upload.Speed = 0
		task.UpdatedAt = now
		task.FinishedAt = nil
		return nil
	})
	if err != nil {
		return model.Task{}, err
	}
	if err := s.appendTaskLog(task.ID, "info", "saving task recovered for save retry"); err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func (s *TaskService) RecoverInterruptedUpload(ctx context.Context, taskID string) (model.Task, error) {
	task, err := s.GetTask(ctx, taskID)
	if err != nil {
		return model.Task{}, err
	}
	if ok, _ := hasReusableLocalFile(task); !ok {
		return s.MarkUploadFailedWithDetails(ctx, taskID, "recovery", "local_file_missing", "local file missing after interrupted upload")
	}
	// Preserve resumable upload context (FileID, UploadURL, multipart presigns/parts) for retry
	// Set status to upload_pending so the scheduler will retry with resume
	now := time.Now()
	task, err = s.store.UpdateTask(taskID, func(task *model.Task) error {
		task.Status = model.TaskStatusUploadPending
		task.Upload.MediaID = ""
		task.Upload.Progress = 0
		task.Upload.Speed = 0
		task.Upload.Status = "pending"
		task.Upload.SaveRetryCount = 0
		task.Upload.LastSaveError = ""
		task.UpdatedAt = now
		task.FinishedAt = nil
		clearTaskError(task)
		return nil
	})
	if err != nil {
		return model.Task{}, err
	}
	if err := s.appendTaskLog(taskID, "info", "task recovered from interrupted upload with resume context"); err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func (s *TaskService) MarkTaskRecovered(_ context.Context, taskID string, ariaStatus client.Aria2Status) (model.Task, error) {
	now := time.Now()
	task, err := s.store.UpdateTask(taskID, func(task *model.Task) error {
		task.Status = model.TaskStatusDownloading
		clearTaskError(task)
		task.Download.Status = ariaStatus.Status
		task.Download.TotalBytes = maxInt64(task.Download.TotalBytes, ariaStatus.TotalLength)
		task.Download.CompletedBytes = ariaStatus.CompletedLength
		task.Download.Progress = calculateProgress(task.Download.CompletedBytes, task.Download.TotalBytes)
		task.Download.Speed = ariaStatus.DownloadSpeed
		if localPath := firstAria2FilePath(ariaStatus); localPath != "" {
			task.Download.LocalPath = localPath
		}
		task.UpdatedAt = now
		task.FinishedAt = nil
		return nil
	})
	if err != nil {
		return model.Task{}, err
	}
	if err := s.appendTaskLog(task.ID, "info", "task recovered from aria2 status"); err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func (s *TaskService) RecoverCompletedDownload(ctx context.Context, taskID string) (model.Task, error) {
	task, err := s.GetTask(ctx, taskID)
	if err != nil {
		return model.Task{}, err
	}
	if ok, _ := hasReusableLocalFile(task); !ok {
		return s.MarkDownloadFailedWithDetails(ctx, taskID, "recovery", "download_file_missing", "download completed but local file is invalid during recovery")
	}

	return s.MoveTaskToUploadPending(ctx, taskID)
}

func (s *TaskService) RetryTask(ctx context.Context, id string) (model.Task, error) {
	task, err := s.GetTask(ctx, id)
	if err != nil {
		return model.Task{}, err
	}
	previousStatus := task.Status

	switch task.Status {
	case model.TaskStatusCanceled, model.TaskStatusDownloadFailed, model.TaskStatusUploadFailed, model.TaskStatusCompleted:
	default:
		return model.Task{}, newTaskServiceError(http.StatusBadRequest, fmt.Sprintf("task status %q cannot be retried", task.Status))
	}

	refreshedRawURL := false
	if previousStatus == model.TaskStatusDownloadFailed {
		if _, refreshed, refreshErr := s.RefreshTaskRawURL(ctx, task.ID); refreshErr == nil {
			refreshedRawURL = refreshed
			if refreshed {
				task, err = s.GetTask(ctx, task.ID)
				if err != nil {
					return model.Task{}, err
				}
			}
		}
	}

	reuseLocalFile, localFileSize := hasReusableLocalFile(task)
	nextStatus := model.TaskStatusQueued
	if (previousStatus == model.TaskStatusUploadFailed || previousStatus == model.TaskStatusCanceled) && reuseLocalFile {
		nextStatus = model.TaskStatusUploadPending
	}

	now := time.Now()
	task, err = s.store.UpdateTask(task.ID, func(task *model.Task) error {
		task.Status = nextStatus
		task.RetryCount++
		clearTaskError(task)
		task.Download.Aria2GID = ""
		task.Download.Speed = 0

		if nextStatus == model.TaskStatusUploadPending {
			task.Download.Status = "complete"
			task.Download.TotalBytes = maxInt64(task.Source.FileSize, localFileSize)
			task.Download.CompletedBytes = task.Download.TotalBytes
			task.Download.Progress = 100
		} else {
			task.Download.SaveDir = ""
			task.Download.LocalPath = ""
			task.Download.Status = ""
			task.Download.CompletedBytes = 0
			task.Download.Progress = 0
			task.Download.TotalBytes = maxInt64(task.Source.FileSize, task.Download.TotalBytes)
		}

		// Preserve upload context for resumable uploads when local file is reusable
		if nextStatus == model.TaskStatusUploadPending && hasResumableUploadContext(*task) {
			task.Upload.MediaID = ""
			task.Upload.Progress = 0
			task.Upload.Speed = 0
			task.Upload.Status = "pending"
			task.Upload.SaveRetryCount = 0
			task.Upload.LastSaveError = ""
		} else {
			task.Upload.FileID = ""
			task.Upload.UploadURL = ""
			task.Upload.UploadType = ""
			task.Upload.MultipartSizeMin = 0
			task.Upload.MultipartSizeMax = 0
			task.Upload.MultipartPresigns = nil
			task.Upload.MultipartParts = nil
			task.Upload.MediaID = ""
			task.Upload.UploadedBytes = 0
			task.Upload.Progress = 0
			task.Upload.Speed = 0
			task.Upload.Status = "pending"
			task.Upload.SaveRetryCount = 0
			task.Upload.LastSaveError = ""
		}
		task.Upload.TotalBytes = maxInt64(maxInt64(task.Source.FileSize, task.Upload.TotalBytes), localFileSize)
		task.UpdatedAt = now
		task.FinishedAt = nil
		return nil
	})
	if err != nil {
		return model.Task{}, err
	}
	logMessage := fmt.Sprintf(
		"task retried from %s; reused_local_file=%t; raw_url_refreshed=%t; next_status=%s",
		previousStatus,
		reuseLocalFile && nextStatus == model.TaskStatusUploadPending,
		refreshedRawURL,
		nextStatus,
	)
	if err := s.appendTaskLog(task.ID, "info", logMessage); err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func (s *TaskService) CancelTask(ctx context.Context, id string) (model.Task, error) {
	task, err := s.GetTask(ctx, id)
	if err != nil {
		return model.Task{}, err
	}

	if task.Status == model.TaskStatusCanceled {
		return task, nil
	}

	switch task.Status {
	case model.TaskStatusQueued:
		return s.markTaskCanceled(task.ID, "queued", "task canceled by user")
	case model.TaskStatusDownloading:
		if err := s.cancelAria2Task(ctx, task); err != nil {
			return model.Task{}, err
		}
		return s.markTaskCanceled(task.ID, "removed", "task canceled by user")
	case model.TaskStatusUploadPending:
		return s.markTaskCanceled(task.ID, task.Download.Status, "task canceled by user")
	case model.TaskStatusUploading, model.TaskStatusSaving:
		s.cancelTaskRun(task.ID)
		return s.markTaskCanceled(task.ID, task.Download.Status, "upload cancel requested; remote partial upload may remain")
	case model.TaskStatusDownloadFailed, model.TaskStatusUploadFailed:
		return s.markTaskCanceled(task.ID, task.Download.Status, "failed task canceled by user")
	default:
		return model.Task{}, newTaskServiceError(http.StatusBadRequest, fmt.Sprintf("task status %q cannot be canceled", task.Status))
	}
}

func (s *TaskService) GetAria2Access(_ context.Context) (client.Aria2Access, model.AppConfig, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return client.Aria2Access{}, model.AppConfig{}, err
	}
	if strings.TrimSpace(cfg.Aria2.RPCURL) == "" {
		return client.Aria2Access{}, model.AppConfig{}, newTaskServiceError(http.StatusInternalServerError, "aria2 rpc_url is required")
	}

	timeout := time.Duration(cfg.Aria2.ConnectTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return client.Aria2Access{
		RPCURL:         cfg.Aria2.RPCURL,
		Secret:         cfg.Aria2.Secret,
		ConnectTimeout: timeout,
	}, cfg, nil
}

func (s *TaskService) GetEmosAccess(_ context.Context) (client.EmosAccess, model.AppConfig, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return client.EmosAccess{}, model.AppConfig{}, err
	}
	if strings.TrimSpace(cfg.Emos.BaseURL) == "" {
		return client.EmosAccess{}, model.AppConfig{}, newTaskServiceError(http.StatusInternalServerError, "emos base_url is required")
	}
	if strings.TrimSpace(cfg.Emos.Token) == "" {
		return client.EmosAccess{}, model.AppConfig{}, newTaskServiceError(http.StatusInternalServerError, "emos token is required")
	}

	return client.EmosAccess{
		BaseURL: cfg.Emos.BaseURL,
		Token:   cfg.Emos.Token,
	}, cfg, nil
}

func (s *TaskService) GetOpenListAccess(_ context.Context) (client.OpenListAccess, model.AppConfig, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return client.OpenListAccess{}, model.AppConfig{}, err
	}
	if strings.TrimSpace(cfg.OpenList.BaseURL) == "" {
		return client.OpenListAccess{}, model.AppConfig{}, newTaskServiceError(http.StatusInternalServerError, "openlist base_url is required")
	}

	return client.OpenListAccess{
		BaseURL:  cfg.OpenList.BaseURL,
		Username: cfg.OpenList.Username,
		Password: cfg.OpenList.Password,
		Token:    cfg.OpenList.Token,
	}, cfg, nil
}

func (s *TaskService) SetTaskError(_ context.Context, taskID string, stage, code, message string) (model.Task, error) {
	now := time.Now()
	return s.store.UpdateTask(taskID, func(task *model.Task) error {
		applyTaskError(task, now, stage, code, message)
		task.UpdatedAt = now
		return nil
	})
}

func (s *TaskService) RefreshTaskRawURL(ctx context.Context, taskID string) (model.Task, bool, error) {
	task, err := s.GetTask(ctx, taskID)
	if err != nil {
		return model.Task{}, false, err
	}
	if s.openListClient == nil || strings.TrimSpace(task.Source.Path) == "" {
		return task, false, nil
	}

	access, cfg, err := s.GetOpenListAccess(ctx)
	if err != nil {
		return model.Task{}, false, err
	}

	// Auto-login if username/password provided but no token
	if access.Token == "" && s.openListClient != nil {
		token, loginErr := s.openListClient.Login(ctx, access)
		if loginErr == nil {
			access.Token = token
		}
	}

	rawURL, err := s.openListClient.GetRawLink(ctx, access, task.Source.Path)
	if err != nil {
		return model.Task{}, false, err
	}
	rawURL = client.ResolveMaybeRelativeURL(cfg.OpenList.BaseURL, rawURL)
	if strings.TrimSpace(rawURL) == "" {
		return model.Task{}, false, errors.New("refreshed raw url is empty")
	}

	now := time.Now()
	task, err = s.store.UpdateTask(taskID, func(task *model.Task) error {
		task.Source.RawURL = rawURL
		task.UpdatedAt = now
		return nil
	})
	if err != nil {
		return model.Task{}, false, err
	}

	if err := s.appendTaskLog(task.ID, "info", "raw url refreshed"); err != nil {
		return model.Task{}, false, err
	}
	return task, true, nil
}

func (s *TaskService) IsTaskCanceled(ctx context.Context, taskID string) (bool, error) {
	task, err := s.GetTask(ctx, taskID)
	if err != nil {
		return false, err
	}
	return task.Status == model.TaskStatusCanceled, nil
}

func (s *TaskService) AddTaskLog(_ context.Context, taskID, level, message string) error {
	return s.appendTaskLog(taskID, level, message)
}

func (s *TaskService) appendTaskLog(taskID, level, message string) error {
	level = strings.TrimSpace(level)
	if level == "" {
		level = "info"
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "-"
	}
	return s.store.AppendTaskLog(taskID, model.TaskLogItem{
		ID:      utils.NewID("log"),
		Level:   level,
		Message: message,
		Time:    time.Now(),
	})
}

func (s *TaskService) cancelAria2Task(ctx context.Context, task model.Task) error {
	if s.aria2Client == nil || strings.TrimSpace(task.Download.Aria2GID) == "" {
		return nil
	}

	access, _, err := s.GetAria2Access(ctx)
	if err != nil {
		return err
	}
	err = s.aria2Client.ForceRemove(ctx, access, task.Download.Aria2GID)
	if err != nil && !isAria2NotFoundError(err) {
		return err
	}
	return nil
}

func (s *TaskService) markTaskCanceled(taskID, downloadStatus, message string) (model.Task, error) {
	now := time.Now()
	task, err := s.store.UpdateTask(taskID, func(task *model.Task) error {
		task.Status = model.TaskStatusCanceled
		task.Download.Status = downloadStatus
		task.Download.Speed = 0
		task.Upload.Speed = 0
		if task.Upload.Status != "" {
			task.Upload.Status = "canceled"
		}
		clearTaskError(task)
		task.UpdatedAt = now
		task.FinishedAt = &now
		return nil
	})
	if err != nil {
		return model.Task{}, err
	}
	if err := s.appendTaskLog(task.ID, "info", message); err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func newTaskFromScan(scan model.ScanSession, item model.ScanItem, storage, localRootCfg, downloadDir string) model.Task {
	now := time.Now()
	title := strings.TrimSpace(item.SelectedTitle)
	if title == "" {
		title = item.FileName
	}
	if strings.TrimSpace(storage) == "" {
		storage = "default"
	}

	isLocal := strings.EqualFold(scan.Source, "local")
	sourceType := "openlist"
	status := model.TaskStatusQueued
	localPath := ""
	downloadStatus := ""
	downloadProgress := float64(0)
	uploadStatus := "pending"

	if isLocal {
		sourceType = "local"
		status = model.TaskStatusUploadPending
		// Resolve against local media root (same rules as LocalService.Root).
		localRoot := resolveLocalMediaRoot(localRootCfg, downloadDir)
		cleanRel := filepath.Clean(strings.TrimPrefix(filepath.Clean(item.OpenListPath), "/"))
		localPath = filepath.Join(localRoot, cleanRel)
		downloadStatus = "complete"
		downloadProgress = 100
		uploadStatus = "pending"
	}

	return model.Task{
		ID:            utils.NewID("task"),
		ScanSessionID: scan.ID,
		ScanItemID:    item.ID,
		Status:        status,
		RetryCount:    0,
		Source: model.TaskSource{
			Type:     sourceType,
			Path:     item.OpenListPath,
			RawURL:   item.RawURL,
			FileName: item.FileName,
			FileSize: item.FileSize,
		},
		Parsed: model.TaskParsed{
			Season:    item.Parsed.Season,
			Episode:   item.Parsed.Episode,
			IsSpecial: item.Parsed.IsSpecial,
		},
		Target: model.TaskTarget{
			TMDBID:   scan.TMDBID,
			ItemType: item.SelectedItemType,
			ItemID:   item.SelectedItemID,
			Title:    title,
		},
		Download: model.TaskDownload{
			TotalBytes:     item.FileSize,
			CompletedBytes: item.FileSize,
			LocalPath:      localPath,
			Status:         downloadStatus,
			Progress:       downloadProgress,
		},
		Upload: model.TaskUpload{
			Storage:    storage,
			TotalBytes: item.FileSize,
			Status:     uploadStatus,
		},
		Result:    model.TaskResult{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func validateScanItemForTask(scan model.ScanSession, item model.ScanItem) string {
	if !item.Confirmed {
		return "scan item not confirmed"
	}
	if strings.TrimSpace(item.SelectedItemType) == "" {
		return "selected item_type is required"
	}
	if item.SelectedItemID <= 0 {
		return "selected item_id is required"
	}
	if !item.IsVideo {
		return "scan item is not a video file"
	}
	// Only OpenList scans require raw_url; local files are already on disk
	if !strings.EqualFold(scan.Source, "local") && strings.TrimSpace(item.RawURL) == "" {
		return "raw_url is required"
	}
	return ""
}

func findScanItem(scan model.ScanSession, itemID string) (model.ScanItem, bool) {
	for _, item := range scan.Items {
		if item.ID == itemID {
			return item, true
		}
	}
	return model.ScanItem{}, false
}

func buildDownloadFileName(task model.Task) string {
	return task.ID + "__" + task.Source.FileName
}

func calculateProgress(completed, total int64) float64 {
	if total <= 0 {
		return 0
	}
	if completed >= total {
		return 100
	}
	return float64(completed) * 100 / float64(total)
}

func firstAria2FilePath(status client.Aria2Status) string {
	if len(status.Files) == 0 {
		return ""
	}
	return strings.TrimSpace(status.Files[0].Path)
}

func isAria2NotFoundError(err error) bool {
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "not found") || strings.Contains(message, "cannot be found")
}

func firstNonEmptyTaskString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func clearTaskError(task *model.Task) {
	task.Result.ErrorMessage = ""
	task.Result.ErrorStage = ""
	task.Result.ErrorCode = ""
	task.Result.LastErrorAt = nil
}

func applyTaskError(task *model.Task, at time.Time, stage, code, message string) {
	task.Result.ErrorMessage = strings.TrimSpace(message)
	task.Result.ErrorStage = strings.TrimSpace(stage)
	task.Result.ErrorCode = strings.TrimSpace(code)
	task.Result.LastErrorAt = &at
}

func hasReusableLocalFile(task model.Task) (bool, int64) {
	localPath := toContainerPath(strings.TrimSpace(task.Download.LocalPath))
	if localPath == "" {
		return false, 0
	}

	info, err := os.Stat(localPath)
	if err != nil || info.IsDir() {
		return false, 0
	}
	if info.Size() <= 0 {
		return false, 0
	}

	expectedSize := maxInt64(task.Source.FileSize, task.Download.TotalBytes)
	if expectedSize > 0 && info.Size() < expectedSize {
		return false, info.Size()
	}

	return true, info.Size()
}

func hasResumableUploadContext(task model.Task) bool {
	if strings.TrimSpace(task.Upload.FileID) == "" {
		return false
	}
	return strings.TrimSpace(task.Upload.UploadURL) != "" || len(task.Upload.MultipartPresigns) > 0
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// resolveLocalMediaRoot uses the same rules as LocalService.Root.
func resolveLocalMediaRoot(localRoot, downloadDir string) string {
	dataRoot := ""
	if strings.HasSuffix(filepath.Clean(downloadDir), string(filepath.Separator)+"downloads") {
		dataRoot = filepath.Dir(downloadDir)
	}
	return ResolveLocalMediaRoot(localRoot, downloadDir, dataRoot)
}

func newTaskServiceError(code int, message string) error {
	return &TaskServiceError{Code: code, Message: message}
}

func taskSeason(task model.Task) int {
	if task.Parsed.Season != nil {
		return *task.Parsed.Season
	}
	return 999
}

func taskEpisode(task model.Task) int {
	if task.Parsed.Episode != nil {
		return *task.Parsed.Episode
	}
	return 999
}

func statusPriority(status model.TaskStatus) int {
	switch status {
	case model.TaskStatusUploading, model.TaskStatusSaving:
		return 1
	case model.TaskStatusDownloading:
		return 2
	case model.TaskStatusQueued, model.TaskStatusUploadPending, model.TaskStatusDownloadCompleted:
		return 3
	case model.TaskStatusDownloadFailed, model.TaskStatusUploadFailed:
		return 4
	case model.TaskStatusCompleted:
		return 5
	case model.TaskStatusCanceled:
		return 6
	default:
		return 10
	}
}
