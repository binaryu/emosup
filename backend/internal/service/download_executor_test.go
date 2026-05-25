package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"emosup/backend/internal/client"
	"emosup/backend/internal/model"
)

func TestDownloadExecutorExecuteQueuedTask(t *testing.T) {
	t.Parallel()

	fileStore := newTaskTestStore(t)
	aria2Client := &sequenceAria2Client{}
	taskService := NewTaskService(fileStore, aria2Client)
	executor := NewDownloadExecutor(taskService, aria2Client, nil)
	scan := seedTaskTestScan(t, fileStore)

	result, err := taskService.BatchCreateTasks(context.Background(), BatchCreateTasksRequest{
		ScanSessionID: scan.ID,
		ItemIDs:       []string{"item-confirmed"},
	})
	if err != nil {
		t.Fatalf("batch create tasks: %v", err)
	}

	task, err := taskService.GetTask(context.Background(), result.Created[0].TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}

	cfg, err := fileStore.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	localPath := filepath.Join(cfg.Aria2.DownloadDir, buildDownloadFileName(task))
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(localPath, make([]byte, 1024), 0o644); err != nil {
		t.Fatalf("write local file: %v", err)
	}

	aria2Client.statuses = []client.Aria2Status{
		{
			GID:             "gid-1",
			Status:          "active",
			TotalLength:     1024,
			CompletedLength: 512,
			DownloadSpeed:   128,
			Files:           []client.Aria2File{{Path: localPath}},
		},
		{
			GID:             "gid-1",
			Status:          "complete",
			TotalLength:     1024,
			CompletedLength: 1024,
			Files:           []client.Aria2File{{Path: localPath}},
		},
	}

	if err := executor.Execute(context.Background(), task.ID); err != nil {
		t.Fatalf("execute task: %v", err)
	}

	task, err = taskService.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task after execution: %v", err)
	}
	if task.Status != model.TaskStatusUploadPending {
		t.Fatalf("expected upload_pending status, got %s", task.Status)
	}
	if task.Download.Aria2GID != "gid-1" {
		t.Fatalf("expected aria2 gid to be saved, got %s", task.Download.Aria2GID)
	}
	if task.Download.Progress != 100 {
		t.Fatalf("expected progress 100, got %v", task.Download.Progress)
	}
}

func TestDownloadExecutorRecoverDownloadingTaskWithoutGIDUsesLocalFile(t *testing.T) {
	t.Parallel()

	fileStore := newTaskTestStore(t)
	aria2Client := &sequenceAria2Client{}
	taskService := NewTaskService(fileStore, aria2Client)
	executor := NewDownloadExecutor(taskService, aria2Client, nil)
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

	task, err = fileStore.UpdateTask(taskID, func(task *model.Task) error {
		task.Status = model.TaskStatusDownloading
		task.Download.Aria2GID = ""
		task.Download.TotalBytes = 1024
		task.Download.LocalPath = task.Download.LocalPath
		return nil
	})
	if err != nil {
		t.Fatalf("update task: %v", err)
	}

	shouldResume, err := executor.RecoverTask(context.Background(), task)
	if err != nil {
		t.Fatalf("recover task: %v", err)
	}
	if shouldResume {
		t.Fatalf("expected no active download resume when local file is reusable")
	}

	recoveredTask, err := taskService.GetTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if recoveredTask.Status != model.TaskStatusUploadPending {
		t.Fatalf("expected upload_pending after recovery, got %s", recoveredTask.Status)
	}
}

type sequenceAria2Client struct {
	statuses []client.Aria2Status
}

func (c *sequenceAria2Client) AddURI(_ context.Context, _ client.Aria2Access, _ string, _ client.Aria2AddURIOptions) (string, error) {
	return "gid-1", nil
}

func (c *sequenceAria2Client) TellStatus(_ context.Context, _ client.Aria2Access, _ string) (client.Aria2Status, error) {
	if len(c.statuses) == 0 {
		return client.Aria2Status{GID: "gid-1", Status: "complete"}, nil
	}
	status := c.statuses[0]
	c.statuses = c.statuses[1:]
	return status, nil
}

func (c *sequenceAria2Client) ForceRemove(_ context.Context, _ client.Aria2Access, _ string) error {
	return nil
}
