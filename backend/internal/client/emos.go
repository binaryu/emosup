package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	Storage   string
	FileID    string
	UploadURL string
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
	UploadFile(ctx context.Context, uploadURL, filePath string, chunkSize int64, onProgress func(EmosUploadProgress) error) error
	SaveVideo(ctx context.Context, access EmosAccess, req EmosSaveVideoRequest) (EmosSaveVideoResult, error)
}

type HTTPEmosClient struct {
	apiClient    *http.Client
	uploadClient *http.Client
}

func NewHTTPEmosClient() *HTTPEmosClient {
	return &HTTPEmosClient{
		apiClient:    &http.Client{Timeout: 20 * time.Second},
		uploadClient: &http.Client{Timeout: 2 * time.Minute},
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

	var tree EmosVideoTree
	if err := json.NewDecoder(response.Body).Decode(&tree); err != nil {
		return EmosVideoTree{}, err
	}

	return tree, nil
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
		Type      string         `json:"type"`
		Storage   string         `json:"storage"`
		FileID    string         `json:"file_id"`
		UploadURL string         `json:"upload_url"`
		Data      map[string]any `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return EmosUploadTokenResult{}, err
	}

	uploadURL := strings.TrimSpace(raw.UploadURL)
	if uploadURL == "" {
		uploadURL = firstNonEmptyString(
			stringValue(raw.Data["upload_url"]),
			stringValue(raw.Data["url"]),
			stringValue(raw.Data["put_url"]),
		)
	}

	result := EmosUploadTokenResult{
		Storage:   firstNonEmptyString(raw.Storage, raw.Type, req.FileStorage, "default"),
		FileID:    strings.TrimSpace(raw.FileID),
		UploadURL: uploadURL,
	}
	if result.FileID == "" || result.UploadURL == "" {
		return EmosUploadTokenResult{}, errors.New("emos upload token response missing file_id or upload_url")
	}

	return result, nil
}

func (c *HTTPEmosClient) UploadFile(ctx context.Context, uploadURL, filePath string, chunkSize int64, onProgress func(EmosUploadProgress) error) error {
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
	contentType := detectContentType(filePath)

	var uploadedBytes int64
	for uploadedBytes < totalBytes {
		if err := ctx.Err(); err != nil {
			return err
		}

		partSize := minInt64(chunkSize, totalBytes-uploadedBytes)
		startByte := uploadedBytes
		endByte := uploadedBytes + partSize - 1

		request, err := http.NewRequestWithContext(
			ctx,
			http.MethodPut,
			uploadURL,
			io.NewSectionReader(file, startByte, partSize),
		)
		if err != nil {
			return err
		}
		request.ContentLength = partSize
		request.Header.Set("Content-Type", contentType)
		request.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", startByte, endByte, totalBytes))

		startedAt := time.Now()
		response, err := c.uploadClient.Do(request)
		if err != nil {
			return err
		}

		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		response.Body.Close()
		if response.StatusCode >= http.StatusMultipleChoices {
			message := strings.TrimSpace(string(body))
			if message == "" {
				message = response.Status
			}
			return fmt.Errorf("upload chunk failed: %s", message)
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

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
