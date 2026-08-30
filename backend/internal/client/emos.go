package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type EmosAccess struct {
	BaseURL string
	Token   string
}

type EmosVideoTree struct {
	VideoType string       `json:"video_type"`
	ItemType  string       `json:"item_type"`
	ItemID    int64        `json:"item_id"`
	TMDBID    int64        `json:"tmdb_id"`
	Title     string       `json:"title"`
	Seasons   []EmosSeason `json:"seasons"`
}

type EmosSeason struct {
	ItemType     string        `json:"item_type"`
	ItemID       int64         `json:"item_id"`
	SeasonTitle  string        `json:"season_title"`
	SeasonNumber int           `json:"season_number"`
	Episodes     []EmosEpisode `json:"episodes"`
}

type EmosEpisode struct {
	ItemType      string `json:"item_type"`
	ItemID        int64  `json:"item_id"`
	EpisodeTitle  string `json:"episode_title"`
	EpisodeNumber int    `json:"episode_number"`
	HasMedia      bool   `json:"has_media"`
}

type EmosVideoBase struct {
	Title string `json:"title"`
}

type EmosUploadTokenRequest struct {
	ResourceType string
	FileType     string
	FileName     string
	FileSize     int64
	FileStorage  string
}

type EmosUploadTokenResult struct {
	Storage          string
	FileID           string
	UploadURL        string
	UploadType       string
	MultipartSizeMin int64
	MultipartSizeMax int64
}

type EmosMultipartPart struct {
	Number    int    `json:"number"`
	UploadURL string `json:"upload_url,omitempty"`
	ETag      string `json:"etag,omitempty"`
}

type EmosSaveVideoRequest struct {
	ItemType string `json:"item_type"`
	ItemID   int64  `json:"item_id"`
	FileID   string `json:"file_id"`
}

type EmosSaveVideoResult struct {
	Count   int    `json:"count"`
	Carrot  int    `json:"carrot"`
	MediaID string `json:"media_id"`
}

type EmosUploadProgress struct {
	UploadedBytes int64
	TotalBytes    int64
	Speed         int64
}

type EmosSaveError struct {
	StatusCode int
	Message    string
}

const (
	uploadRetryMaxAttempts = 5
	uploadRetryBackoff     = 500 * time.Millisecond
	// uploadRequestTimeout bounds a single chunk upload attempt. It used to be
	// 10 minutes, which turned a stalled connection (e.g. mainland VPS → R2)
	// into a silent 50-minute hang across retries before any error surfaced.
	uploadRequestTimeout = 3 * time.Minute
)

// uploadStallTimeout aborts an in-flight chunk upload when no bytes are
// handed to the transport for this long (connection stalled, upstream
// stopped responding). Slow-but-progressing uploads are never affected
// because every read resets the timer. Variable so tests can shrink it.
var uploadStallTimeout = 60 * time.Second
func (e *EmosSaveError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Message) == "" {
		return fmt.Sprintf("emos save request failed: status %d", e.StatusCode)
	}
	return fmt.Sprintf("emos save request failed: %s", e.Message)
}

func (e *EmosSaveError) Retryable() bool {
	return e.RetryableWaiting() || e.RetryableTemporary()
}

func (e *EmosSaveError) RetryableWaiting() bool {
	if e == nil {
		return false
	}
	if e.StatusCode == http.StatusUnprocessableEntity {
		return true
	}

	message := strings.ToLower(strings.TrimSpace(e.Message))
	return strings.Contains(message, "合并中") ||
		strings.Contains(message, "请稍后") ||
		strings.Contains(message, "processing") ||
		strings.Contains(message, "merging")
}

func (e *EmosSaveError) RetryableTemporary() bool {
	if e == nil {
		return false
	}
	if e.StatusCode == http.StatusBadGateway ||
		e.StatusCode == http.StatusServiceUnavailable ||
		e.StatusCode == http.StatusGatewayTimeout {
		return true
	}

	message := strings.ToLower(strings.TrimSpace(e.Message))
	return strings.Contains(message, "temporary") ||
		strings.Contains(message, "timeout") ||
		strings.Contains(message, "gateway")
}

