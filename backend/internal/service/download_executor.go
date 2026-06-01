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

	// Prepare download
	task, err := e.taskService.PrepareTaskDownload(ctx, task.ID)
	if err != nil {
		return err
	}

	// Check disk space
	freeBytes, _ := getFreeDiskSpace(task.Download.SaveDir)
	if task.Download.TotalBytes > 0 && freeBytes > 0 && freeBytes < task.Download.TotalBytes+500*1024*1024 {
		_, _ = e.taskService.MarkDownloadFailed(ctx, task.ID,
			fmt.Sprintf("磁盘不足: 需%.1fG 可用%.1fG", float64(task.Download.TotalBytes)/1e9, float64(freeBytes)/1e9))
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
	if access.Token == "" && access.Username != "" {
		if token, loginErr := e.openListClient.Login(ctx, access); loginErr == nil {
			access.Token = token
			log.Printf("[download] login ok: task=%s", task.ID)
		} else {
			log.Printf("[download] login failed: task=%s err=%v", task.ID, loginErr)
		}
	}

	// Verify OpenList access works by fetching raw link (for auth check)
	_, err = e.openListClient.GetRawLink(ctx, access, task.Source.Path)
	if err != nil {
		log.Printf("[download] get link failed: task=%s err=%v", task.ID, err)
		_, _ = e.taskService.MarkDownloadFailed(ctx, task.ID, "获取下载链接失败: "+err.Error())
		return err
	}
	// Build proxy URL: download through OpenList directly (handles auth internally)
	// This is more reliable than raw CDN URLs which may have short TTLs
	proxyURL := strings.TrimRight(cfg.OpenList.BaseURL, "/") + "/d" + task.Source.Path
	log.Printf("[download] using proxy: %s", proxyURL[:minInt(len(proxyURL), 80)])

	// Download with retry using OpenList proxy (handles auth + CDN internally)
	localPath := toContainerPath(task.Download.LocalPath)
	return e.downloadWithResume(ctx, task, access, cfg, proxyURL, localPath)
}

func (e *DownloadExecutor) downloadWithResume(ctx context.Context, task model.Task, access client.OpenListAccess, cfg model.AppConfig, downloadURL, localPath string) error {
	maxRetries := 3
	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			log.Printf("[download] retry %d/%d: task=%s", retry+1, maxRetries, task.ID)
			time.Sleep(time.Duration(retry*retry) * time.Second) // 1s, 4s backoff
		}

		err := e.downloadOnce(ctx, task, access, cfg, downloadURL, localPath)
		if err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		log.Printf("[download] failed: task=%s retry=%d err=%v", task.ID, retry+1, err)

		// Proxy URL doesn't expire - just retry with same URL
		log.Printf("[download] retrying with same proxy url: task=%s", task.ID)
	}
	_, _ = e.taskService.MarkDownloadFailed(ctx, task.ID, "下载失败：已重试3次")
	return fmt.Errorf("download failed after %d retries", maxRetries)
}

var downloadHTTPClient = &http.Client{
	Timeout: 0,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
		WriteBufferSize:     256 * 1024,
		ReadBufferSize:      256 * 1024,
		ForceAttemptHTTP2:   true,
	},
}

