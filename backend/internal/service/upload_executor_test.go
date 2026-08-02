package service

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"emosup/backend/internal/client"
	"emosup/backend/internal/model"
	"emosup/backend/internal/store"
)

func TestUploadExecutorExecuteUploadPendingTaskWithSaveRetry(t *testing.T) {
	t.Parallel()

	fileStore := newTaskTestStore(t)
	cfg, err := fileStore.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Worker.UploadChunkSizeMB = 1
	cfg.Worker.SaveRetryIntervalSeconds = 1
	cfg.Worker.SaveRetryMaxAttempts = 3
	cfg.Emos.Token = "demo-token"
	if err := fileStore.SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	taskService := NewTaskService(fileStore)
	emosClient := &stubEmosUploadClient{
		tokenResult: client.EmosUploadTokenResult{
			Storage:   "onedrive",
			FileID:    "file-123",
			UploadURL: "https://upload.example/session",
		},
		uploadProgress: []client.EmosUploadProgress{
			{UploadedBytes: 512, TotalBytes: 1024, Speed: 128},
			{UploadedBytes: 1024, TotalBytes: 1024, Speed: 256},
		},
		saveAttempts: []stubSaveAttempt{
			{
				err: &client.EmosSaveError{
					StatusCode: 422,
					Message:    "视频正在合并中 请稍后再试",
				},
			},
			{
				result: client.EmosSaveVideoResult{
					Count:   1,
					MediaID: "media-123",
				},
			},
		},
	}
	executor := NewUploadExecutor(taskService, emosClient, nil, nil)
	taskID := seedUploadPendingTask(t, taskService, fileStore)

	if err := executor.Execute(context.Background(), taskID); err != nil {
		t.Fatalf("execute upload task: %v", err)
	}

	task, err := taskService.GetTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}

	if task.Status != model.TaskStatusCompleted {
		t.Fatalf("expected completed status, got %s", task.Status)
	}
	if task.Upload.Status != "completed" {
		t.Fatalf("expected upload status completed, got %s", task.Upload.Status)
	}
	if task.Upload.FileID != "file-123" {
		t.Fatalf("expected file id to be saved, got %s", task.Upload.FileID)
	}
	if task.Upload.MediaID != "media-123" {
		t.Fatalf("expected media id to be saved, got %s", task.Upload.MediaID)
	}
	if task.Upload.Progress != 100 {
		t.Fatalf("expected upload progress 100, got %v", task.Upload.Progress)
	}
	if task.Upload.SaveRetryCount != 1 {
		t.Fatalf("expected save retry count 1, got %d", task.Upload.SaveRetryCount)
	}
	if task.Result.ErrorMessage != "" {
		t.Fatalf("expected error message to be cleared, got %s", task.Result.ErrorMessage)
	}
	if task.FinishedAt == nil {
		t.Fatalf("expected finished_at to be set")
	}

	taskLog, err := taskService.GetTaskLog(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get task log: %v", err)
	}
	if len(taskLog.Items) < 8 {
		t.Fatalf("expected upload task log entries, got %d", len(taskLog.Items))
	}
}

func TestRetryUploadFailedTaskPreservesUploadSessionForResume(t *testing.T) {
	t.Parallel()

	fileStore := newTaskTestStore(t)
	taskService := NewTaskService(fileStore)
	taskID := seedUploadPendingTask(t, taskService, fileStore)

	if _, err := fileStore.UpdateTask(taskID, func(task *model.Task) error {
		task.Status = model.TaskStatusUploadFailed
		task.Upload.Status = "failed"
		task.Upload.FileID = "file-123"
		task.Upload.UploadURL = "https://upload.example/session"
		task.Upload.MediaID = "media-123"
		task.Upload.UploadedBytes = 512
		task.Upload.Progress = 50
		task.Upload.Speed = 128
		task.Upload.SaveRetryCount = 2
		task.Upload.LastSaveError = "save timeout"
		task.Result.ErrorMessage = "save timeout"
		return nil
	}); err != nil {
		t.Fatalf("prepare upload failed task: %v", err)
	}

	task, err := taskService.RetryTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("retry upload failed task: %v", err)
	}

	if task.Status != model.TaskStatusUploadPending {
		t.Fatalf("expected upload_pending status after retry, got %s", task.Status)
	}
	// Keep FileID/UploadURL so upload can resume; clear save-side error state.
	if task.Upload.FileID != "file-123" || task.Upload.UploadURL != "https://upload.example/session" {
		t.Fatalf("expected upload session to be preserved for resume, got %#v", task.Upload)
	}
	if task.Upload.MediaID != "" {
		t.Fatalf("expected media_id cleared before re-save, got %q", task.Upload.MediaID)
	}
	if task.Upload.SaveRetryCount != 0 || task.Upload.LastSaveError != "" {
		t.Fatalf("expected save retry state to be reset, got %#v", task.Upload)
	}
	if task.Upload.Status != "pending" {
		t.Fatalf("expected upload status pending after retry, got %s", task.Upload.Status)
	}
	if task.Download.LocalPath == "" || task.Download.Progress != 100 {
		t.Fatalf("expected local download to be reused, got %#v", task.Download)
	}
	if task.Result.ErrorMessage != "" {
		t.Fatalf("expected error message to be cleared, got %s", task.Result.ErrorMessage)
	}
}

