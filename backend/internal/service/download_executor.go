package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"emosup/backend/internal/client"
	"emosup/backend/internal/eventbus"
	"emosup/backend/internal/model"
)

var (
	downloadHostPath      string
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%dB", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(n)/1024)
	}
	if n < 1024*1024*1024 {
		return fmt.Sprintf("%.1fMB", float64(n)/(1024*1024))
	}
	return fmt.Sprintf("%.2fGB", float64(n)/(1024*1024*1024))
}

func responseSnippet(body io.Reader, limit int64) string {
	if body == nil || limit <= 0 {
		return ""
	}
	data, _ := io.ReadAll(io.LimitReader(body, limit))
	return strings.TrimSpace(strings.ReplaceAll(string(data), "\n", " "))
}

func (e *DownloadExecutor) taskLog(ctx context.Context, taskID, level, message string) {
	if e == nil || e.taskService == nil {
		return
	}
	if err := e.taskService.AddTaskLog(ctx, taskID, level, message); err != nil {
		log.Printf("append task log failed: task=%s level=%s err=%v", taskID, level, err)
	}
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
			// Direct download interrupted — re-queue to restart
			_, err := e.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "recovery", "download_interrupted", "direct download interrupted during recovery; retry to restart")
			// Also set status back to queued so scheduler can re-pick it
			if _, retryErr := e.taskService.RetryTask(ctx, task.ID); retryErr == nil {
				log.Printf("recovery re-queued direct-download task: %s", task.ID)
				return true, nil
			} else {
				log.Printf("recovery failed to re-queue task %s: %v", task.ID, retryErr)
			}
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
	log.Printf("[download] start: task=%s path=%s", task.ID, task.Source.Path)
	e.taskLog(ctx, task.ID, "info", "direct download started: "+task.Source.Path)

	// Prepare download
	task, err := e.taskService.PrepareTaskDownload(ctx, task.ID)
	if err != nil {
		e.taskLog(ctx, task.ID, "error", "prepare direct download failed: "+err.Error())
		return err
	}

	// Check disk space
	freeBytes, diskErr := getFreeDiskSpace(task.Download.SaveDir)
	if diskErr != nil {
		log.Printf("[download] disk check failed: task=%s dir=%s err=%v", task.ID, task.Download.SaveDir, diskErr)
		e.taskLog(ctx, task.ID, "warn", "disk check failed: "+diskErr.Error())
	}
	if task.Download.TotalBytes > 0 && freeBytes > 0 && freeBytes < task.Download.TotalBytes+500*1024*1024 {
		message := fmt.Sprintf("磁盘不足: 需%.1fG 可用%.1fG", float64(task.Download.TotalBytes)/1e9, float64(freeBytes)/1e9)
		_, _ = e.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "download", "insufficient_disk_space", message)
		return fmt.Errorf("insufficient disk space")
	}
	log.Printf("[download] disk ok: task=%s free=%.1fG", task.ID, float64(freeBytes)/1e9)

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
	if e.openListClient != nil && access.Token == "" && access.Username != "" {
		if token, loginErr := e.openListClient.Login(ctx, access); loginErr == nil {
			access.Token = token
			log.Printf("[download] login ok: task=%s", task.ID)
		} else {
			log.Printf("[download] login failed: task=%s err=%v", task.ID, loginErr)
			e.taskLog(ctx, task.ID, "warn", "OpenList login failed, continue with existing credentials: "+loginErr.Error())
		}
	}

	// Get the actual CDN download URL from OpenList. Tests and imported tasks may only have RawURL.
	rawURL := strings.TrimSpace(task.Source.RawURL)
	if e.openListClient != nil {
		refreshedURL, linkErr := e.openListClient.GetRawLink(ctx, access, task.Source.Path)
		if linkErr != nil {
			log.Printf("[download] get link failed: task=%s err=%v", task.ID, linkErr)
			if rawURL == "" {
				_, _ = e.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "download", "openlist_raw_link_failed", "获取下载链接失败: "+linkErr.Error())
				return linkErr
			}
			e.taskLog(ctx, task.ID, "warn", "refresh download URL failed, fallback to task raw_url: "+linkErr.Error())
		} else {
			rawURL = refreshedURL
		}
	}
	if rawURL == "" {
		err := errors.New("download raw_url is empty")
		_, _ = e.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "download", "raw_url_empty", err.Error())
		return err
	}
	downloadURL := client.ResolveMaybeRelativeURL(cfg.OpenList.BaseURL, rawURL)
	log.Printf("[download] cdn url acquired: task=%s", task.ID)
	e.taskLog(ctx, task.ID, "info", "download URL acquired from OpenList")

	localPath := toContainerPath(task.Download.LocalPath)
	return e.downloadWithResume(ctx, task, access, cfg, downloadURL, localPath)
}