func (e *DownloadExecutor) downloadOnce(ctx context.Context, task model.Task, access client.OpenListAccess, cfg model.AppConfig, rawURL, localPath string) error {
	threads := cfg.Worker.DownloadThreads
	if threads <= 1 {
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
		log.Printf("[download] resume: task=%s offset=%d", task.ID, offset)
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
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

	if resp.StatusCode >= 400 && resp.StatusCode != 416 {
		return fmt.Errorf("upstream returned %d", resp.StatusCode)
	}

	// If server doesn't support Range, start over
	var file *os.File
	if offset > 0 && resp.StatusCode == 206 {
		file, err = os.OpenFile(localPath, os.O_APPEND|os.O_WRONLY, 0644)
	} else {
		offset = 0
		file, err = os.Create(localPath)
	}
	if err != nil {
		return err
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
				return writeErr
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
			return fmt.Errorf("read error: %w", readErr)
		}
	}

	// Validate
	info, err := os.Stat(localPath)
	if err != nil || info.Size() <= 0 {
		return fmt.Errorf("file empty after download")
	}
	if task.Source.FileSize > 0 && info.Size() < task.Source.FileSize {
		log.Printf("[download] size mismatch: task=%s expected=%d got=%d", task.ID, task.Source.FileSize, info.Size())
	}

	log.Printf("[download] complete: task=%s size=%s", task.ID, formatBytes(info.Size()))
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
		return e.downloadSingle(ctx, task, access, cfg, rawURL, localPath)
	}
	totalSize := resp.ContentLength
	resp.Body.Close()

	if totalSize <= 0 || threads <= 1 {
		return e.downloadSingle(ctx, task, access, cfg, rawURL, localPath)
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}

	segmentSize := totalSize / int64(threads)
	type segment struct {
		index int
		data  []byte
		start int64
		err   error
	}
	results := make(chan segment, threads)
	var wg sync.WaitGroup

	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			start := int64(idx) * segmentSize
			end := start + segmentSize - 1
			if idx == threads-1 {
				end = totalSize - 1
			}

			req, _ := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
			req.Header.Set("Referer", strings.TrimRight(cfg.OpenList.BaseURL, "/")+"/")
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
			if access.Token != "" {
				req.Header.Set("Authorization", access.Token)
			}

			segResp, err := downloadHTTPClient.Do(req)
			if err != nil {
				results <- segment{idx, nil, start, err}
				return
			}
			defer segResp.Body.Close()

			if segResp.StatusCode != 206 && segResp.StatusCode != 200 {
				results <- segment{idx, nil, start, fmt.Errorf("segment %d returned %d", idx, segResp.StatusCode)}
				return
			}

			data, err := io.ReadAll(segResp.Body)
			results <- segment{idx, data, start, err}
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	file, err := os.OpenFile(localPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	var totalDone int64
	startTime := time.Now()
	lastLog := time.Now()

	for seg := range results {
		if seg.err != nil {
			log.Printf("[download] segment %d failed: %v, falling back to single-thread", seg.index, seg.err)
			return e.downloadSingle(ctx, task, access, cfg, rawURL, localPath)
		}
		if _, err := file.WriteAt(seg.data, seg.start); err != nil {
			return err
		}
		totalDone += int64(len(seg.data))

		now := time.Now()
		if now.Sub(lastLog) > 3*time.Second || totalDone >= totalSize {
			lastLog = now
			progress := float64(totalDone) * 100 / float64(totalSize)
			speed := int64(0)
			if elapsed := now.Sub(startTime); elapsed > 0 {
				speed = int64(float64(totalDone) / elapsed.Seconds())
			}
			log.Printf("[download] progress: task=%s %.1f%% %s/%s %s/s (%d threads)",
				task.ID, progress, formatBytes(totalDone), formatBytes(totalSize), formatBytes(speed), threads)
			if _, syncErr := e.taskService.SyncDownloadStatus(ctx, task.ID, client.Aria2Status{
				Status: "active", TotalLength: totalSize, CompletedLength: totalDone, DownloadSpeed: speed,
			}); syncErr == nil && e.eventBus != nil {
				e.eventBus.Publish(eventbus.TaskEvent{
					TaskID: task.ID, Status: "downloading",
					DlProg: progress, DlSpeed: speed, DlDone: totalDone, DlTotal: totalSize,
				})
			}
		}
	}

	info, _ := os.Stat(localPath)
	log.Printf("[download] complete: task=%s size=%s", task.ID, formatBytes(info.Size()))
	_, err = e.taskService.MarkDownloadCompleted(ctx, task.ID, client.Aria2Status{
		Status: "complete", TotalLength: info.Size(), CompletedLength: info.Size(),
		Files: []client.Aria2File{{Path: localPath}},
	})
	return err
}
