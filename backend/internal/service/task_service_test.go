package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"emosup/backend/internal/client"
	"emosup/backend/internal/model"
	"emosup/backend/internal/store"
)

func TestBatchCreateTasks(t *testing.T) {
	t.Parallel()

	fileStore := newTaskTestStore(t)
	taskService := NewTaskService(fileStore, &stubAria2Client{})
	scan := seedTaskTestScan(t, fileStore)

	result, err := taskService.BatchCreateTasks(context.Background(), BatchCreateTasksRequest{
		ScanSessionID: scan.ID,
		ItemIDs:       []string{"item-confirmed", "item-unconfirmed", "item-missing"},
	})
	if err != nil {
		t.Fatalf("batch create tasks: %v", err)
	}

	if len(result.Created) != 1 {
		t.Fatalf("expected 1 created task, got %d", len(result.Created))
	}
	if len(result.Failed) != 2 {
		t.Fatalf("expected 2 failed items, got %d", len(result.Failed))
	}

	task, err := taskService.GetTask(context.Background(), result.Created[0].TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}

	if task.ScanItemID != "item-confirmed" {
		t.Fatalf("expected scan item id to be copied, got %s", task.ScanItemID)
	}
	if task.Source.Path != "/anime/demo/S01E01.mkv" {
		t.Fatalf("unexpected source path: %s", task.Source.Path)
	}
	if task.Target.ItemType != "ve" || task.Target.ItemID != 101 {
		t.Fatalf("unexpected target snapshot: %#v", task.Target)
	}
	if task.Upload.Storage != "default" {
		t.Fatalf("expected upload storage default, got %s", task.Upload.Storage)
	}

	taskLog, err := taskService.GetTaskLog(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task log: %v", err)
	}
	if len(taskLog.Items) != 1 {
		t.Fatalf("expected 1 task log entry, got %d", len(taskLog.Items))
	}
	if taskLog.Items[0].Message != "task created from scan item" {
		t.Fatalf("unexpected task log message: %s", taskLog.Items[0].Message)
	}
}

func TestBatchCreateTasksPreventsDuplicateActiveTask(t *testing.T) {
	t.Parallel()

	fileStore := newTaskTestStore(t)
	taskService := NewTaskService(fileStore, &stubAria2Client{})
	scan := seedTaskTestScan(t, fileStore)

	first, err := taskService.BatchCreateTasks(context.Background(), BatchCreateTasksRequest{
		ScanSessionID: scan.ID,
		ItemIDs:       []string{"item-confirmed"},
	})
	if err != nil {
		t.Fatalf("first batch create: %v", err)
	}
	if len(first.Created) != 1 {
		t.Fatalf("expected first task creation to succeed")
	}

	second, err := taskService.BatchCreateTasks(context.Background(), BatchCreateTasksRequest{
		ScanSessionID: scan.ID,
		ItemIDs:       []string{"item-confirmed"},
	})
	if err != nil {
		t.Fatalf("second batch create: %v", err)
	}
	if len(second.Created) != 0 || len(second.Failed) != 1 {
		t.Fatalf("expected duplicate creation to fail, got %#v", second)
	}
	if second.Failed[0].Reason != "active task already exists for scan item" {
		t.Fatalf("unexpected duplicate failure reason: %s", second.Failed[0].Reason)
	}
}

func TestCancelAndRetryTask(t *testing.T) {
	t.Parallel()

	fileStore := newTaskTestStore(t)
	taskService := NewTaskService(fileStore, &stubAria2Client{})
	scan := seedTaskTestScan(t, fileStore)

	result, err := taskService.BatchCreateTasks(context.Background(), BatchCreateTasksRequest{
		ScanSessionID: scan.ID,
		ItemIDs:       []string{"item-confirmed"},
	})
	if err != nil {
		t.Fatalf("batch create tasks: %v", err)
	}

	taskID := result.Created[0].TaskID

	canceledTask, err := taskService.CancelTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("cancel task: %v", err)
	}
	if canceledTask.Status != model.TaskStatusCanceled {
		t.Fatalf("expected canceled status, got %s", canceledTask.Status)
	}
	if canceledTask.FinishedAt == nil {
		t.Fatalf("expected finished_at to be set when canceled")
	}

	retriedTask, err := taskService.RetryTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("retry task: %v", err)
	}
	if retriedTask.Status != model.TaskStatusQueued {
		t.Fatalf("expected queued status after retry, got %s", retriedTask.Status)
	}
	if retriedTask.RetryCount != 1 {
		t.Fatalf("expected retry_count 1, got %d", retriedTask.RetryCount)
	}
	if retriedTask.FinishedAt != nil {
		t.Fatalf("expected finished_at to be cleared after retry")
	}
	if retriedTask.Download.Aria2GID != "" || retriedTask.Download.Progress != 0 {
		t.Fatalf("expected retry to clear download state, got %#v", retriedTask.Download)
	}

	taskLog, err := taskService.GetTaskLog(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get task log: %v", err)
	}
	if len(taskLog.Items) != 3 {
		t.Fatalf("expected 3 log entries, got %d", len(taskLog.Items))
	}
	if taskLog.Items[1].Message != "task canceled by user" {
		t.Fatalf("unexpected cancel log: %s", taskLog.Items[1].Message)
	}
	if !strings.Contains(taskLog.Items[2].Message, "task retried from canceled") {
		t.Fatalf("unexpected retry log: %s", taskLog.Items[2].Message)
	}
}