func (e *DownloadExecutor) downloadWithResume(ctx context.Context, task model.Task, access client.OpenListAccess, cfg model.AppConfig, downloadURL, localPath string) error {
	maxRetries := 3
	var lastErr error
	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			log.Printf("[download] retry %d/%d: task=%s", retry+1, maxRetries, task.ID)
			e.taskLog(ctx, task.ID, "warn", fmt.Sprintf("download retry %d/%d after error: %v", retry+1, maxRetries, lastErr))
			time.Sleep(time.Duration(retry*retry) * time.Second) // 1s, 4s backoff
		}

		err := e.downloadOnce(ctx, task, access, cfg, downloadURL, localPath)
		if err == nil {
			return nil
		}
		lastErr = err
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			e.taskLog(ctx, task.ID, "error", "download aborted: "+err.Error())
			_, _ = e.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "download", "download_aborted", err.Error())
			return err
		}
		log.Printf("[download] failed: task=%s retry=%d err=%v", task.ID, retry+1, err)
		e.taskLog(ctx, task.ID, "warn", fmt.Sprintf("download attempt %d/%d failed: %v", retry+1, maxRetries, err))

		// Proxy URL doesn't expire - just retry with same URL
		log.Printf("[download] retrying with same proxy url: task=%s", task.ID)
	}
	message := fmt.Sprintf("下载失败：已重试%d次，最后错误：%v", maxRetries, lastErr)
	_, _ = e.taskService.MarkDownloadFailedWithDetails(ctx, task.ID, "download", "download_http_error", message)
	return fmt.Errorf("download failed after %d retries: %w", maxRetries, lastErr)
}

var downloadHTTPClient = &http.Client{
	Timeout: 0,
	Transport: &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    90 * time.Second,
		DisableCompression: true,
		WriteBufferSize:    256 * 1024,
		ReadBufferSize:     256 * 1024,
		ForceAttemptHTTP2:  true,
	},
}

func (e *DownloadExecutor) downloadOnce(ctx context.Context, task model.Task, access client.OpenListAccess, cfg model.AppConfig, rawURL, localPath string) error {
	threads := cfg.Worker.DownloadThreads
	if threads <= 1 {
		threads = 1
	}
	if info, err := os.Stat(localPath); err == nil && info.Size() > 0 && threads > 1 {
		log.Printf("[download] partial file detected, using single-thread resume: task=%s size=%s", task.ID, formatBytes(info.Size()))
		e.taskLog(ctx, task.ID, "info", "partial local file detected, switch to single-thread resume")
		threads = 1
	}
	if threads == 1 {
		return e.downloadSingle(ctx, task, access, cfg, rawURL, localPath)
	}
	return e.downloadMulti(ctx, task, access, cfg, rawURL, localPath, threads)
}