func TestUploadExecutorR2UploadType(t *testing.T) {
	t.Parallel()

	fileStore := newTaskTestStore(t)
	cfg, err := fileStore.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Worker.UploadChunkSizeMB = 1
	cfg.Emos.Token = "demo-token"
	cfg.Emos.Storage = "zn_r2_upload"
	if err := fileStore.SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	taskService := NewTaskService(fileStore)
	emosClient := &stubEmosUploadClient{
		tokenResult: client.EmosUploadTokenResult{
			Storage:    "r2",
			FileID:     "file-r2",
			UploadURL:  "https://upload.example/r2",
			UploadType: "r2",
		},
		uploadProgress: []client.EmosUploadProgress{
			{UploadedBytes: 1024, TotalBytes: 1024, Speed: 256},
		},
	}
	executor := NewUploadExecutor(taskService, emosClient, nil, nil)
	taskID := seedUploadPendingTask(t, taskService, fileStore)

	if err := executor.Execute(context.Background(), taskID); err != nil {
		t.Fatalf("execute r2 upload task: %v", err)
	}

	if len(emosClient.uploadFileCalls) != 1 || emosClient.uploadFileCalls[0] != "r2" {
		t.Fatalf("expected r2 upload dispatch, got %v", emosClient.uploadFileCalls)
	}
	if len(emosClient.uploadTokenRequests) != 1 ||
		emosClient.uploadTokenRequests[0].FileStorage != "zn_r2_upload" {
		t.Fatalf("expected upload token storage zn_r2_upload, got %#v", emosClient.uploadTokenRequests)
	}

	task, err := taskService.GetTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Status != model.TaskStatusCompleted {
		t.Fatalf("expected completed status, got %s", task.Status)
	}
	if task.Upload.UploadType != "r2" {
		t.Fatalf("expected upload type r2, got %s", task.Upload.UploadType)
	}
}

func TestUploadExecutorMultipartUpload(t *testing.T) {
	t.Parallel()

	const fileSize = 3 * 1024 * 1024
	fileStore := newTaskTestStore(t)
	cfg, err := fileStore.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Worker.UploadChunkSizeMB = 1
	cfg.Emos.Token = "demo-token"
	cfg.Emos.Storage = "zn_r2_upload"
	if err := fileStore.SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	taskService := NewTaskService(fileStore)
	presigns := []client.EmosMultipartPart{
		{Number: 1, UploadURL: "https://upload.example/part/1"},
		{Number: 2, UploadURL: "https://upload.example/part/2"},
		{Number: 3, UploadURL: "https://upload.example/part/3"},
	}
	emosClient := &stubEmosUploadClient{
		tokenResult: client.EmosUploadTokenResult{
			Storage:          "r2",
			FileID:           "file-multipart",
			UploadType:       "multipart",
			MultipartSizeMin: 1024 * 1024,
			MultipartSizeMax: 1024 * 1024,
		},
		presigns: presigns,
	}
	executor := NewUploadExecutor(taskService, emosClient, nil, nil)
	taskID := seedUploadPendingTaskWithFileSize(t, taskService, fileStore, fileSize)

	if err := executor.Execute(context.Background(), taskID); err != nil {
		t.Fatalf("execute multipart upload task: %v", err)
	}

	if emosClient.presignCalls != 1 {
		t.Fatalf("expected one presign request, got %d", emosClient.presignCalls)
	}
	uploadedParts := emosClient.uploadedPartNumbersSnapshot()
	sort.Ints(uploadedParts)
	if !intSliceEqual(uploadedParts, []int{1, 2, 3}) {
		t.Fatalf("expected uploaded parts 1,2,3, got %v", emosClient.uploadedPartNumbers)
	}
	if emosClient.completeCalls != 1 {
		t.Fatalf("expected one complete call, got %d", emosClient.completeCalls)
	}

	task, err := taskService.GetTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Status != model.TaskStatusCompleted {
		t.Fatalf("expected completed status, got %s", task.Status)
	}
	if len(task.Upload.MultipartPresigns) != 3 {
		t.Fatalf("expected persisted multipart presigns, got %d", len(task.Upload.MultipartPresigns))
	}
	if len(task.Upload.MultipartParts) != 3 {
		t.Fatalf("expected persisted multipart parts, got %d", len(task.Upload.MultipartParts))
	}
	if task.Upload.UploadedBytes != fileSize {
		t.Fatalf("expected uploaded bytes %d, got %d", fileSize, task.Upload.UploadedBytes)
	}
}

