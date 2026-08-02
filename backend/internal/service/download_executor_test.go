package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"emosup/backend/internal/model"
)

func TestDownloadExecutorExecuteQueuedTask(t *testing.T) {
	t.Parallel()

	fileStore := newTaskTestStore(t)
	taskService := NewTaskService(fileStore)
	executor := NewDownloadExecutor(taskService, nil, nil)
	scan := seedTaskTestScan(t, fileStore)

	result, err := taskService.BatchCreateTasks(context.Background(), BatchCreateTasksRequest{
		ScanSessionID: scan.ID,
		ItemIDs:       []string{"item-confirmed"},
	})
	if err != nil {
		t.Fatalf("batch create tasks: %v", err)
	}
	taskID := result.Created[0].TaskID

	// Prepare once to get the deterministic local path, write a complete file, then
	// leave status as downloading so Execute/direct path can finish from local file.
	prepared, err := taskService.PrepareTaskDownload(context.Background(), taskID)
	if err != nil {
		t.Fatalf("prepare download: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(prepared.Download.LocalPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	size := prepared.Source.FileSize
	if size <= 0 {
		size = 1024
	}
	if err := os.WriteFile(prepared.Download.LocalPath, make([]byte, size), 0o644); err != nil {
		t.Fatalf("write local file: %v", err)
	}
	// Reset to queued so Execute enters downloadDirect from the start.
	if _, err := fileStore.UpdateTask(taskID, func(tk *model.Task) error {
		tk.Status = model.TaskStatusQueued
		return nil
	}); err != nil {
		t.Fatalf("reset to queued: %v", err)
	}

	if err := executor.Execute(context.Background(), taskID); err != nil {
		t.Fatalf("execute task: %v", err)
	}

	task, err := taskService.GetTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get task after execution: %v", err)
	}
	if task.Status != model.TaskStatusUploadPending {
		t.Fatalf("expected upload_pending status, got %s", task.Status)
	}
	if task.Download.Progress != 100 {
		t.Fatalf("expected progress 100, got %v", task.Download.Progress)
	}
	if task.Download.LocalPath == "" {
		t.Fatalf("expected local path to be set")
	}
}

func TestDownloadExecutorRecoverDownloadingTaskWithoutGIDUsesLocalFile(t *testing.T) {
	t.Parallel()

	fileStore := newTaskTestStore(t)
	taskService := NewTaskService(fileStore)
	executor := NewDownloadExecutor(taskService, nil, nil)
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
		task.Download.TotalBytes = 1024
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