type EmosClient interface {
	GetVideoTree(ctx context.Context, access EmosAccess, tmdbID int64, videoType string) (EmosVideoTree, error)
	GetVideoBase(ctx context.Context, access EmosAccess, itemType string, itemID int64) (EmosVideoBase, error)
	GetUploadToken(ctx context.Context, access EmosAccess, req EmosUploadTokenRequest) (EmosUploadTokenResult, error)
	UploadFile(ctx context.Context, uploadType, uploadURL, filePath string, chunkSize int64, offset int64, onProgress func(EmosUploadProgress) error) error
	UploadMultipartPart(ctx context.Context, part EmosMultipartPart, filePath string, startByte, partSize int64) (string, error)
	// UploadMultipartPartWithProgress is UploadMultipartPart with a live
	// per-part byte counter (incremental bytes read so far, called from the
	// HTTP body reader). progress may be nil to disable reporting.
	UploadMultipartPartWithProgress(ctx context.Context, part EmosMultipartPart, filePath string, startByte, partSize int64, progress func(int64)) (string, error)
	RequestMultipartPresigns(ctx context.Context, access EmosAccess, fileID string, numChunks int) ([]EmosMultipartPart, error)
	CompleteMultipart(ctx context.Context, access EmosAccess, fileID string, parts []EmosMultipartPart) error
	SaveVideo(ctx context.Context, access EmosAccess, req EmosSaveVideoRequest) (EmosSaveVideoResult, error)
}

type HTTPEmosClient struct {
	apiClient    *http.Client
	uploadClient *http.Client
}

func NewHTTPEmosClient() *HTTPEmosClient {
	return &HTTPEmosClient{
		apiClient: &http.Client{Timeout: 60 * time.Second},
		uploadClient: &http.Client{
			Timeout: uploadRequestTimeout,
			Transport: &http.Transport{
				// Fail fast when the storage endpoint is unreachable or the
				// connection is silently dropped (common for mainland VPS →
				// Cloudflare R2): dial/TLS stalls error within seconds instead
				// of blocking the upload for minutes.
				DialContext:           (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
				TLSHandshakeTimeout:   15 * time.Second,
				ResponseHeaderTimeout: 60 * time.Second,
				IdleConnTimeout:       90 * time.Second,
				MaxIdleConns:          32,
				MaxIdleConnsPerHost:   16,
				DisableCompression:    true,
				ForceAttemptHTTP2:     true,
			},
		},
	}
}

func (c *HTTPEmosClient) GetVideoTree(ctx context.Context, access EmosAccess, tmdbID int64, videoType string) (EmosVideoTree, error) {
	if err := validateEmosAccess(access); err != nil {
		return EmosVideoTree{}, err
	}

	endpoint, err := url.Parse(strings.TrimRight(access.BaseURL, "/") + "/api/video/tree")
	if err != nil {
		return EmosVideoTree{}, err
	}

	query := endpoint.Query()
	query.Set("tmdb_id", strconv.FormatInt(tmdbID, 10))
	if strings.TrimSpace(videoType) != "" {
		query.Set("type", videoType)
	}
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return EmosVideoTree{}, err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(access.Token))

	response, err := c.apiClient.Do(request)
	if err != nil {
		return EmosVideoTree{}, err
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		return EmosVideoTree{}, fmt.Errorf("emos tree request failed: %s", response.Status)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return EmosVideoTree{}, err
	}

	var tree EmosVideoTree
	if err := json.Unmarshal(body, &tree); err == nil {
		if tree.ItemID != 0 || tree.TMDBID != 0 || len(tree.Seasons) > 0 || strings.TrimSpace(tree.Title) != "" {
			return tree, nil
		}
	}

	var trees []EmosVideoTree
	if err := json.Unmarshal(body, &trees); err == nil {
		if len(trees) == 0 {
			return EmosVideoTree{}, fmt.Errorf("EMOS 媒体库中未找到该剧集/影视信息 (TMDB ID: %d)，请确认已在 EMOS 中添加该条目", tmdbID)
		}
		if len(trees) == 1 {
			return trees[0], nil
		}
		for _, candidate := range trees {
			if candidate.TMDBID == tmdbID {
				return candidate, nil
			}
		}
		return trees[0], nil
	}

	return EmosVideoTree{}, fmt.Errorf("EMOS 媒体库中未找到该剧集/影视信息 (TMDB ID: %d)，请确认已在 EMOS 中添加该条目", tmdbID)
}