func TestUploadExecutorMultipartUploadConcurrent(t *testing.T) {
	t.Parallel()

	const fileSize = 6 * 1024 * 1024
	fileStore := newTaskTestStore(t)
	cfg, err := fileStore.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Worker.UploadChunkSizeMB = 1
	cfg.Worker.UploadPartConcurrency = 3
	cfg.Emos.Token = "demo-token"
	cfg.Emos.Storage = "zn_r2_upload"
	if err := fileStore.SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	taskService := NewTaskService(fileStore)
	presigns := make([]client.EmosMultipartPart, 6)
	for i := 1; i <= 6; i++ {
		presigns[i-1] = client.EmosMultipartPart{
			Number:    i,
			UploadURL: fmt.Sprintf("https://upload.example/part/%d", i),
		}
	}
	base := &stubEmosUploadClient{
		tokenResult: client.EmosUploadTokenResult{
			Storage:          "r2",
			FileID:           "file-multipart",
			UploadType:       "multipart",
			MultipartSizeMin: 1024 * 1024,
			MultipartSizeMax: 1024 * 1024,
		},
		presigns: presigns,
	}
	blockingClient := &blockingMultipartClient{
		stubEmosUploadClient: base,
		started:              make(chan struct{}, 1),
		release:              make(chan struct{}),
	}
	executor := NewUploadExecutor(taskService, blockingClient, nil, nil)
	taskID := seedUploadPendingTaskWithFileSize(t, taskService, fileStore, fileSize)

	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() { close(blockingClient.release) })
	}
	defer release()

	done := make(chan error, 1)
	go func() {
		done <- executor.Execute(context.Background(), taskID)
	}()

	select {
	case <-blockingClient.started:
	case <-time.After(2 * time.Second):
		t.Fatal("multipart upload did not start enough concurrent workers")
	}
	if active := blockingClient.maxActive.Load(); active < 3 {
		t.Fatalf("expected at least 3 concurrent part uploads, got %d", active)
	}

	release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("execute concurrent multipart upload task: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent multipart upload did not finish")
	}

	if base.presignCalls != 1 {
		t.Fatalf("expected one presign request, got %d", base.presignCalls)
	}
	uploadedParts := base.uploadedPartNumbersSnapshot()
	sort.Ints(uploadedParts)
	if !intSliceEqual(uploadedParts, []int{1, 2, 3, 4, 5, 6}) {
		t.Fatalf("expected uploaded parts 1..6, got %v", uploadedParts)
	}
	if base.completeCalls != 1 {
		t.Fatalf("expected one complete call, got %d", base.completeCalls)
	}

	task, err := taskService.GetTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Status != model.TaskStatusCompleted {
		t.Fatalf("expected completed status, got %s", task.Status)
	}
	if len(task.Upload.MultipartParts) != 6 {
		t.Fatalf("expected six persisted multipart parts, got %d", len(task.Upload.MultipartParts))
	}
}

