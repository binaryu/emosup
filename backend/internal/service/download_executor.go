package service

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"emosup/backend/internal/client"
	"emosup/backend/internal/model"
	"emosup/backend/internal/eventbus"
)

type DownloadExecutor struct {
	taskService *TaskService
	aria2Client client.Aria2Client
	eventBus    *eventbus.Bus
}

func NewDownloadExecutor(taskService *TaskService, aria2Client client.Aria2Client, eventBus *eventbus.Bus) *DownloadExecutor {
	return &DownloadExecutor{
		taskService: taskService,
		aria2Client: aria2Client,
		eventBus:    eventBus,
	}
}

func (e *DownloadExecutor) Execute(ctx context.Context, taskID string) error {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Hour)
	defer cancel()

	task, err := e.taskService.GetTask(ctx, taskID)
	if err != nil {
		return err
	}

	access, cfg, err := e.taskService.GetAria2Access(ctx)
	if err != nil {
		return err
	}

	pollInterval := time.Duration(cfg.Aria2.PollIntervalSeconds) * time.Second
	if pollInterval <= 0 {
		pollInterval = 3 * time.Second
	}

	switch task.Status {
	case model.TaskStatusQueued:
		task, err = e.taskService.PrepareTaskDownload(ctx, task.ID)
		if err != nil {
			return err
		}

		gid, addErr := e.addDownloadWithRefresh(ctx, access, task)
		if addErr != nil {
			return addErr
		}

		task, err = e.taskService.AttachAria2GID(ctx, task.ID, gid)
		if err != nil {
			return err
		}
	case model.TaskStatusDownloading:
		if strings.TrimSpace(task.Download.Aria2GID) == "" {
			if ok, size := hasReusableLocalFile(task); ok {
				_, err := e.taskService.MarkDownloadCompleted(ctx, task.ID, client.Aria2Status{
					Status:          "complete",
					TotalLength:     size,
					CompletedLength: size,
					Files:           []client.Aria2File{{Path: task.Download.LocalPath}},
				})
				return err
			}

			_, _ = e.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "recovery", "aria2_gid_not_found", "aria2 gid missing while resuming download")
			return fmt.Errorf("task %s aria2 gid missing while resuming", task.ID)
		}
	default:
		return nil
	}

	for {
		status, err := e.aria2Client.TellStatus(ctx, access, task.Download.Aria2GID)
		if err != nil {
			latestTask, latestErr := e.taskService.GetTask(ctx, task.ID)
			if latestErr == nil && latestTask.Status == model.TaskStatusCanceled {
				return nil
			}

			if isAria2NotFoundError(err) {
				if ok, size := hasReusableLocalFile(task); ok {
					_, completeErr := e.taskService.MarkDownloadCompleted(ctx, task.ID, client.Aria2Status{
						Status:          "complete",
						TotalLength:     size,
						CompletedLength: size,
						Files:           []client.Aria2File{{Path: task.Download.LocalPath}},
					})
					return completeErr
				}

				_, _ = e.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "download", "aria2_gid_not_found", "aria2 tellStatus failed: "+err.Error())
				return err
			}

			_, _ = e.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "download", "aria2_rpc_error", "aria2 tellStatus failed: "+err.Error())
			return err
		}

		if _, err := e.taskService.SyncDownloadStatus(ctx, task.ID, status); err != nil {
			return err
		}
		if e.eventBus != nil {
			e.eventBus.Publish(eventbus.TaskEvent{
				TaskID: task.ID, Status: "downloading",
				DlProg: task.Download.Progress, DlSpeed: task.Download.Speed,
				DlDone: task.Download.CompletedBytes, DlTotal: task.Download.TotalBytes,
			})
		}

		switch status.Status {
		case "active", "waiting", "paused":
		case "complete":
			_, err = e.taskService.MarkDownloadCompleted(ctx, task.ID, status)
			return err
		case "error", "removed":
			latestTask, latestErr := e.taskService.GetTask(ctx, task.ID)
			if latestErr == nil && latestTask.Status == model.TaskStatusCanceled {
				return nil
			}

			message := strings.TrimSpace(status.ErrorMessage)
			if message == "" {
				message = "aria2 returned status " + status.Status
			}
			code := "download_failed"
			if isRawURLExpiredMessage(message) {
				code = "raw_url_expired"
			}
			_, err = e.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "download", code, message)
			return err
		default:
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func (e *DownloadExecutor) RecoverTask(ctx context.Context, task model.Task) (bool, error) {
	switch task.Status {
	case model.TaskStatusDownloading:
		if strings.TrimSpace(task.Download.Aria2GID) == "" {
			if ok, size := hasReusableLocalFile(task); ok {
				_, err := e.taskService.MarkDownloadCompleted(ctx, task.ID, client.Aria2Status{
					Status:          "complete",
					TotalLength:     size,
					CompletedLength: size,
					Files:           []client.Aria2File{{Path: task.Download.LocalPath}},
				})
				return false, err
			}
			_, err := e.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "recovery", "aria2_gid_not_found", "aria2 gid missing during recovery")
			return false, err
		}

		access, _, err := e.taskService.GetAria2Access(ctx)
		if err != nil {
			return false, err
		}

		status, err := e.aria2Client.TellStatus(ctx, access, task.Download.Aria2GID)
		if err != nil {
			if isAria2NotFoundError(err) {
				if ok, size := hasReusableLocalFile(task); ok {
					_, completeErr := e.taskService.MarkDownloadCompleted(ctx, task.ID, client.Aria2Status{
						Status:          "complete",
						TotalLength:     size,
						CompletedLength: size,
						Files:           []client.Aria2File{{Path: task.Download.LocalPath}},
					})
					return false, completeErr
				}
				_, markErr := e.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "recovery", "aria2_gid_not_found", "aria2 status unavailable during recovery: "+err.Error())
				return false, markErr
			}

			_, markErr := e.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "recovery", "aria2_rpc_error", "aria2 status unavailable during recovery: "+err.Error())
			if markErr != nil {
				return false, markErr
			}
			return false, nil
		}

		switch status.Status {
		case "active", "waiting", "paused":
			_, err = e.taskService.MarkTaskRecovered(ctx, task.ID, status)
			return true, err
		case "complete":
			_, err = e.taskService.MarkDownloadCompleted(ctx, task.ID, status)
			return false, err
		case "error", "removed":
			message := strings.TrimSpace(status.ErrorMessage)
			if message == "" {
				message = "aria2 returned status " + status.Status + " during recovery"
			}
			code := "download_failed"
			if isRawURLExpiredMessage(message) {
				code = "raw_url_expired"
			}
			_, err = e.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "recovery", code, message)
			return false, err
		default:
			_, err = e.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "recovery", "aria2_rpc_error", "unexpected aria2 status during recovery: "+status.Status)
			return false, err
		}
	case model.TaskStatusDownloadCompleted:
		_, err := e.taskService.RecoverCompletedDownload(ctx, task.ID)
		return false, err
	default:
		return false, nil
	}
}