func TestCancelDownloadingTaskRemovesAria2(t *testing.T) {
	t.Parallel()

	fileStore := newTaskTestStore(t)
	aria2Client := &stubAria2Client{}
	taskService := NewTaskService(fileStore, aria2Client)
	scan := seedTaskTestScan(t, fileStore)

	result, err := taskService.BatchCreateTasks(context.Background(), BatchCreateTasksRequest{
		ScanSessionID: scan.ID,
		ItemIDs:       []string{"item-confirmed"},
	})
	if err != nil {
		t.Fatalf("batch create tasks: %v", err)
	}

	taskID := result.Created[0].TaskID
	if _, err := taskService.PrepareTaskDownload(context.Background(), taskID); err != nil {
		t.Fatalf("prepare task download: %v", err)
	}
	if _, err := taskService.AttachAria2GID(context.Background(), taskID, "gid-123"); err != nil {
		t.Fatalf("attach gid: %v", err)
	}

	canceledTask, err := taskService.CancelTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("cancel task: %v", err)
	}
	if canceledTask.Status != model.TaskStatusCanceled {
		t.Fatalf("expected canceled status, got %s", canceledTask.Status)
	}
	if aria2Client.removedGID != "gid-123" {
		t.Fatalf("expected aria2 gid to be removed, got %s", aria2Client.removedGID)
	}
}

func TestRetryDownloadFailedTaskRefreshesRawURL(t *testing.T) {
	t.Parallel()

	fileStore := newTaskTestStore(t)
	openListClient := &stubOpenListClient{rawURL: "https://openlist.example/raw/refreshed"}
	taskService := NewTaskService(fileStore, &stubAria2Client{}, openListClient)
	scan := seedTaskTestScan(t, fileStore)

	result, err := taskService.BatchCreateTasks(context.Background(), BatchCreateTasksRequest{
		ScanSessionID: scan.ID,
		ItemIDs:       []string{"item-confirmed"},
	})
	if err != nil {
		t.Fatalf("batch create tasks: %v", err)
	}

	taskID := result.Created[0].TaskID
	if _, err := fileStore.UpdateTask(taskID, func(task *model.Task) error {
		task.Status = model.TaskStatusDownloadFailed
		task.Source.RawURL = "https://openlist.example/raw/expired"
		task.Result.ErrorMessage = "403 forbidden"
		return nil
	}); err != nil {
		t.Fatalf("prepare download failed task: %v", err)
	}

	task, err := taskService.RetryTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("retry task: %v", err)
	}

	if task.Status != model.TaskStatusQueued {
		t.Fatalf("expected queued status, got %s", task.Status)
	}
	if task.Source.RawURL != "https://openlist.example/raw/refreshed" {
		t.Fatalf("expected refreshed raw url, got %s", task.Source.RawURL)
	}
}

func newTaskTestStore(t *testing.T) *store.FileStore {
	t.Helper()

	fileStore := store.NewFileStore(t.TempDir())
	if err := fileStore.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}

	cfg := model.DefaultAppConfig()
	cfg.Emos.Storage = "default"
	cfg.OpenList.BaseURL = "https://openlist.example"
	if err := fileStore.SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	return fileStore
}

func seedTaskTestScan(t *testing.T, fileStore *store.FileStore) model.ScanSession {
	t.Helper()

	season := 1
	episode := 1
	now := time.Now()
	scan := model.ScanSession{
		ID:             "scan-test",
		Path:           "/anime/demo",
		TMDBID:         7788,
		Status:         model.ScanSessionStatusCompleted,
		TotalCount:     2,
		MatchedCount:   1,
		UnmatchedCount: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
		Items: []model.ScanItem{
			{
				ID:               "item-confirmed",
				ScanSessionID:    "scan-test",
				OpenListPath:     "/anime/demo/S01E01.mkv",
				FileName:         "S01E01.mkv",
				FileSize:         1024,
				RawURL:           "https://openlist.example/raw/1",
				IsVideo:          true,
				Parsed:           model.ParsedEpisodeInfo{Season: &season, Episode: &episode, IsSpecial: false},
				SelectedItemType: "ve",
				SelectedItemID:   101,
				SelectedTitle:    "Demo - S01E01",
				Confirmed:        true,
				CreatedAt:        now,
				UpdatedAt:        now,
			},
			{
				ID:               "item-unconfirmed",
				ScanSessionID:    "scan-test",
				OpenListPath:     "/anime/demo/S01E02.mkv",
				FileName:         "S01E02.mkv",
				FileSize:         2048,
				RawURL:           "https://openlist.example/raw/2",
				IsVideo:          true,
				Parsed:           model.ParsedEpisodeInfo{Season: &season, Episode: &episode, IsSpecial: false},
				SelectedItemType: "ve",
				SelectedItemID:   102,
				SelectedTitle:    "Demo - S01E02",
				Confirmed:        false,
				CreatedAt:        now,
				UpdatedAt:        now,
			},
		},
	}

	if err := fileStore.SaveScan(scan); err != nil {
		t.Fatalf("save scan: %v", err)
	}

	return scan
}

type stubAria2Client struct {
	removedGID string
}

type stubOpenListClient struct {
	rawURL string
}

func (c *stubAria2Client) AddURI(_ context.Context, _ client.Aria2Access, _ string, _ client.Aria2AddURIOptions) (string, error) {
	return "stub-gid", nil
}

func (c *stubAria2Client) TellStatus(_ context.Context, _ client.Aria2Access, _ string) (client.Aria2Status, error) {
	return client.Aria2Status{}, nil
}

func (c *stubAria2Client) ForceRemove(_ context.Context, _ client.Aria2Access, gid string) error {
	c.removedGID = gid
	return nil
}

func (c *stubOpenListClient) List(context.Context, client.OpenListAccess, string) ([]client.OpenListEntry, error) {
	return nil, nil
}

func (c *stubOpenListClient) GetRawLink(context.Context, client.OpenListAccess, string) (string, error) {
	return c.rawURL, nil
}