func TestUploadExecutorMultipartResumeSkipsUploadedParts(t *testing.T) {
	t.Parallel()

	const fileSize = 3 * 1024 * 1024
	fileStore := newTaskTestStore(t)
	cfg, err := fileStore.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Worker.UploadChunkSizeMB = 1
	cfg.Emos.Token = "demo-token"
	if err := fileStore.SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	taskService := NewTaskService(fileStore)
	presigns := []model.UploadMultipartPart{
		{Number: 1, UploadURL: "https://upload.example/part/1"},
		{Number: 2, UploadURL: "https://upload.example/part/2"},
		{Number: 3, UploadURL: "https://upload.example/part/3"},
	}
	emosClient := &stubEmosUploadClient{}
	executor := NewUploadExecutor(taskService, emosClient, nil, nil)
	taskID := seedUploadPendingTaskWithFileSize(t, taskService, fileStore, fileSize)

	if _, err := fileStore.UpdateTask(taskID, func(task *model.Task) error {
		task.Upload = model.TaskUpload{
			Storage:           "r2",
			FileID:            "file-multipart",
			UploadType:        "multipart",
			MultipartSizeMin:  1024 * 1024,
			MultipartSizeMax:  1024 * 1024,
			MultipartPresigns: presigns,
			MultipartParts: []model.UploadMultipartPart{
				{Number: 1, UploadURL: "https://upload.example/part/1", ETag: "etag-1"},
			},
			TotalBytes:    fileSize,
			UploadedBytes: 1024 * 1024,
			Progress:      33,
			Status:        "pending",
		}
		return nil
	}); err != nil {
		t.Fatalf("seed multipart resume context: %v", err)
	}

	if err := executor.Execute(context.Background(), taskID); err != nil {
		t.Fatalf("execute multipart resume task: %v", err)
	}

	if emosClient.presignCalls != 0 {
		t.Fatalf("expected no presign request on resume, got %d", emosClient.presignCalls)
	}
	uploadedParts := emosClient.uploadedPartNumbersSnapshot()
	sort.Ints(uploadedParts)
	if !intSliceEqual(uploadedParts, []int{2, 3}) {
		t.Fatalf("expected only parts 2,3 to be uploaded, got %v", emosClient.uploadedPartNumbers)
	}
	if emosClient.completeCalls != 1 {
		t.Fatalf("expected one complete call, got %d", emosClient.completeCalls)
	}

	task, err := taskService.GetTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Status != model.TaskStatusCompleted {
		t.Fatalf("expected completed status, got %s", task.Status)
	}
	if len(task.Upload.MultipartParts) != 3 {
		t.Fatalf("expected three persisted multipart parts, got %d", len(task.Upload.MultipartParts))
	}
	if task.Upload.UploadedBytes != fileSize {
		t.Fatalf("expected uploaded bytes %d, got %d", fileSize, task.Upload.UploadedBytes)
	}
}

func TestUploadExecutorMultipartStalePresignCountRefreshesUploadContext(t *testing.T) {
	t.Parallel()

	const fileSize = 3 * 1024 * 1024
	fileStore := newTaskTestStore(t)
	cfg, err := fileStore.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Worker.UploadChunkSizeMB = 1
	cfg.Emos.Token = "demo-token"
	cfg.Emos.Storage = "zn_r2_upload"
	if err := fileStore.SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	taskService := NewTaskService(fileStore)
	presigns := []client.EmosMultipartPart{
		{Number: 1, UploadURL: "https://upload.example/new/1"},
		{Number: 2, UploadURL: "https://upload.example/new/2"},
		{Number: 3, UploadURL: "https://upload.example/new/3"},
	}
	emosClient := &stubEmosUploadClient{
		tokenResult: client.EmosUploadTokenResult{
			Storage:          "r2",
			FileID:           "file-new",
			UploadType:       "multipart",
			MultipartSizeMin: 1024 * 1024,
			MultipartSizeMax: 1024 * 1024,
		},
		presigns: presigns,
	}
	executor := NewUploadExecutor(taskService, emosClient, nil, nil)
	taskID := seedUploadPendingTaskWithFileSize(t, taskService, fileStore, fileSize)

	if _, err := fileStore.UpdateTask(taskID, func(task *model.Task) error {
		task.Upload = model.TaskUpload{
			Storage:          "r2",
			FileID:           "file-old",
			UploadType:       "multipart",
			MultipartSizeMin: 1024 * 1024,
			MultipartSizeMax: 1024 * 1024,
			MultipartPresigns: []model.UploadMultipartPart{
				{Number: 1, UploadURL: "https://upload.example/old/1"},
				{Number: 2, UploadURL: "https://upload.example/old/2"},
			},
			MultipartParts: []model.UploadMultipartPart{
				{Number: 1, UploadURL: "https://upload.example/old/1", ETag: "stale-etag-1"},
			},
			TotalBytes:    fileSize,
			UploadedBytes: 1024 * 1024,
			Progress:      33,
			Status:        "pending",
		}
		return nil
	}); err != nil {
		t.Fatalf("seed stale multipart context: %v", err)
	}

	if err := executor.Execute(context.Background(), taskID); err != nil {
		t.Fatalf("execute multipart upload with stale presign count: %v", err)
	}

	if emosClient.tokenCalls != 1 {
		t.Fatalf("expected one refreshed upload token, got %d", emosClient.tokenCalls)
	}
	if emosClient.presignCalls != 1 {
		t.Fatalf("expected one presign request after refresh, got %d", emosClient.presignCalls)
	}
	uploadedParts := emosClient.uploadedPartNumbersSnapshot()
	sort.Ints(uploadedParts)
	if !intSliceEqual(uploadedParts, []int{1, 2, 3}) {
		t.Fatalf("expected uploaded parts 1,2,3, got %v", emosClient.uploadedPartNumbers)
	}
	if emosClient.completeCalls != 1 {
		t.Fatalf("expected one complete call, got %d", emosClient.completeCalls)
	}

	task, err := taskService.GetTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Status != model.TaskStatusCompleted {
		t.Fatalf("expected completed status, got %s", task.Status)
	}
	if task.Upload.FileID != "file-new" {
		t.Fatalf("expected refreshed file id file-new, got %s", task.Upload.FileID)
	}
	if len(task.Upload.MultipartParts) != 3 {
		t.Fatalf("expected three persisted multipart parts, got %d", len(task.Upload.MultipartParts))
	}
}

