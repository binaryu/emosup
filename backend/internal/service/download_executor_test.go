package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"emosup/backend/internal/model"
)

func TestDownloadExecutorExecuteQueuedTask(t *testing.T) {
	t.Parallel()

	fileStore := newTaskTestStore(t)
	taskService := NewTaskService(fileStore)
	executor := NewDownloadExecutor(taskService, nil, nil, nil)
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
	executor := NewDownloadExecutor(taskService, nil, nil, nil)
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

func TestCancelStopsDirectDownloadAndKeepsCanceled(t *testing.T) {
	t.Parallel()

	// Slow streaming server so the download is still in flight when we cancel.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		for i := 0; i < 2000; i++ {
			_, _ = w.Write(make([]byte, 512*1024))
			if ok {
				flusher.Flush()
			}
			select {
			case <-r.Context().Done():
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}))
	defer server.Close()

	fileStore := newTaskTestStore(t)
	taskService := NewTaskService(fileStore)
	executor := NewDownloadExecutor(taskService, nil, nil, nil)
	scan := seedTaskTestScan(t, fileStore)

	result, err := taskService.BatchCreateTasks(context.Background(), BatchCreateTasksRequest{
		ScanSessionID: scan.ID,
		ItemIDs:       []string{"item-confirmed"},
	})
	if err != nil {
		t.Fatalf("batch create tasks: %v", err)
	}
	taskID := result.Created[0].TaskID
	if _, err := fileStore.UpdateTask(taskID, func(tk *model.Task) error {
		tk.Source.RawURL = server.URL + "/big.bin"
		tk.Source.FileSize = 1024 * 1024 * 1024
		tk.Download.TotalBytes = tk.Source.FileSize
		return nil
	}); err != nil {
		t.Fatalf("set raw url: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- executor.Execute(context.Background(), taskID) }()

	// Wait until the download is actually in progress (PrepareTaskDownload
	// flips the status to downloading before the first byte is written).
	deadline := time.Now().Add(5 * time.Second)
	for {
		task, getErr := taskService.GetTask(context.Background(), taskID)
		if getErr != nil {
			t.Fatalf("get task: %v", getErr)
		}
		if task.Status == model.TaskStatusDownloading {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("download never started; status=%s completed=%d", task.Status, task.Download.CompletedBytes)
		}
		time.Sleep(20 * time.Millisecond)
	}

	canceled, err := taskService.CancelTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("cancel task: %v", err)
	}
	if canceled.Status != model.TaskStatusCanceled {
		t.Fatalf("expected canceled, got %s", canceled.Status)
	}

	select {
	case err := <-done:
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("Execute expected context.Canceled, got %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("download did not stop after cancel")
	}

	final, err := taskService.GetTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get final task: %v", err)
	}
	if final.Status != model.TaskStatusCanceled {
		t.Fatalf("cancel must not be overwritten; got %s (error=%q)", final.Status, final.Result.ErrorMessage)
	}
}

func TestCancelIsNoopForCompletedTask(t *testing.T) {
	t.Parallel()
	fileStore := newTaskTestStore(t)
	taskService := NewTaskService(fileStore)
	scan := seedTaskTestScan(t, fileStore)
	result, err := taskService.BatchCreateTasks(context.Background(), BatchCreateTasksRequest{
		ScanSessionID: scan.ID,
		ItemIDs:       []string{"item-confirmed"},
	})
	if err != nil {
		t.Fatalf("batch create: %v", err)
	}
	taskID := result.Created[0].TaskID
	if _, err := taskService.CancelTask(context.Background(), taskID); err != nil {
		t.Fatalf("cancel queued task: %v", err)
	}
	// Cancel is idempotent: canceling a canceled task succeeds silently.
	again, err := taskService.CancelTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("second cancel should be a no-op: %v", err)
	}
	if again.Status != model.TaskStatusCanceled {
		t.Fatalf("expected canceled, got %s", again.Status)
	}
}
