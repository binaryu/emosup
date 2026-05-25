package service

import (
	"context"
	"os"
	"testing"

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

	taskService := NewTaskService(fileStore, &stubAria2Client{})
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
	executor := NewUploadExecutor(taskService, emosClient, nil)
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

func TestRetryUploadFailedTaskClearsUploadState(t *testing.T) {
	t.Parallel()

	fileStore := newTaskTestStore(t)
	taskService := NewTaskService(fileStore, &stubAria2Client{})
	taskID := seedUploadPendingTask(t, taskService, fileStore)

	if _, err := fileStore.UpdateTask(taskID, func(task *model.Task) error {
		task.Status = model.TaskStatusUploadFailed
		task.Upload.Status = "failed"
		task.Upload.FileID = "file-123"
		task.Upload.UploadURL = "https://upload.example/session"
		task.Upload.MediaID = "media-123"
		task.Upload.UploadedBytes = 1024
		task.Upload.Progress = 100
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
	if task.Upload.FileID != "" || task.Upload.UploadURL != "" || task.Upload.MediaID != "" {
		t.Fatalf("expected upload context to be cleared, got %#v", task.Upload)
	}
	if task.Upload.UploadedBytes != 0 || task.Upload.Progress != 0 || task.Upload.Speed != 0 {
		t.Fatalf("expected upload progress to be reset, got %#v", task.Upload)
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

func seedUploadPendingTask(t *testing.T, taskService *TaskService, fileStore *store.FileStore) string {
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
	if err := os.WriteFile(task.Download.LocalPath, make([]byte, 1024), 0o644); err != nil {
		t.Fatalf("write local file: %v", err)
	}

	if _, err := taskService.MarkDownloadCompleted(context.Background(), taskID, client.Aria2Status{
		Status:          "complete",
		TotalLength:     1024,
		CompletedLength: 1024,
		Files:           []client.Aria2File{{Path: task.Download.LocalPath}},
	}); err != nil {
		t.Fatalf("mark download completed: %v", err)
	}

	return taskID
}

type stubSaveAttempt struct {
	result client.EmosSaveVideoResult
	err    error
}

type stubEmosUploadClient struct {
	tokenResult    client.EmosUploadTokenResult
	tokenErr       error
	uploadProgress []client.EmosUploadProgress
	uploadErr      error
	saveAttempts   []stubSaveAttempt
}

func (c *stubEmosUploadClient) GetVideoTree(context.Context, client.EmosAccess, int64, string) (client.EmosVideoTree, error) {
	return client.EmosVideoTree{}, nil
}

func (c *stubEmosUploadClient) GetVideoBase(context.Context, client.EmosAccess, string, int64) (client.EmosVideoBase, error) {
	return client.EmosVideoBase{}, nil
}

func (c *stubEmosUploadClient) GetUploadToken(context.Context, client.EmosAccess, client.EmosUploadTokenRequest) (client.EmosUploadTokenResult, error) {
	return c.tokenResult, c.tokenErr
}

func (c *stubEmosUploadClient) UploadFile(_ context.Context, _ string, _ string, _ int64, _ int64, onProgress func(client.EmosUploadProgress) error) error {
	for _, progress := range c.uploadProgress {
		if onProgress != nil {
			if err := onProgress(progress); err != nil {
				return err
			}
		}
	}
	return c.uploadErr
}

func (c *stubEmosUploadClient) SaveVideo(context.Context, client.EmosAccess, client.EmosSaveVideoRequest) (client.EmosSaveVideoResult, error) {
	if len(c.saveAttempts) == 0 {
		return client.EmosSaveVideoResult{}, nil
	}
	attempt := c.saveAttempts[0]
	c.saveAttempts = c.saveAttempts[1:]
	return attempt.result, attempt.err
}
