package service

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"emosup/backend/internal/client"
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

// TestDownloadMultiSegmentIsolationRetry verifies that a transient failure on
// one segment only retries that segment (keeping the multi-thread download
// alive) instead of tearing everything down and falling back to single-thread.
func TestDownloadMultiSegmentIsolationRetry(t *testing.T) {
	t.Parallel()

	const fileSize = 6 << 20 // 6MB → 2 segments of 3MB each
	payload := make([]byte, fileSize)
	for i := range payload {
		payload[i] = byte(i % 251)
	}

	var mu sync.Mutex
	segment0Attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, end := parseTestRange(r.Header.Get("Range"))
		if start == 0 && end == fileSize/2-1 {
			// Segment 0 fails once (transient 500), succeeds on retry.
			mu.Lock()
			segment0Attempts++
			first := segment0Attempts
			mu.Unlock()
			if first == 1 {
				http.Error(w, "transient boom", http.StatusInternalServerError)
				return
			}
		}
		http.ServeContent(w, r, "f.bin", time.Time{}, bytes.NewReader(payload))
	}))
	defer srv.Close()

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
	prepared, err := taskService.PrepareTaskDownload(context.Background(), taskID)
	if err != nil {
		t.Fatalf("prepare download: %v", err)
	}

	cfg := model.DefaultAppConfig()
	cfg.OpenList.BaseURL = srv.URL
	executor := NewDownloadExecutor(taskService, nil, nil, nil)

	url := srv.URL + "/f.bin"
	err = executor.downloadMulti(context.Background(), prepared, client.OpenListAccess{}, cfg, url, prepared.Download.LocalPath, 2)
	if err != nil {
		t.Fatalf("downloadMulti: %v", err)
	}

	mu.Lock()
	attempts := segment0Attempts
	mu.Unlock()
	if attempts != 2 {
		t.Fatalf("expected segment 0 to retry once (2 attempts total), got %d", attempts)
	}

	got, err := os.ReadFile(prepared.Download.LocalPath)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("downloaded content mismatch: got %d bytes, want %d", len(got), len(payload))
	}
}

func parseTestRange(value string) (int64, int64) {
	if value == "" {
		return -1, -1
	}
	value = strings.TrimPrefix(value, "bytes=")
	parts := strings.SplitN(value, "-", 2)
	if len(parts) != 2 {
		return -1, -1
	}
	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return -1, -1
	}
	end, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return -1, -1
	}
	return start, end
}