func TestUploadExecutorMultipartStaleFileIDRefreshesUploadContext(t *testing.T) {
	t.Parallel()

	const fileSize = 3 * 1024 * 1024
	fileStore := newTaskTestStore(t)
	cfg, err := fileStore.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Worker.UploadChunkSizeMB = 1
	cfg.Emos.Token = "demo-token"
	cfg.Emos.Storage = "zn_r2_upload"
	if err := fileStore.SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	taskService := NewTaskService(fileStore)
	presigns := []client.EmosMultipartPart{
		{Number: 1, UploadURL: "https://upload.example/new/1"},
		{Number: 2, UploadURL: "https://upload.example/new/2"},
		{Number: 3, UploadURL: "https://upload.example/new/3"},
	}
	base := &stubEmosUploadClient{
		tokenResult: client.EmosUploadTokenResult{
			Storage:          "r2",
			FileID:           "file-new",
			UploadType:       "multipart",
			MultipartSizeMin: 1024 * 1024,
			MultipartSizeMax: 1024 * 1024,
		},
		presigns: presigns,
	}
	emosClient := &stalePresignUploadClient{
		stubEmosUploadClient: base,
		staleFileID:          "file-old",
	}
	executor := NewUploadExecutor(taskService, emosClient, nil, nil)
	taskID := seedUploadPendingTaskWithFileSize(t, taskService, fileStore, fileSize)

	if _, err := fileStore.UpdateTask(taskID, func(task *model.Task) error {
		task.Upload = model.TaskUpload{
			Storage:          "r2",
			FileID:           "file-old",
			UploadURL:        "https://upload.example/old-session",
			UploadType:       "multipart",
			MultipartSizeMin: 1024 * 1024,
			MultipartSizeMax: 1024 * 1024,
			TotalBytes:       fileSize,
			Status:           "pending",
		}
		return nil
	}); err != nil {
		t.Fatalf("seed stale multipart file id: %v", err)
	}

	if err := executor.Execute(context.Background(), taskID); err != nil {
		t.Fatalf("execute multipart upload with stale file id: %v", err)
	}

	if base.tokenCalls != 1 {
		t.Fatalf("expected one refreshed upload token, got %d", base.tokenCalls)
	}
	if emosClient.presignCalls != 2 {
		t.Fatalf("expected old and new presign requests, got %d", emosClient.presignCalls)
	}
	if base.completeCalls != 1 {
		t.Fatalf("expected one complete call, got %d", base.completeCalls)
	}

	task, err := taskService.GetTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Status != model.TaskStatusCompleted {
		t.Fatalf("expected completed status, got %s", task.Status)
	}
	if task.Upload.FileID != "file-new" {
		t.Fatalf("expected refreshed file id file-new, got %s", task.Upload.FileID)
	}
	if len(task.Upload.MultipartParts) != 3 {
		t.Fatalf("expected three persisted multipart parts, got %d", len(task.Upload.MultipartParts))
	}
}