func (c *HTTPEmosClient) GetVideoBase(ctx context.Context, access EmosAccess, itemType string, itemID int64) (EmosVideoBase, error) {
	if err := validateEmosAccess(access); err != nil {
		return EmosVideoBase{}, err
	}

	endpoint, err := url.Parse(strings.TrimRight(access.BaseURL, "/") + "/api/upload/video/base")
	if err != nil {
		return EmosVideoBase{}, err
	}

	query := endpoint.Query()
	query.Set("item_type", itemType)
	query.Set("item_id", strconv.FormatInt(itemID, 10))
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return EmosVideoBase{}, err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(access.Token))

	response, err := c.apiClient.Do(request)
	if err != nil {
		return EmosVideoBase{}, err
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		return EmosVideoBase{}, fmt.Errorf("emos base request failed: %s", response.Status)
	}

	var base EmosVideoBase
	if err := json.NewDecoder(response.Body).Decode(&base); err != nil {
		return EmosVideoBase{}, err
	}

	return base, nil
}

func (c *HTTPEmosClient) GetUploadToken(ctx context.Context, access EmosAccess, req EmosUploadTokenRequest) (EmosUploadTokenResult, error) {
	if err := validateEmosAccess(access); err != nil {
		return EmosUploadTokenResult{}, err
	}

	payload := map[string]any{
		"type":         defaultString(req.ResourceType, "video"),
		"file_type":    defaultString(req.FileType, "application/octet-stream"),
		"file_name":    req.FileName,
		"file_size":    req.FileSize,
		"file_storage": defaultString(req.FileStorage, "default"),
	}

	response, err := c.doAuthorizedJSON(ctx, access, http.MethodPost, "/api/upload/getUploadToken", payload)
	if err != nil {
		return EmosUploadTokenResult{}, err
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return EmosUploadTokenResult{}, fmt.Errorf("emos upload token request failed: %s", strings.TrimSpace(string(body)))
	}

	var raw struct {
		Type      string          `json:"type"`
		Storage   string          `json:"storage"`
		FileID    string          `json:"file_id"`
		UploadURL string          `json:"upload_url"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return EmosUploadTokenResult{}, err
	}

	type tokenData struct {
		UploadURL     string          `json:"upload_url"`
		URL           string          `json:"url"`
		PutURL        string          `json:"put_url"`
		FileID        string          `json:"file_id"`
		MultipartSize json.RawMessage `json:"multipart_size"`
	}
	var data tokenData
	if len(raw.Data) > 0 {
		if err := json.Unmarshal(raw.Data, &data); err != nil {
			var dataURL string
			if stringErr := json.Unmarshal(raw.Data, &dataURL); stringErr == nil {
				data.UploadURL = dataURL
			}
		}
	}

	uploadType := strings.ToLower(strings.TrimSpace(raw.Type))
	if uploadType == "" {
		uploadType = "onedrive"
	}
	fileID := firstNonEmptyString(raw.FileID, data.FileID)
	uploadURL := strings.TrimSpace(raw.UploadURL)
	if uploadURL == "" {
		uploadURL = firstNonEmptyString(
			data.UploadURL,
			data.URL,
			data.PutURL,
		)
	}
	minSize, maxSize := multipartSizeBounds(data.MultipartSize)

	result := EmosUploadTokenResult{
		Storage:          firstNonEmptyString(raw.Storage, req.FileStorage, "default"),
		FileID:           fileID,
		UploadURL:        uploadURL,
		UploadType:       uploadType,
		MultipartSizeMin: minSize,
		MultipartSizeMax: maxSize,
	}
	if result.FileID == "" {
		return EmosUploadTokenResult{}, errors.New("emos upload token response missing file_id")
	}
	if uploadType != "multipart" && result.UploadURL == "" {
		return EmosUploadTokenResult{}, errors.New("emos upload token response missing upload_url")
	}

	return result, nil
}

func (c *HTTPEmosClient) UploadFile(ctx context.Context, uploadType, uploadURL, filePath string, chunkSize int64, offset int64, onProgress func(EmosUploadProgress) error) error {
	uploadType = strings.ToLower(strings.TrimSpace(uploadType))
	if uploadType == "" {
		uploadType = "onedrive"
	}
	if uploadType != "onedrive" && uploadType != "r2" {
		return fmt.Errorf("unsupported upload type %q", uploadType)
	}
	if strings.TrimSpace(uploadURL) == "" {
		return errors.New("upload url is required")
	}
	if strings.TrimSpace(filePath) == "" {
		return errors.New("local file path is required")
	}
	if chunkSize <= 0 {
		return errors.New("upload chunk size must be positive")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() <= 0 {
		return errors.New("local file is empty")
	}

	totalBytes := info.Size()

	var uploadedBytes int64 = offset
	for uploadedBytes < totalBytes {
		if err := ctx.Err(); err != nil {
			return err
		}

		partSize := minInt64(chunkSize, totalBytes-uploadedBytes)
		startByte := uploadedBytes
		endByte := uploadedBytes + partSize - 1

		contentRange := ""
		if uploadType == "onedrive" {
			contentRange = fmt.Sprintf("bytes %d-%d/%d", startByte, endByte, totalBytes)
		}

		startedAt := time.Now()
		_, uploadErr := c.putFileSectionWithRetry(
			ctx,
			uploadType+" upload chunk",
			uploadURL,
			file,
			startByte,
			partSize,
			"application/octet-stream",
			contentRange,
			nil,
			func(statusCode int) bool {
				return acceptedUploadStatus(statusCode, uploadType == "r2")
			},
		)
		if uploadErr != nil {
			return uploadErr
		}

		uploadedBytes += partSize
		elapsed := time.Since(startedAt)
		speed := int64(0)
		if elapsed > 0 {
			speed = int64(float64(partSize) / elapsed.Seconds())
		}

		if onProgress != nil {
			if err := onProgress(EmosUploadProgress{
				UploadedBytes: uploadedBytes,
				TotalBytes:    totalBytes,
				Speed:         speed,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *HTTPEmosClient) UploadMultipartPart(ctx context.Context, part EmosMultipartPart, filePath string, startByte, partSize int64) (string, error) {
	return c.UploadMultipartPartWithProgress(ctx, part, filePath, startByte, partSize, nil)
}

func (c *HTTPEmosClient) UploadMultipartPartWithProgress(ctx context.Context, part EmosMultipartPart, filePath string, startByte, partSize int64, progress func(int64)) (string, error) {
	if part.Number <= 0 {
		return "", errors.New("multipart part number must be positive")
	}
	if strings.TrimSpace(part.UploadURL) == "" {
		return "", errors.New("multipart part upload url is required")
	}
	if strings.TrimSpace(filePath) == "" {
		return "", errors.New("local file path is required")
	}
	if startByte < 0 || partSize <= 0 {
		return "", errors.New("multipart part range is invalid")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	return c.putFileSectionWithRetry(
		ctx,
		fmt.Sprintf("multipart part %d upload", part.Number),
		part.UploadURL,
		file,
		startByte,
		partSize,
		"application/octet-stream",
		"",
		progress,
		func(statusCode int) bool {
			return statusCode == http.StatusOK ||
				statusCode == http.StatusCreated ||
				statusCode == http.StatusNoContent
		},
	)
}

// progressSectionReader wraps a section reader and reports incremental bytes
// read to fn (called from the HTTP transport's body reads).
type progressSectionReader struct {
	r  io.Reader
	fn func(int64)
}

func (p *progressSectionReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 && p.fn != nil {
		p.fn(int64(n))
	}
	return n, err
}

func (c *HTTPEmosClient) putFileSectionWithRetry(
	ctx context.Context,
	operation, uploadURL string,
	file *os.File,
	startByte, partSize int64,
	contentType, contentRange string,
	progress func(int64),
	isSuccess func(int) bool,
) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= uploadRetryMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		// Each attempt gets its own context so a detected stall only aborts
		// this attempt and the retry loop can try again.
		attemptCtx, attemptCancel := context.WithCancel(ctx)

		var lastProgressAt atomic.Int64
		lastProgressAt.Store(time.Now().UnixNano())
		var stalled atomic.Bool
		stopMonitor := make(chan struct{})
		monitorInterval := uploadStallTimeout / 2
		if monitorInterval < 100*time.Millisecond {
			monitorInterval = 100 * time.Millisecond
		}
		if monitorInterval > 5*time.Second {
			monitorInterval = 5 * time.Second
		}
		go func() {
			ticker := time.NewTicker(monitorInterval)
			defer ticker.Stop()
			for {
				select {
				case <-stopMonitor:
					return
				case <-attemptCtx.Done():
					return
				case <-ticker.C:
					if time.Since(time.Unix(0, lastProgressAt.Load())) > uploadStallTimeout {
						stalled.Store(true)
						attemptCancel()
						return
					}
				}
			}
		}()

		var reqBody io.Reader = io.NewSectionReader(file, startByte, partSize)
		reqBody = &progressSectionReader{
			r: reqBody,
			fn: func(n int64) {
				lastProgressAt.Store(time.Now().UnixNano())
				if progress != nil {
					progress(n)
				}
			},
		}
		request, err := http.NewRequestWithContext(
			attemptCtx,
			http.MethodPut,
			uploadURL,
			reqBody,
		)
		if err != nil {
			attemptCancel()
			return "", err
		}
		request.ContentLength = partSize
		request.Header.Set("Content-Type", contentType)
		if contentRange != "" {
			request.Header.Set("Content-Range", contentRange)
		}

		response, err := c.uploadClient.Do(request)
		close(stopMonitor)
		if err != nil {
			if stalled.Load() {
				lastErr = fmt.Errorf("%s stalled: no network progress for %s (connection to the storage endpoint may be blocked)", operation, uploadStallTimeout)
			} else {
				lastErr = err
			}
			attemptCancel()
			if attempt < uploadRetryMaxAttempts {
				if waitErr := waitUploadRetry(ctx, attempt); waitErr != nil {
					return "", waitErr
				}
			}
			continue
		}
		attemptCancel()

		body, readErr := io.ReadAll(io.LimitReader(response.Body, 4096))
		response.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt < uploadRetryMaxAttempts {
				if waitErr := waitUploadRetry(ctx, attempt); waitErr != nil {
					return "", waitErr
				}
			}
			continue
		}

		if isSuccess(response.StatusCode) {
			return strings.Trim(response.Header.Get("ETag"), `"`), nil
		}

		message := strings.TrimSpace(string(body))
		if message == "" {
			message = response.Status
		}
		lastErr = fmt.Errorf("%s failed: %s", operation, message)
		if !retryableUploadStatus(response.StatusCode) || attempt == uploadRetryMaxAttempts {
			return "", lastErr
		}
		if waitErr := waitUploadRetry(ctx, attempt); waitErr != nil {
			return "", waitErr
		}
	}
	return "", lastErr
}

func retryableUploadStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

func waitUploadRetry(ctx context.Context, attempt int) error {
	delay := time.Duration(attempt) * uploadRetryBackoff
	if delay > 2*time.Second {
		delay = 2 * time.Second
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

func (c *HTTPEmosClient) RequestMultipartPresigns(ctx context.Context, access EmosAccess, fileID string, numChunks int) ([]EmosMultipartPart, error) {
	if err := validateEmosAccess(access); err != nil {
		return nil, err
	}
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil, errors.New("file id is required for multipart presigns")
	}
	if numChunks <= 0 {
		return nil, errors.New("multipart part count must be positive")
	}

	path := "/api/upload/multipart/" + url.PathEscape(fileID) + "/presign"
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		response, err := c.doAuthorizedJSON(ctx, access, http.MethodPost, path, map[string]int{"number": numChunks})
		if err != nil {
			lastErr = err
			if attempt < 3 {
				if waitErr := waitUploadRetry(ctx, attempt); waitErr != nil {
					return nil, waitErr
				}
			}
			continue
		}

		if response.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
			response.Body.Close()
			lastErr = fmt.Errorf("emos multipart presign request failed: %s", strings.TrimSpace(string(body)))
			if attempt < 3 {
				if waitErr := waitUploadRetry(ctx, attempt); waitErr != nil {
					return nil, waitErr
				}
			}
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(response.Body, 8<<20))
		response.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt < 3 {
				if waitErr := waitUploadRetry(ctx, attempt); waitErr != nil {
					return nil, waitErr
				}
			}
			continue
		}

		var parts []EmosMultipartPart
		if err := json.Unmarshal(body, &parts); err != nil {
			var wrapper struct {
				Data []EmosMultipartPart `json:"data"`
			}
			if wrapErr := json.Unmarshal(body, &wrapper); wrapErr != nil {
				return nil, errors.New("emos multipart presign response format unexpected")
			}
			parts = wrapper.Data
		}
		if len(parts) == 0 {
			return nil, errors.New("emos multipart presign response is empty")
		}
		for _, part := range parts {
			if part.Number <= 0 || strings.TrimSpace(part.UploadURL) == "" {
				return nil, errors.New("emos multipart presign response contains invalid part")
			}
		}
		return parts, nil
	}
	return nil, lastErr
}

