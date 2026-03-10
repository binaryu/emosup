package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type UploadService struct {
	client *http.Client
}

type UploadTokenResult struct {
	UploadURL string
	FileID    string
}

type UploadProgress struct {
	Percent float64
	Speed   string
	ETA     string
	Done    bool
}

func NewUploadService() *UploadService {
	return &UploadService{client: &http.Client{Timeout: 60 * time.Second}}
}

func (s *UploadService) GetToken(apiBase, token, path, storage string) (*UploadTokenResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	mimeType := mime.TypeByExtension(filepath.Ext(path))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	payload := map[string]any{
		"type":         "video",
		"file_type":    mimeType,
		"file_name":    filepath.Base(path),
		"file_size":    info.Size(),
		"file_storage": storage,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, apiBase+"/api/upload/getUploadToken", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "EMOS-PRO-PANEL/5.1")
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getUploadToken failed http %d", resp.StatusCode)
	}
	var out struct {
		Data struct {
			UploadURL string `json:"upload_url"`
		} `json:"data"`
		FileID string `json:"file_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Data.UploadURL == "" || out.FileID == "" {
		return nil, fmt.Errorf("getUploadToken invalid response")
	}
	return &UploadTokenResult{UploadURL: out.Data.UploadURL, FileID: out.FileID}, nil
}

func (s *UploadService) SaveUpload(apiBase, token, itemType string, itemID int, fileID string) error {
	payload := map[string]any{"item_type": itemType, "item_id": itemID, "file_id": fileID}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, apiBase+"/api/upload/video/save", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "EMOS-PRO-PANEL/5.1")
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("save upload failed http %d", resp.StatusCode)
	}
	return nil
}

func (s *UploadService) UploadStreamChunked(ctx context.Context, filePath, uploadURL string, chunkSizeMB int, onProgress func(UploadProgress)) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	fileSize := info.Size()
	if chunkSizeMB <= 0 {
		chunkSizeMB = 64
	}
	chunkSize := int64(chunkSizeMB) * 1024 * 1024
	chunkSize = (chunkSize / (256 * 1024)) * (256 * 1024)
	if chunkSize == 0 {
		chunkSize = 256 * 1024
	}

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, chunkSize)
	var uploaded int64
	startAt := time.Now()
	for uploaded < fileSize {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, err := f.Read(buf)
		if err != nil && n == 0 {
			break
		}
		if n == 0 {
			break
		}
		start := uploaded
		end := start + int64(n) - 1
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(buf[:n]))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
		req.Header.Set("Content-Length", fmt.Sprintf("%d", n))
		resp, err := s.client.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("chunk upload failed http %d", resp.StatusCode)
		}
		uploaded += int64(n)
		elapsed := time.Since(startAt).Seconds()
		speedBytes := int64(0)
		if elapsed > 0 {
			speedBytes = int64(float64(uploaded) / elapsed)
		}
		eta := "N/A"
		if speedBytes > 0 {
			remain := (fileSize - uploaded) / speedBytes
			eta = fmt.Sprintf("%dm %ds", remain/60, remain%60)
		}
		if onProgress != nil {
			onProgress(UploadProgress{
				Percent: float64(uploaded) / float64(fileSize) * 100,
				Speed:   bytesToSpeed(speedBytes),
				ETA:     eta,
				Done:    uploaded >= fileSize,
			})
		}
	}
	if onProgress != nil {
		onProgress(UploadProgress{Percent: 100, Speed: "done", ETA: "N/A", Done: true})
	}
	return nil
}