func (e *DownloadExecutor) downloadSingle(ctx context.Context, task model.Task, access client.OpenListAccess, cfg model.AppConfig, rawURL, localPath string) error {
	// Check for partial file for resume
	var offset int64
	if info, err := os.Stat(localPath); err == nil && info.Size() > 0 {
		offset = info.Size()
		if task.Source.FileSize > 0 && offset >= task.Source.FileSize {
			log.Printf("[download] existing file already complete: task=%s size=%s", task.ID, formatBytes(offset))
			e.taskLog(ctx, task.ID, "info", "local file already complete, skip download")
			_, completeErr := e.taskService.MarkDownloadCompleted(ctx, task.ID, client.Aria2Status{
				Status:          "complete",
				TotalLength:     offset,
				CompletedLength: offset,
				Files:           []client.Aria2File{{Path: localPath}},
			})
			return completeErr
		}
		log.Printf("[download] resume: task=%s offset=%d", task.ID, offset)
		e.taskLog(ctx, task.ID, "info", fmt.Sprintf("resume download from %s", formatBytes(offset)))
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("create download dir %s: %w", filepath.Dir(localPath), err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", strings.TrimRight(cfg.OpenList.BaseURL, "/")+"/")
	if access.Token != "" {
		req.Header.Set("Authorization", access.Token)
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	resp, err := downloadHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		message := fmt.Sprintf("upstream returned 416 range not satisfiable; offset=%d expected=%d body=%q", offset, task.Source.FileSize, responseSnippet(resp.Body, 512))
		return errors.New(message)
	}
	if resp.StatusCode >= 400 {
		message := fmt.Sprintf("upstream returned %d %s; content_type=%q content_length=%d body=%q", resp.StatusCode, resp.Status, resp.Header.Get("Content-Type"), resp.ContentLength, responseSnippet(resp.Body, 1024))
		return errors.New(message)
	}

	// If server doesn't support Range, start over.
	var file *os.File
	if offset > 0 && resp.StatusCode == http.StatusPartialContent {
		file, err = os.OpenFile(localPath, os.O_APPEND|os.O_WRONLY, 0644)
	} else {
		if offset > 0 {
			log.Printf("[download] range ignored, restart from zero: task=%s status=%d", task.ID, resp.StatusCode)
			e.taskLog(ctx, task.ID, "warn", fmt.Sprintf("server ignored Range request (status=%d), restart download from 0", resp.StatusCode))
		}
		offset = 0
		file, err = os.Create(localPath)
	}
	if err != nil {
		return fmt.Errorf("open local file %s: %w", localPath, err)
	}
	defer file.Close()

	totalBytes := resp.ContentLength + offset
	if totalBytes < offset {
		totalBytes = task.Source.FileSize
	}
	var doneBytes int64 = offset
	buf := make([]byte, 1024*1024)
	startTime := time.Now()
	lastLog := time.Now()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("write local file %s: %w", localPath, writeErr)
			}
			doneBytes += int64(n)

			now := time.Now()
			if now.Sub(lastLog) > 3*time.Second || doneBytes >= totalBytes {
				lastLog = now
				progress := float64(0)
				elapsed := now.Sub(startTime)
				speed := int64(0)
				if elapsed > 0 && doneBytes > offset {
					speed = int64(float64(doneBytes-offset) / elapsed.Seconds())
				}
				if totalBytes > 0 {
					progress = float64(doneBytes) * 100 / float64(totalBytes)
				}
				log.Printf("[download] progress: task=%s %.1f%% %s/%s %s/s",
					task.ID, progress, formatBytes(doneBytes), formatBytes(totalBytes), formatBytes(speed))
				if _, syncErr := e.taskService.SyncDownloadStatus(ctx, task.ID, client.Aria2Status{
					Status:          "active",
					TotalLength:     totalBytes,
					CompletedLength: doneBytes,
					DownloadSpeed:   speed,
				}); syncErr == nil && e.eventBus != nil {
					e.eventBus.Publish(eventbus.TaskEvent{
						TaskID: task.ID, Status: "downloading",
						DlProg: progress, DlSpeed: speed, DlDone: doneBytes, DlTotal: totalBytes,
					})
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read response body: %w", readErr)
		}
	}

	// Validate
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("stat downloaded file %s: %w", localPath, err)
	}
	if info.Size() <= 0 {
		return fmt.Errorf("file empty after download: %s", localPath)
	}
	if task.Source.FileSize > 0 && info.Size() < task.Source.FileSize {
		return fmt.Errorf("downloaded file smaller than expected: expected=%s got=%s", formatBytes(task.Source.FileSize), formatBytes(info.Size()))
	}

	log.Printf("[download] complete: task=%s size=%s", task.ID, formatBytes(info.Size()))
	e.taskLog(ctx, task.ID, "info", "download completed: "+formatBytes(info.Size()))
	_, err = e.taskService.MarkDownloadCompleted(ctx, task.ID, client.Aria2Status{
		Status:          "complete",
		TotalLength:     info.Size(),
		CompletedLength: info.Size(),
		Files:           []client.Aria2File{{Path: localPath}},
	})
	return err
}