func (c *HTTPEmosClient) CompleteMultipart(ctx context.Context, access EmosAccess, fileID string, parts []EmosMultipartPart) error {
	if err := validateEmosAccess(access); err != nil {
		return err
	}
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return errors.New("file id is required for multipart complete")
	}
	if len(parts) == 0 {
		return errors.New("multipart complete requires uploaded parts")
	}

	path := "/api/upload/multipart/" + url.PathEscape(fileID) + "/complete"
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		completeParts := make([]struct {
			Number int    `json:"number"`
			ETag   string `json:"etag"`
		}, len(parts))
		for i, part := range parts {
			completeParts[i] = struct {
				Number int    `json:"number"`
				ETag   string `json:"etag"`
			}{
				Number: part.Number,
				ETag:   part.ETag,
			}
		}
		response, err := c.doAuthorizedJSON(ctx, access, http.MethodPost, path, map[string]any{"parts": completeParts})
		if err != nil {
			lastErr = err
			if attempt < 3 {
				if waitErr := waitUploadRetry(ctx, attempt); waitErr != nil {
					return waitErr
				}
			}
			continue
		}

		if response.StatusCode == http.StatusOK || response.StatusCode == http.StatusCreated {
			response.Body.Close()
			return nil
		}

		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		response.Body.Close()
		lastErr = fmt.Errorf("emos multipart complete failed: %s", strings.TrimSpace(string(body)))
		if !retryableUploadStatus(response.StatusCode) || attempt == 3 {
			return lastErr
		}
		if waitErr := waitUploadRetry(ctx, attempt); waitErr != nil {
			return waitErr
		}
	}
	return lastErr
}