func (e *DownloadExecutor) addDownloadWithRefresh(ctx context.Context, access client.Aria2Access, task model.Task) (string, error) {
	downloadURL := task.Source.RawURL
	// For OpenList sources, route through proxy to handle 302-unfriendly backends (Quark etc)
	if task.Source.Type == "openlist" && task.Source.Path != "" {
		// Use container service name or env var so aria2 (in another container) can reach us
		proxyHost := os.Getenv("EMOSUP_PROXY_HOST")
		if proxyHost == "" {
			proxyHost = "host.docker.internal"
		}
		downloadURL = fmt.Sprintf("http://%s:8080/api/proxy/download?path=%s", proxyHost, url.QueryEscape(task.Source.Path))
	}

	gid, err := e.aria2Client.AddURI(ctx, access, downloadURL, client.Aria2AddURIOptions{
		Dir:              task.Download.SaveDir,
		Out:              filepath.Base(task.Download.LocalPath),
		ContinueDownload: true,
		UserAgent:        "Mozilla/5.0 emosup/phase6",
	})
	if err == nil {
		return gid, nil
	}

	if !isRawURLExpiredMessage(err.Error()) {
		_, _ = e.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "download", "aria2_rpc_error", "aria2 addUri failed: "+err.Error())
		return "", err
	}

	refreshedTask, refreshed, refreshErr := e.taskService.RefreshTaskRawURL(ctx, task.ID)
	if refreshErr != nil || !refreshed {
		_, _ = e.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "download", "raw_url_expired", "aria2 addUri failed after raw_url refresh attempt: "+err.Error())
		if refreshErr != nil {
			return "", refreshErr
		}
		return "", err
	}

	retryURL := refreshedTask.Source.RawURL
	if refreshedTask.Source.Type == "openlist" && refreshedTask.Source.Path != "" {
		proxyHost := os.Getenv("EMOSUP_PROXY_HOST")
		if proxyHost == "" {
			proxyHost = "host.docker.internal"
		}
		retryURL = fmt.Sprintf("http://%s:8080/api/proxy/download?path=%s", proxyHost, url.QueryEscape(refreshedTask.Source.Path))
	}
	gid, retryErr := e.aria2Client.AddURI(ctx, access, retryURL, client.Aria2AddURIOptions{
		Dir:              refreshedTask.Download.SaveDir,
		Out:              filepath.Base(refreshedTask.Download.LocalPath),
		ContinueDownload: true,
		UserAgent:        "Mozilla/5.0 emosup/phase6",
	})
	if retryErr != nil {
		_, _ = e.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "download", "raw_url_expired", "aria2 addUri failed after raw_url refresh: "+retryErr.Error())
		return "", retryErr
	}

	return gid, nil
}