func TestUploadExecutorCancelThenRetryDoesNotFailTask(t *testing.T) {
	t.Parallel()

	const fileSize = 3 * 1024 * 1024
	fileStore := newTaskTestStore(t)
	cfg, err := fileStore.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Worker.UploadChunkSizeMB = 1
	cfg.Worker.UploadPartConcurrency = 1
	cfg.Emos.Token = "demo-token"
	cfg.Emos.Storage = "zn_r2_upload"
	if err := fileStore.SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	taskService := NewTaskService(fileStore)
	presigns := []client.EmosMultipartPart{
		{Number: 1, UploadURL: "https://upload.example/part/1"},
		{Number: 2, UploadURL: "https://upload.example/part/2"},
		{Number: 3, UploadURL: "https://upload.example/part/3"},
	}
	base := &stubEmosUploadClient{
		tokenResult: client.EmosUploadTokenResult{
			Storage:          "r2",
			FileID:           "file-multipart",
			UploadType:       "multipart",
			MultipartSizeMin: 1024 * 1024,
			MultipartSizeMax: 1024 * 1024,
		},
		presigns: presigns,
	}
	blockingClient := &cancelableMultipartClient{
		stubEmosUploadClient: base,
		firstStarted:         make(chan struct{}, 1),
		releaseFirst:         make(chan struct{}),
	}
	executor := NewUploadExecutor(taskService, blockingClient, nil, nil)
	taskID := seedUploadPendingTaskWithFileSize(t, taskService, fileStore, fileSize)

	done := make(chan error, 1)
	go func() {
		done <- executor.Execute(context.Background(), taskID)
	}()

	select {
	case <-blockingClient.firstStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("multipart upload did not start")
	}

	if _, err := taskService.CancelTask(context.Background(), taskID); err != nil {
		t.Fatalf("cancel task: %v", err)
	}
	if _, err := taskService.RetryTask(context.Background(), taskID); err != nil {
		t.Fatalf("retry canceled task: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("canceled upload returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled upload did not stop")
	}

	task, err := taskService.GetTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get task after cancel/retry: %v", err)
	}
	if task.Status != model.TaskStatusUploadPending {
		t.Fatalf("expected upload_pending after retry, got %s", task.Status)
	}
	if task.RetryCount != 1 {
		t.Fatalf("expected retry count 1, got %d", task.RetryCount)
	}
	if task.Result.ErrorMessage != "" {
		t.Fatalf("expected no error after cancel/retry, got %q", task.Result.ErrorMessage)
	}

	completeClient := &stubEmosUploadClient{}
	secondExecutor := NewUploadExecutor(taskService, completeClient, nil, nil)
	if err := secondExecutor.Execute(context.Background(), taskID); err != nil {
		t.Fatalf("execute resumed upload after retry: %v", err)
	}

	task, err = taskService.GetTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get completed task: %v", err)
	}
	if task.Status != model.TaskStatusCompleted {
		t.Fatalf("expected completed status, got %s", task.Status)
	}
	if len(task.Upload.MultipartParts) != 3 {
		t.Fatalf("expected three persisted multipart parts, got %d", len(task.Upload.MultipartParts))
	}
}

func TestRecordMultipartPartRejectsStaleRetryAttempt(t *testing.T) {
	t.Parallel()

	fileStore := newTaskTestStore(t)
	taskService := NewTaskService(fileStore)
	taskID := seedUploadPendingTask(t, taskService, fileStore)
	if _, err := fileStore.UpdateTask(taskID, func(task *model.Task) error {
		task.Status = model.TaskStatusUploading
		task.RetryCount = 1
		task.Upload = model.TaskUpload{
			Storage:          "r2",
			FileID:           "file-multipart",
			UploadType:       "multipart",
			MultipartSizeMin: 1024 * 1024,
			MultipartSizeMax: 1024 * 1024,
			MultipartPresigns: []model.UploadMultipartPart{
				{Number: 1, UploadURL: "https://upload.example/part/1"},
			},
			TotalBytes: 1024,
			Status:     "uploading",
		}
		return nil
	}); err != nil {
		t.Fatalf("seed uploading task: %v", err)
	}

	if _, err := taskService.RecordMultipartPart(context.Background(), taskID, 0, model.UploadMultipartPart{
		Number: 1,
		ETag:   "stale-etag",
	}, client.EmosUploadProgress{UploadedBytes: 1024, TotalBytes: 1024}); err == nil {
		t.Fatal("expected stale multipart attempt to be rejected")
	}

	task, err := taskService.GetTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if len(task.Upload.MultipartParts) != 0 {
		t.Fatalf("stale attempt must not record a multipart part, got %d parts", len(task.Upload.MultipartParts))
	}

	updated, err := taskService.RecordMultipartPart(context.Background(), taskID, 1, model.UploadMultipartPart{
		Number: 1,
		ETag:   "etag-1",
	}, client.EmosUploadProgress{UploadedBytes: 1024, TotalBytes: 1024})
	if err != nil {
		t.Fatalf("record current multipart attempt: %v", err)
	}
	if len(updated.Upload.MultipartParts) != 1 {
		t.Fatalf("expected one persisted multipart part, got %d", len(updated.Upload.MultipartParts))
	}
}