func (c *HTTPEmosClient) SaveVideo(ctx context.Context, access EmosAccess, req EmosSaveVideoRequest) (EmosSaveVideoResult, error) {
	if err := validateEmosAccess(access); err != nil {
		return EmosSaveVideoResult{}, err
	}

	response, err := c.doAuthorizedJSON(ctx, access, http.MethodPost, "/api/upload/video/save", req)
	if err != nil {
		return EmosSaveVideoResult{}, err
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		message := parseErrorMessage(body)
		if message == "" {
			message = response.Status
		}
		return EmosSaveVideoResult{}, &EmosSaveError{
			StatusCode: response.StatusCode,
			Message:    message,
		}
	}

	var result EmosSaveVideoResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return EmosSaveVideoResult{}, err
	}

	return result, nil
}

func (c *HTTPEmosClient) doAuthorizedJSON(ctx context.Context, access EmosAccess, method, path string, payload any) (*http.Response, error) {
	endpoint := strings.TrimRight(access.BaseURL, "/") + path

	var bodyReader io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		bodyReader = strings.NewReader(string(body))
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(access.Token))
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	return c.apiClient.Do(request)
}

func validateEmosAccess(access EmosAccess) error {
	if strings.TrimSpace(access.BaseURL) == "" {
		return errors.New("emos base_url is required")
	}
	if strings.TrimSpace(access.Token) == "" {
		return errors.New("emos token is required")
	}
	return nil
}

func detectContentType(filePath string) string {
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(filePath)))
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}

func parseErrorMessage(body []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		return firstNonEmptyString(
			stringValue(payload["message"]),
			stringValue(payload["error"]),
			stringValue(payload["detail"]),
		)
	}
	return strings.TrimSpace(string(body))
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func multipartSizeBounds(raw json.RawMessage) (int64, int64) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, 0
	}

	var number int64
	if err := json.Unmarshal(raw, &number); err == nil && number > 0 {
		return number, number
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if number, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64); err == nil && number > 0 {
			return number, number
		}
	}

	var bounds struct {
		Min int64 `json:"min"`
		Max int64 `json:"max"`
	}
	if err := json.Unmarshal(raw, &bounds); err == nil {
		return maxInt64(0, bounds.Min), maxInt64(0, bounds.Max)
	}
	return 0, 0
}

func acceptedUploadStatus(statusCode int, r2 bool) bool {
	switch statusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent:
		return true
	case http.StatusPermanentRedirect:
		return !r2
	default:
		return false
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