func (e *DownloadExecutor) downloadMulti(ctx context.Context, task model.Task, access client.OpenListAccess, cfg model.AppConfig, rawURL, localPath string, threads int) error {
	log.Printf("[download] multi-thread start: task=%s threads=%d", task.ID, threads)
	e.taskLog(ctx, task.ID, "info", fmt.Sprintf("multi-thread download started: threads=%d", threads))

	// Check file size with timeout
	headCtx, headCancel := context.WithTimeout(ctx, 10*time.Second)
	defer headCancel()
	headReq, _ := http.NewRequestWithContext(headCtx, "HEAD", rawURL, nil)
	headReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	if access.Token != "" {
		headReq.Header.Set("Authorization", access.Token)
	}
	resp, err := downloadHTTPClient.Do(headReq)
	if err != nil {
		log.Printf("[download] HEAD failed, falling back to single-thread: task=%s err=%v", task.ID, err)
		e.taskLog(ctx, task.ID, "warn", "HEAD request failed, fallback to single-thread: "+err.Error())
		return e.downloadSingle(ctx, task, access, cfg, rawURL, localPath)
	}
	totalSize := resp.ContentLength
	statusCode := resp.StatusCode
	contentType := resp.Header.Get("Content-Type")
	resp.Body.Close()

	if statusCode >= 400 {
		return fmt.Errorf("HEAD upstream returned %d; content_type=%q", statusCode, contentType)
	}
	if totalSize <= 0 || threads <= 1 {
		log.Printf("[download] size unknown, falling back to single-thread: task=%s size=%d", task.ID, totalSize)
		e.taskLog(ctx, task.ID, "warn", "file size unknown from HEAD, fallback to single-thread")
		return e.downloadSingle(ctx, task, access, cfg, rawURL, localPath)
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("create download dir %s: %w", filepath.Dir(localPath), err)
	}

	partPath := localPath + ".partmulti"
	_ = os.Remove(partPath)
	defer func() { _ = os.Remove(partPath) }()

	file, err := os.OpenFile(partPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open temp file %s: %w", partPath, err)
	}
	defer file.Close()
	if err := file.Truncate(totalSize); err != nil {
		return fmt.Errorf("preallocate temp file %s: %w", partPath, err)
	}

	downloadCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	segmentSize := totalSize / int64(threads)
	type segmentResult struct {
		index int
		err   error
	}
	results := make(chan segmentResult, threads)
	var wg sync.WaitGroup
	var progressMu sync.Mutex
	var totalDone int64

	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			start := int64(idx) * segmentSize
			end := start + segmentSize - 1
			if idx == threads-1 {
				end = totalSize - 1
			}
			results <- segmentResult{index: idx, err: e.downloadSegment(downloadCtx, file, access, cfg, rawURL, start, end, idx, func(n int64) {
				progressMu.Lock()
				totalDone += n
				progressMu.Unlock()
			})}
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	startTime := time.Now()
	lastLog := time.Now()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	completedSegments := 0
	var firstErr error

	reportProgress := func(force bool) {
		now := time.Now()
		if !force && now.Sub(lastLog) <= 3*time.Second {
			return
		}
		lastLog = now
		progressMu.Lock()
		done := totalDone
		progressMu.Unlock()
		progress := float64(done) * 100 / float64(totalSize)
		speed := int64(0)
		if elapsed := now.Sub(startTime); elapsed > 0 {
			speed = int64(float64(done) / elapsed.Seconds())
		}
		log.Printf("[download] progress: task=%s %.1f%% %s/%s %s/s (%d threads)",
			task.ID, progress, formatBytes(done), formatBytes(totalSize), formatBytes(speed), threads)
		if _, syncErr := e.taskService.SyncDownloadStatus(ctx, task.ID, client.Aria2Status{
			Status: "active", TotalLength: totalSize, CompletedLength: done, DownloadSpeed: speed,
		}); syncErr == nil && e.eventBus != nil {
			e.eventBus.Publish(eventbus.TaskEvent{
				TaskID: task.ID, Status: "downloading",
				DlProg: progress, DlSpeed: speed, DlDone: done, DlTotal: totalSize,
			})
		}
	}

	for completedSegments < threads {
		select {
		case result, ok := <-results:
			if !ok {
				completedSegments = threads
				break
			}
			completedSegments++
			if result.err != nil && firstErr == nil {
				firstErr = result.err
				cancel()
				log.Printf("[download] segment %d failed: %v", result.index, result.err)
				e.taskLog(ctx, task.ID, "warn", fmt.Sprintf("multi-thread segment failed, fallback to single-thread: %v", result.err))
			}
			reportProgress(true)
		case <-ticker.C:
			reportProgress(false)
		case <-ctx.Done():
			cancel()
			return ctx.Err()
		}
	}

	if firstErr != nil {
		_ = file.Close()
		_ = os.Remove(partPath)
		return e.downloadSingle(ctx, task, access, cfg, rawURL, localPath)
	}

	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync temp file %s: %w", partPath, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temp file %s: %w", partPath, err)
	}

	info, err := os.Stat(partPath)
	if err != nil {
		return fmt.Errorf("stat temp file %s: %w", partPath, err)
	}
	if info.Size() != totalSize {
		return fmt.Errorf("multi-thread file size mismatch: expected=%s got=%s", formatBytes(totalSize), formatBytes(info.Size()))
	}
	_ = os.Remove(localPath)
	if err := os.Rename(partPath, localPath); err != nil {
		return fmt.Errorf("move temp file to final path: %w", err)
	}

	log.Printf("[download] complete: task=%s size=%s", task.ID, formatBytes(info.Size()))
	e.taskLog(ctx, task.ID, "info", "download completed: "+formatBytes(info.Size()))
	_, err = e.taskService.MarkDownloadCompleted(ctx, task.ID, client.Aria2Status{
		Status: "complete", TotalLength: info.Size(), CompletedLength: info.Size(),
		Files: []client.Aria2File{{Path: localPath}},
	})
	return err
}