func seedUploadPendingTask(t *testing.T, taskService *TaskService, fileStore *store.FileStore) string {
	return seedUploadPendingTaskWithFileSize(t, taskService, fileStore, 1024)
}

func seedUploadPendingTaskWithFileSize(t *testing.T, taskService *TaskService, fileStore *store.FileStore, size int) string {
	t.Helper()

	scan := seedTaskTestScan(t, fileStore)

	result, err := taskService.BatchCreateTasks(context.Background(), BatchCreateTasksRequest{
		ScanSessionID: scan.ID,
		ItemIDs:       []string{"item-confirmed"},
	})
	if err != nil {
		t.Fatalf("batch create tasks: %v", err)
	}

	taskID := result.Created[0].TaskID
	task, err := taskService.PrepareTaskDownload(context.Background(), taskID)
	if err != nil {
		t.Fatalf("prepare task download: %v", err)
	}
	if err := os.WriteFile(task.Download.LocalPath, make([]byte, size), 0o644); err != nil {
		t.Fatalf("write local file: %v", err)
	}

	if _, err := taskService.MarkDownloadCompleted(context.Background(), taskID, client.DownloadStatus{
		Status:          "complete",
		TotalLength:     int64(size),
		CompletedLength: int64(size),
		Files:           []client.DownloadFile{{Path: task.Download.LocalPath}},
	}); err != nil {
		t.Fatalf("mark download completed: %v", err)
	}

	return taskID
}

