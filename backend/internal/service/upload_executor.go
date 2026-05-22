package service

import (
	"context"
	"errors"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"emosup/backend/internal/client"
	"emosup/backend/internal/model"
	"emosup/backend/internal/eventbus"
)

var errTaskCanceled = errors.New("task canceled")

type UploadExecutor struct {
	taskService *TaskService
	emosClient  client.EmosClient
	eventBus    *eventbus.Bus
}

func NewUploadExecutor(taskService *TaskService, emosClient client.EmosClient, eventBus *eventbus.Bus) *UploadExecutor {
	return &UploadExecutor{
		taskService: taskService,
		emosClient:  emosClient,
		eventBus:    eventBus,
	}
}

func (e *UploadExecutor) Execute(ctx context.Context, taskID string) error {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Hour)
	defer cancel()

	task, err := e.taskService.GetTask(ctx, taskID)
	if err != nil {
		return err
	}

	access, cfg, err := e.taskService.GetEmosAccess(ctx)
	if err != nil {
		if task.Status == model.TaskStatusUploadPending || task.Status == model.TaskStatusSaving {
			_, _ = e.taskService.MarkUploadFailedWithDetails(ctx, task.ID, "system", "upload_token_failed", err.Error())
		}
		return err
	}

	chunkSize := int64(cfg.Worker.UploadChunkSizeMB) * 1024 * 1024
	if chunkSize <= 0 {
		chunkSize = 8 * 1024 * 1024
	}

	saveRetryInterval := time.Duration(cfg.Worker.SaveRetryIntervalSeconds) * time.Second
	if saveRetryInterval <= 0 {
		saveRetryInterval = 25 * time.Second
	}

	saveRetryMaxAttempts := cfg.Worker.SaveRetryMaxAttempts
	if saveRetryMaxAttempts <= 0 {
		saveRetryMaxAttempts = 8
	}

	switch task.Status {
	case model.TaskStatusUploadPending:
		task, err = e.uploadFile(ctx, task, access, chunkSize)
		if err != nil {
			if errors.Is(err, errTaskCanceled) {
				return nil
			}
			return err
		}
	case model.TaskStatusSaving:
	default:
		return nil
	}

	return e.saveWithRetry(ctx, task, access, saveRetryInterval, saveRetryMaxAttempts)
}

func (e *UploadExecutor) uploadFile(ctx context.Context, task model.Task, access client.EmosAccess, chunkSize int64) (model.Task, error) {
	localPath := strings.TrimSpace(task.Download.LocalPath)
	if localPath == "" {
		_, err := e.taskService.MarkUploadFailedWithDetails(ctx, task.ID, "upload", "local_file_missing", "local file path is empty")
		return model.Task{}, err
	}

	info, err := os.Stat(localPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_, markErr := e.taskService.MarkUploadFailedWithDetails(ctx, task.ID, "upload", "local_file_missing", "local file does not exist")
			return model.Task{}, firstNonNil(markErr, err)
		}
		_, markErr := e.taskService.MarkUploadFailedWithDetails(ctx, task.ID, "upload", "local_file_missing", "failed to stat local file: "+err.Error())
		return model.Task{}, firstNonNil(markErr, err)
	}

	task, err = e.taskService.PrepareTaskUpload(ctx, task.ID, info.Size())
	if err != nil {
		return model.Task{}, err
	}

	tokenResult, err := e.emosClient.GetUploadToken(ctx, access, client.EmosUploadTokenRequest{
		ResourceType: "video",
		FileType:     detectUploadMimeType(localPath),
		FileName:     filepath.Base(localPath),
		FileSize:     info.Size(),
		FileStorage:  task.Upload.Storage,
	})
	if err != nil {
		_, markErr := e.taskService.MarkUploadFailedWithDetails(ctx, task.ID, "upload", "upload_token_failed", "upload token failed: "+err.Error())
		return model.Task{}, firstNonNil(markErr, err)
	}

	task, err = e.taskService.SetUploadContext(ctx, task.ID, tokenResult)
	if err != nil {
		return model.Task{}, err
	}

	err = e.emosClient.UploadFile(ctx, task.Upload.UploadURL, localPath, chunkSize, func(progress client.EmosUploadProgress) error {
		if canceled, cancelErr := e.taskService.IsTaskCanceled(ctx, task.ID); cancelErr != nil {
			return cancelErr
		} else if canceled {
			return errTaskCanceled
		}

		_, syncErr := e.taskService.SyncUploadProgress(ctx, task.ID, progress)
		if e.eventBus != nil {
			e.eventBus.Publish(eventbus.TaskEvent{TaskID: task.ID, Status: "uploading"})
		}
		return syncErr
	})
	if err != nil {
		if errors.Is(err, errTaskCanceled) {
			return task, errTaskCanceled
		}
		_, markErr := e.taskService.MarkUploadFailedWithDetails(ctx, task.ID, "upload", "upload_put_failed", "upload put failed: "+err.Error())
		return model.Task{}, firstNonNil(markErr, err)
	}

	task, err = e.taskService.MarkUploadSaving(ctx, task.ID)
	if err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func (e *UploadExecutor) saveWithRetry(ctx context.Context, task model.Task, access client.EmosAccess, interval time.Duration, maxAttempts int) error {
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if canceled, err := e.taskService.IsTaskCanceled(ctx, task.ID); err != nil {
			return err
		} else if canceled {
			return nil
		}

		result, err := e.emosClient.SaveVideo(ctx, access, client.EmosSaveVideoRequest{
			ItemType: task.Target.ItemType,
			ItemID:   task.Target.ItemID,
			FileID:   task.Upload.FileID,
		})
		if err == nil {
			_, completeErr := e.taskService.MarkUploadCompleted(ctx, task.ID, result.MediaID)
			return completeErr
		}

		message := strings.TrimSpace(err.Error())
		if message == "" {
			message = "unknown save error"
		}

		switch classifySaveError(err) {
		case SaveErrorKindRetryableWaiting:
			if _, retryErr := e.taskService.RecordSaveRetry(ctx, task.ID, message); retryErr != nil {
				return retryErr
			}
			if attempt == maxAttempts {
				_, markErr := e.taskService.MarkUploadFailedWithDetails(ctx, task.ID, "save", "save_wait_timeout", "save timeout after retries: "+message)
				return firstNonNil(markErr, err)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(interval):
			}
		case SaveErrorKindRetryableTemporary:
			if _, retryErr := e.taskService.RecordSaveRetry(ctx, task.ID, message); retryErr != nil {
				return retryErr
			}
			if attempt == maxAttempts {
				_, markErr := e.taskService.MarkUploadFailedWithDetails(ctx, task.ID, "save", "save_wait_timeout", "save temporary failure after retries: "+message)
				return firstNonNil(markErr, err)
			}
			wait := interval * time.Duration(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		default:
			_, markErr := e.taskService.MarkUploadFailedWithDetails(ctx, task.ID, "save", "save_fatal_error", "save failed: "+message)
			return firstNonNil(markErr, err)
		}
	}

	return nil
}

func detectUploadMimeType(filePath string) string {
	fileType := mime.TypeByExtension(strings.ToLower(filepath.Ext(filePath)))
	if fileType == "" {
		return "application/octet-stream"
	}
	return fileType
}

func firstNonNil(primary error, fallback error) error {
	if primary != nil {
		return primary
	}
	return fallback
}