func (e *DownloadExecutor) downloadSegment(ctx context.Context, file *os.File, access client.OpenListAccess, cfg model.AppConfig, rawURL string, start, end int64, index int, onProgress func(int64)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", strings.TrimRight(cfg.OpenList.BaseURL, "/")+"/")
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	if access.Token != "" {
		req.Header.Set("Authorization", access.Token)
	}

	resp, err := downloadHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("segment %d request failed: %w", index, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("segment %d returned %d %s; body=%q", index, resp.StatusCode, resp.Status, responseSnippet(resp.Body, 512))
	}

	buf := make([]byte, 1024*1024)
	writeOffset := start
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if writeOffset+int64(n)-1 > end {
				return fmt.Errorf("segment %d exceeded expected range", index)
			}
			if _, writeErr := file.WriteAt(buf[:n], writeOffset); writeErr != nil {
				return fmt.Errorf("segment %d write failed: %w", index, writeErr)
			}
			writeOffset += int64(n)
			if onProgress != nil {
				onProgress(int64(n))
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("segment %d read failed: %w", index, readErr)
		}
	}

	expected := end - start + 1
	if writeOffset-start != expected {
		return fmt.Errorf("segment %d size mismatch: expected=%d got=%d", index, expected, writeOffset-start)
	}
	return nil
}
