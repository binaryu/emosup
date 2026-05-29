package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"emosup/backend/internal/client"
	"emosup/backend/internal/eventbus"
	"emosup/backend/internal/model"
)

var (
	downloadHostPath string
	downloadContainerPath string
)

func initDownloadPaths() {
	downloadHostPath = os.Getenv("EMOSUP_DOWNLOADS_DIR")
	downloadContainerPath = os.Getenv("EMOSUP_LOCAL_ROOT")
	if downloadContainerPath == "" {
		downloadContainerPath = "/app/backend/data/downloads"
	}
}

// toContainerPath converts a host download path to the container's view
func toContainerPath(hostPath string) string {
	if downloadHostPath == "" || downloadContainerPath == "" || downloadHostPath == downloadContainerPath {
		return hostPath
	}
	if strings.HasPrefix(hostPath, downloadHostPath) {
		return downloadContainerPath + strings.TrimPrefix(hostPath, downloadHostPath)
	}
	return hostPath
}

type DownloadExecutor struct {
	taskService    *TaskService
	aria2Client    client.Aria2Client
	openListClient client.OpenListClient
	eventBus       *eventbus.Bus
}

func NewDownloadExecutor(taskService *TaskService, aria2Client client.Aria2Client, openListClient client.OpenListClient, eventBus *eventbus.Bus) *DownloadExecutor {
	initDownloadPaths()
	return &DownloadExecutor{
		taskService:    taskService,
		aria2Client:    aria2Client,
		openListClient: openListClient,
		eventBus:       eventBus,
	}
}

func getFreeDiskSpace(dir string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return 0, err
	}
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}

func (e *DownloadExecutor) Execute(ctx context.Context, taskID string) error {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Hour)
	defer cancel()

	task, err := e.taskService.GetTask(ctx, taskID)
	if err != nil {
		return err
	}

	// All OpenList sources download directly through emosup (proper auth headers, no aria2 needed)
	if task.Source.Type == "openlist" {
		return e.downloadDirect(ctx, task)
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
	gid, err := e.aria2Client.AddURI(ctx, access, task.Source.RawURL, client.Aria2AddURIOptions{
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
	gid, retryErr := e.aria2Client.AddURI(ctx, access, retryURL, client.Aria2AddURIOptions{
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

func (e *DownloadExecutor) needsDirectDownload(ctx context.Context, task model.Task) bool {
	path := strings.TrimPrefix(task.Source.Path, "/")
	firstDir := strings.SplitN(path, "/", 2)[0]
	if firstDir == "" {
		return false
	}

	cfg, err := e.taskService.LoadConfig(ctx)
	if err != nil {
		log.Printf("needsDirectDownload: failed to load config: %v", err)
		return false
	}

	backends := strings.TrimSpace(cfg.Worker.ProxyBackends)
	if backends == "" {
		backends = "quark,夸克" // default for existing configs
	}

	for _, name := range strings.Split(backends, ",") {
		if strings.EqualFold(strings.TrimSpace(name), firstDir) {
			log.Printf("needsDirectDownload: matched %s", firstDir)
			return true
		}
	}
	log.Printf("needsDirectDownload: no match for %s (backends=%s)", firstDir, backends)
	return false
}

func (e *DownloadExecutor) downloadDirect(ctx context.Context, task model.Task) error {
	// Prepare download
	task, err := e.taskService.PrepareTaskDownload(ctx, task.ID)
	if err != nil {
		return err
	}

	// Check disk space: require at least file size + 500MB buffer
	freeBytes, _ := getFreeDiskSpace(task.Download.SaveDir)
	if task.Download.TotalBytes > 0 && freeBytes > 0 && freeBytes < task.Download.TotalBytes+500*1024*1024 {
		_, _ = e.taskService.MarkDownloadFailed(ctx, task.ID,
			fmt.Sprintf("磁盘空间不足: 需要 %.1fGB, 可用 %.1fGB",
				float64(task.Download.TotalBytes)/1073741824, float64(freeBytes)/1073741824))
		return fmt.Errorf("insufficient disk space")
	}

	// Get OpenList access
	cfg, err := e.taskService.LoadConfig(ctx)
	if err != nil {
		return err
	}
	access := client.OpenListAccess{
		BaseURL:  cfg.OpenList.BaseURL,
		Username: cfg.OpenList.Username,
		Password: cfg.OpenList.Password,
		Token:    cfg.OpenList.Token,
	}

	// Login if needed
	if access.Token == "" && access.Username != "" {
		if token, loginErr := e.openListClient.Login(ctx, access); loginErr == nil {
			access.Token = token
		}
	}

	// Get the actual download URL from OpenList
	rawURL, err := e.openListClient.GetRawLink(ctx, access, task.Source.Path)
	if err != nil {
		_, _ = e.taskService.MarkDownloadFailed(ctx, task.ID, "failed to get download link: "+err.Error())
		return err
	}
	rawURL = client.ResolveMaybeRelativeURL(cfg.OpenList.BaseURL, rawURL)

	// Stream download to local file
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", strings.TrimRight(cfg.OpenList.BaseURL, "/")+"/")
	if access.Token != "" {
		req.Header.Set("Authorization", access.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		_, _ = e.taskService.MarkDownloadFailed(ctx, task.ID, "download request failed: "+err.Error())
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		_, _ = e.taskService.MarkDownloadFailed(ctx, task.ID, fmt.Sprintf("upstream returned %d", resp.StatusCode))
		return fmt.Errorf("upstream returned %d", resp.StatusCode)
	}

	localPath := toContainerPath(task.Download.LocalPath)
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}

	file, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	totalBytes := resp.ContentLength
	var doneBytes int64
	buf := make([]byte, 32*1024)

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			doneBytes += int64(n)
			progress := float64(0)
			if totalBytes > 0 {
				progress = float64(doneBytes) * 100 / float64(totalBytes)
			}
			if _, syncErr := e.taskService.SyncDownloadStatus(ctx, task.ID, client.Aria2Status{
				Status:          "active",
				TotalLength:     totalBytes,
				CompletedLength: doneBytes,
			}); syncErr == nil && e.eventBus != nil {
				e.eventBus.Publish(eventbus.TaskEvent{
					TaskID: task.ID, Status: "downloading",
					DlProg: progress, DlDone: doneBytes, DlTotal: totalBytes,
				})
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_, _ = e.taskService.MarkDownloadFailed(ctx, task.ID, "download error: "+readErr.Error())
			return readErr
		}
	}

	info, err := os.Stat(localPath)
	if err != nil || info.Size() <= 0 {
		_, _ = e.taskService.MarkDownloadFailed(ctx, task.ID, "downloaded file is empty or missing")
		return fmt.Errorf("downloaded file is empty")
	}

	_, err = e.taskService.MarkDownloadCompleted(ctx, task.ID, client.Aria2Status{
		Status:          "complete",
		TotalLength:     info.Size(),
		CompletedLength: info.Size(),
		Files:           []client.Aria2File{{Path: localPath}},
	})
	return err
}