func intSliceEqual(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

type stubSaveAttempt struct {
	result client.EmosSaveVideoResult
	err    error
}

type stubEmosUploadClient struct {
	tokenResult    client.EmosUploadTokenResult
	tokenErr       error
	tokenCalls     int
	uploadProgress []client.EmosUploadProgress
	uploadErr      error
	saveAttempts   []stubSaveAttempt
	presigns       []client.EmosMultipartPart
	uploadPartErr  error
	completeErr    error
	completeCalls  int

	uploadFileCalls     []string
	uploadTokenRequests []client.EmosUploadTokenRequest
	presignCalls        int
	uploadedPartsMu     sync.Mutex
	uploadedPartNumbers []int
}

func (c *stubEmosUploadClient) GetVideoTree(context.Context, client.EmosAccess, int64, string) (client.EmosVideoTree, error) {
	return client.EmosVideoTree{}, nil
}

func (c *stubEmosUploadClient) GetVideoBase(context.Context, client.EmosAccess, string, int64) (client.EmosVideoBase, error) {
	return client.EmosVideoBase{}, nil
}

func (c *stubEmosUploadClient) GetUploadToken(_ context.Context, _ client.EmosAccess, req client.EmosUploadTokenRequest) (client.EmosUploadTokenResult, error) {
	c.tokenCalls++
	c.uploadTokenRequests = append(c.uploadTokenRequests, req)
	return c.tokenResult, c.tokenErr
}

func (c *stubEmosUploadClient) UploadFile(_ context.Context, uploadType string, _ string, _ string, _ int64, _ int64, onProgress func(client.EmosUploadProgress) error) error {
	c.uploadFileCalls = append(c.uploadFileCalls, uploadType)
	for _, progress := range c.uploadProgress {
		if onProgress != nil {
			if err := onProgress(progress); err != nil {
				return err
			}
		}
	}
	return c.uploadErr
}

func (c *stubEmosUploadClient) UploadMultipartPart(_ context.Context, part client.EmosMultipartPart, _ string, _ int64, _ int64) (string, error) {
	return c.UploadMultipartPartWithProgress(context.Background(), part, "", 0, 0, nil)
}

func (c *stubEmosUploadClient) UploadMultipartPartWithProgress(_ context.Context, part client.EmosMultipartPart, _ string, _ int64, _ int64, _ func(int64)) (string, error) {
	c.uploadedPartsMu.Lock()
	c.uploadedPartNumbers = append(c.uploadedPartNumbers, part.Number)
	c.uploadedPartsMu.Unlock()
	if c.uploadPartErr != nil {
		return "", c.uploadPartErr
	}
	return fmt.Sprintf("etag-%d", part.Number), nil
}

func (c *stubEmosUploadClient) uploadedPartNumbersSnapshot() []int {
	c.uploadedPartsMu.Lock()
	defer c.uploadedPartsMu.Unlock()
	return append([]int(nil), c.uploadedPartNumbers...)
}

func (c *stubEmosUploadClient) RequestMultipartPresigns(_ context.Context, _ client.EmosAccess, _ string, _ int) ([]client.EmosMultipartPart, error) {
	c.presignCalls++
	return c.presigns, nil
}

func (c *stubEmosUploadClient) CompleteMultipart(_ context.Context, _ client.EmosAccess, _ string, _ []client.EmosMultipartPart) error {
	c.completeCalls++
	return c.completeErr
}

func (c *stubEmosUploadClient) SaveVideo(context.Context, client.EmosAccess, client.EmosSaveVideoRequest) (client.EmosSaveVideoResult, error) {
	if len(c.saveAttempts) == 0 {
		return client.EmosSaveVideoResult{}, nil
	}
	attempt := c.saveAttempts[0]
	c.saveAttempts = c.saveAttempts[1:]
	return attempt.result, attempt.err
}

type blockingMultipartClient struct {
	*stubEmosUploadClient
	started   chan struct{}
	release   chan struct{}
	active    atomic.Int64
	maxActive atomic.Int64
}

type cancelableMultipartClient struct {
	*stubEmosUploadClient
	firstStarted chan struct{}
	releaseFirst chan struct{}
}

type stalePresignUploadClient struct {
	*stubEmosUploadClient
	staleFileID  string
	presignCalls int
}

func (c *stalePresignUploadClient) RequestMultipartPresigns(ctx context.Context, access client.EmosAccess, fileID string, numChunks int) ([]client.EmosMultipartPart, error) {
	c.presignCalls++
	if fileID == c.staleFileID {
		return nil, fmt.Errorf("emos multipart presign request failed: 404 Not Found")
	}
	return c.stubEmosUploadClient.presigns, nil
}

func (c *cancelableMultipartClient) UploadMultipartPart(ctx context.Context, part client.EmosMultipartPart, filePath string, startByte, partSize int64) (string, error) {
	if part.Number == 1 {
		select {
		case c.firstStarted <- struct{}{}:
		default:
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-c.releaseFirst:
		}
	}
	return c.stubEmosUploadClient.UploadMultipartPart(ctx, part, filePath, startByte, partSize)
}

func (c *cancelableMultipartClient) UploadMultipartPartWithProgress(ctx context.Context, part client.EmosMultipartPart, filePath string, startByte, partSize int64, _ func(int64)) (string, error) {
	return c.UploadMultipartPart(ctx, part, filePath, startByte, partSize)
}

func (c *blockingMultipartClient) UploadMultipartPart(ctx context.Context, part client.EmosMultipartPart, filePath string, startByte, partSize int64) (string, error) {
	active := c.active.Add(1)
	for {
		current := c.maxActive.Load()
		if active <= current || c.maxActive.CompareAndSwap(current, active) {
			break
		}
	}
	if active == 3 {
		select {
		case c.started <- struct{}{}:
		default:
		}
	}

	select {
	case <-ctx.Done():
		c.active.Add(-1)
		return "", ctx.Err()
	case <-c.release:
	}
	c.active.Add(-1)
	return c.stubEmosUploadClient.UploadMultipartPart(ctx, part, filePath, startByte, partSize)
}

func (c *blockingMultipartClient) UploadMultipartPartWithProgress(ctx context.Context, part client.EmosMultipartPart, filePath string, startByte, partSize int64, _ func(int64)) (string, error) {
	return c.UploadMultipartPart(ctx, part, filePath, startByte, partSize)
}
