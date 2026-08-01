package service

import (
	"context"
	"errors"
	"log"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"emosup/backend/internal/client"
	"emosup/backend/internal/eventbus"
	"emosup/backend/internal/model"
)

var errTaskCanceled = errors.New("task canceled")

const maxMultipartUploadWorkers = 10

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

func (e *UploadExecutor) isCanceledUpload(ctx context.Context, taskID string) bool {
	if ctx.Err() != nil {
		return true
	}
	canceled, err := e.taskService.IsTaskCanceled(ctx, taskID)
	return err == nil && canceled
}

func (e *UploadExecutor) isStaleUploadAttempt(ctx context.Context, taskID string, retryCount int) bool {
	task, err := e.taskService.GetTask(ctx, taskID)
	if err != nil {
		return ctx.Err() != nil
	}
	return task.RetryCount != retryCount || task.Status != model.TaskStatusUploading
}

func (e *UploadExecutor) Execute(ctx context.Context, taskID string) error {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Hour)
	defer cancel()
	unregister := e.taskService.RegisterTaskRun(taskID, cancel)
	defer unregister()

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
		// For retried uploads with existing upload context, resume from where we left off
		task, err = e.uploadFile(ctx, task, access, chunkSize, clampUploadPartConcurrency(cfg.Worker.UploadPartConcurrency))
		if err != nil {
			if errors.Is(err, errTaskCanceled) || e.isCanceledUpload(ctx, task.ID) {
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

func (e *UploadExecutor) uploadFile(ctx context.Context, task model.Task, access client.EmosAccess, chunkSize int64, maxWorkers int) (model.Task, error) {
	localPath := toContainerPath(strings.TrimSpace(task.Download.LocalPath))
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

	resuming := hasResumableUploadContext(task)
	if !resuming {
		task, err = e.taskService.PrepareTaskUpload(ctx, task.ID, info.Size())
		if err != nil {
			return model.Task{}, err
		}

		tokenResult, tokenErr := e.emosClient.GetUploadToken(ctx, access, client.EmosUploadTokenRequest{
			ResourceType: "video",
			FileType:     detectUploadMimeType(localPath),
			FileName:     filepath.Base(localPath),
			FileSize:     info.Size(),
			FileStorage:  task.Upload.Storage,
		})
		if tokenErr != nil {
			if e.isCanceledUpload(ctx, task.ID) {
				return model.Task{}, errTaskCanceled
			}
			_, markErr := e.taskService.MarkUploadFailedWithDetails(ctx, task.ID, "upload", "upload_token_failed", "upload token failed: "+tokenErr.Error())
			return model.Task{}, firstNonNil(markErr, tokenErr)
		}

		task, err = e.taskService.SetUploadContext(ctx, task.ID, tokenResult)
		if err != nil {
			return model.Task{}, err
		}
	} else {
		// Resume: just update total bytes and set status
		task, err = e.taskService.PrepareTaskUpload(ctx, task.ID, info.Size())
		if err != nil {
			return model.Task{}, err
		}
	}

	uploadType := strings.ToLower(strings.TrimSpace(task.Upload.UploadType))
	switch uploadType {
	case "", "onedrive", "r2":
		if uploadType == "" {
			uploadType = "onedrive"
		}
		// Resume from existing progress if we have upload context and bytes already sent
		effectiveUploadURL := task.Upload.UploadURL
		offset := task.Upload.UploadedBytes
		if offset > 0 && offset < info.Size() {
			log.Printf("upload resuming: task=%s from byte %d/%d", task.ID, offset, info.Size())
		}

		lastSSE := time.Time{}
		err = e.emosClient.UploadFile(ctx, uploadType, effectiveUploadURL, localPath, chunkSize, offset, func(progress client.EmosUploadProgress) error {
			if canceled, cancelErr := e.taskService.IsTaskCanceled(ctx, task.ID); cancelErr != nil {
				return cancelErr
			} else if canceled {
				return errTaskCanceled
			}

			updated, syncErr := e.taskService.SyncUploadProgress(ctx, task.ID, progress)
			if syncErr != nil {
				return syncErr
			}

			e.emitUploadEvent(updated, &lastSSE, progress.TotalBytes > 0 && progress.UploadedBytes >= progress.TotalBytes)
			return nil
		})
		if err != nil {
			if errors.Is(err, errTaskCanceled) || e.isCanceledUpload(ctx, task.ID) {
				return task, errTaskCanceled
			}
			_, markErr := e.taskService.MarkUploadFailedWithDetails(ctx, task.ID, "upload", "upload_put_failed", "upload put failed: "+err.Error())
			return model.Task{}, firstNonNil(markErr, err)
		}
	case "multipart":
		if err := e.uploadMultipart(ctx, task, access, chunkSize, info.Size(), localPath, maxWorkers, task.RetryCount); err != nil {
			if errors.Is(err, errTaskCanceled) || e.isCanceledUpload(ctx, task.ID) {
				return task, errTaskCanceled
			}
			_, markErr := e.taskService.MarkUploadFailedWithDetails(ctx, task.ID, "upload", "multipart_upload_failed", "multipart upload failed: "+err.Error())
			return model.Task{}, firstNonNil(markErr, err)
		}
	default:
		_, markErr := e.taskService.MarkUploadFailedWithDetails(ctx, task.ID, "upload", "unsupported_upload_type", "unsupported emos upload type: "+uploadType)
		return model.Task{}, firstNonNil(markErr, errors.New("unsupported emos upload type: "+uploadType))
	}

	task, err = e.taskService.MarkUploadSaving(ctx, task.ID)
	if err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func (e *UploadExecutor) uploadMultipart(ctx context.Context, task model.Task, access client.EmosAccess, configuredChunkSize, fileSize int64, localPath string, maxWorkers int, retryCount int) error {
	chunkSize := configuredChunkSize
	if task.Upload.MultipartSizeMin > 0 || task.Upload.MultipartSizeMax > 0 {
		if task.Upload.MultipartSizeMin > 0 && chunkSize < task.Upload.MultipartSizeMin {
			chunkSize = task.Upload.MultipartSizeMin
		}
		if task.Upload.MultipartSizeMax > 0 && chunkSize > task.Upload.MultipartSizeMax {
			chunkSize = task.Upload.MultipartSizeMax
		}
	}
	if chunkSize <= 0 {
		return errors.New("multipart chunk size must be positive")
	}

	numChunks := int((fileSize + chunkSize - 1) / chunkSize)
	presigns := task.Upload.MultipartPresigns
	if len(presigns) == 0 {
		clientPresigns, err := e.emosClient.RequestMultipartPresigns(ctx, access, task.Upload.FileID, numChunks)
		if err != nil {
			return err
		}
		presigns = make([]model.UploadMultipartPart, 0, len(clientPresigns))
		for _, part := range clientPresigns {
			presigns = append(presigns, model.UploadMultipartPart{
				Number:    part.Number,
				UploadURL: part.UploadURL,
				ETag:      part.ETag,
			})
		}
		updated, err := e.taskService.SetMultipartPresigns(ctx, task.ID, presigns)
		if err != nil {
			return err
		}
		task = updated
		presigns = task.Upload.MultipartPresigns
	}

	if len(presigns) != numChunks {
		return errors.New("multipart presign count does not match file chunk count")
	}

	sort.Slice(presigns, func(i, j int) bool {
		return presigns[i].Number < presigns[j].Number
	})

	uploaded := make(map[int]model.UploadMultipartPart, len(task.Upload.MultipartParts))
	for _, part := range task.Upload.MultipartParts {
		if part.Number > 0 {
			uploaded[part.Number] = part
		}
	}

	jobs := make([]int, 0, len(presigns))
	for idx, presign := range presigns {
		startByte := int64(presign.Number-1) * chunkSize
		if startByte >= fileSize {
			return errors.New("multipart presign is outside local file range")
		}

		if part, ok := uploaded[presign.Number]; ok && strings.TrimSpace(part.ETag) != "" {
			continue
		}
		jobs = append(jobs, idx)
	}

	completedBytes := int64(0)
	for _, presign := range presigns {
		if part, ok := uploaded[presign.Number]; ok && strings.TrimSpace(part.ETag) != "" {
			completedBytes += partSizeForMultipartPresign(presign.Number, chunkSize, fileSize)
		}
	}

	if len(jobs) > 0 {
		workerCount := clampUploadPartConcurrency(maxWorkers)
		if workerCount > len(jobs) {
			workerCount = len(jobs)
		}

		pending := make(chan int, len(jobs))
		for _, idx := range jobs {
			pending <- idx
		}
		close(pending)

		workerCtx, stopWorkers := context.WithCancel(ctx)
		defer stopWorkers()

		lastSSE := time.Time{}
		var wg sync.WaitGroup
		var stateMu sync.Mutex
		var eventMu sync.Mutex
		var firstErr error

		recordWorkerError := func(workerErr error) {
			stateMu.Lock()
			defer stateMu.Unlock()
			if firstErr != nil {
				return
			}
			if ctx.Err() != nil {
				firstErr = errTaskCanceled
			} else {
				firstErr = workerErr
			}
			stopWorkers()
		}

		worker := func() {
			defer wg.Done()
			for idx := range pending {
				if workerCtx.Err() != nil {
					return
				}

				presign := presigns[idx]
				startByte := int64(presign.Number-1) * chunkSize
				partSize := partSizeForMultipartPresign(presign.Number, chunkSize, fileSize)

				if canceled, cancelErr := e.taskService.IsTaskCanceled(workerCtx, task.ID); cancelErr != nil {
					recordWorkerError(cancelErr)
					return
				} else if canceled {
					recordWorkerError(errTaskCanceled)
					return
				}

				startedAt := time.Now()
				etag, uploadErr := e.emosClient.UploadMultipartPart(workerCtx, client.EmosMultipartPart{
					Number:    presign.Number,
					UploadURL: presign.UploadURL,
				}, localPath, startByte, partSize)
				if uploadErr != nil {
					if workerCtx.Err() != nil || e.isStaleUploadAttempt(workerCtx, task.ID, retryCount) {
						recordWorkerError(errTaskCanceled)
						return
					}
					recordWorkerError(uploadErr)
					return
				}

				part := model.UploadMultipartPart{
					Number:    presign.Number,
					UploadURL: presign.UploadURL,
					ETag:      etag,
				}
				stateMu.Lock()
				uploaded[presign.Number] = part
				completedBytes += partSize
				currentBytes := completedBytes
				stateMu.Unlock()

				speed := int64(0)
				if elapsed := time.Since(startedAt); elapsed > 0 {
					speed = int64(float64(partSize) / elapsed.Seconds())
				}

				if workerCtx.Err() != nil {
					return
				}
				updated, syncErr := e.taskService.RecordMultipartPart(ctx, task.ID, retryCount, part, client.EmosUploadProgress{
					UploadedBytes: currentBytes,
					TotalBytes:    fileSize,
					Speed:         speed,
				})
				if syncErr != nil {
					if workerCtx.Err() != nil || e.isStaleUploadAttempt(workerCtx, task.ID, retryCount) {
						recordWorkerError(errTaskCanceled)
						return
					}
					recordWorkerError(syncErr)
					return
				}

				eventMu.Lock()
				e.emitUploadEvent(updated, &lastSSE, currentBytes >= fileSize)
				eventMu.Unlock()
			}
		}

		for i := 0; i < workerCount; i++ {
			wg.Add(1)
			go worker()
		}
		wg.Wait()
		if firstErr != nil {
			return firstErr
		}
	}
	if ctx.Err() != nil {
		return errTaskCanceled
	}

	parts := make([]client.EmosMultipartPart, 0, len(presigns))
	for _, presign := range presigns {
		part, ok := uploaded[presign.Number]
		if !ok || strings.TrimSpace(part.ETag) == "" {
			return errors.New("multipart upload is incomplete")
		}
		parts = append(parts, client.EmosMultipartPart{
			Number: presign.Number,
			ETag:   part.ETag,
		})
	}
	return e.emosClient.CompleteMultipart(ctx, access, task.Upload.FileID, parts)
}

func clampUploadPartConcurrency(value int) int {
	if value <= 0 {
		return 3
	}
	if value > maxMultipartUploadWorkers {
		return maxMultipartUploadWorkers
	}
	return value
}

func partSizeForMultipartPresign(number int, chunkSize, fileSize int64) int64 {
	startByte := int64(number-1) * chunkSize
	if startByte >= fileSize {
		return 0
	}
	if remaining := fileSize - startByte; remaining < chunkSize {
		return remaining
	}
	return chunkSize
}

func (e *UploadExecutor) emitUploadEvent(task model.Task, lastSSE *time.Time, done bool) {
	if e.eventBus == nil {
		return
	}
	now := time.Now()
	if done || now.Sub(*lastSSE) >= time.Second {
		*lastSSE = now
		e.eventBus.Publish(eventbus.TaskEvent{
			TaskID:  task.ID,
			Status:  "uploading",
			UlProg:  task.Upload.Progress,
			UlSpeed: task.Upload.Speed,
			UlDone:  task.Upload.UploadedBytes,
			UlTotal: task.Upload.TotalBytes,
		})
	}
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
			// Delete local file after successful upload to save disk space
			if localPath := strings.TrimSpace(task.Download.LocalPath); localPath != "" {
				if rmErr := os.Remove(localPath); rmErr != nil {
					log.Printf("failed to remove local file after upload: %s err=%v", localPath, rmErr)
				} else {
					log.Printf("local file removed after upload: %s", localPath)
				}
			}
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
