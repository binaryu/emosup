package model

import "time"

type TaskStatus string

const (
	TaskStatusQueued            TaskStatus = "queued"
	TaskStatusDownloading       TaskStatus = "downloading"
	TaskStatusDownloadFailed    TaskStatus = "download_failed"
	TaskStatusDownloadCompleted TaskStatus = "download_completed"
	TaskStatusUploadPending     TaskStatus = "upload_pending"
	TaskStatusUploading         TaskStatus = "uploading"
	TaskStatusSaving            TaskStatus = "saving"
	TaskStatusUploadFailed      TaskStatus = "upload_failed"
	TaskStatusCompleted         TaskStatus = "completed"
	TaskStatusCanceled          TaskStatus = "canceled"
)

type Task struct {
	ID            string       `json:"id"`
	ScanSessionID string       `json:"scan_session_id"`
	ScanItemID    string       `json:"scan_item_id"`
	Status        TaskStatus   `json:"status"`
	Paused        bool         `json:"paused"`
	RetryCount    int          `json:"retry_count"`
	Source        TaskSource   `json:"source"`
	Parsed        TaskParsed   `json:"parsed"`
	Target        TaskTarget   `json:"target"`
	Download      TaskDownload `json:"download"`
	Upload        TaskUpload   `json:"upload"`
	Result        TaskResult   `json:"result"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
	FinishedAt    *time.Time   `json:"finished_at,omitempty"`
}

type TaskSource struct {
	Type     string `json:"type"`
	Path     string `json:"path"`
	RawURL   string `json:"raw_url"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
}

type TaskParsed struct {
	Season    *int `json:"season,omitempty"`
	Episode   *int `json:"episode,omitempty"`
	IsSpecial bool `json:"is_special"`
}

type TaskTarget struct {
	TMDBID   int64  `json:"tmdb_id"`
	ItemType string `json:"item_type"`
	ItemID   int64  `json:"item_id"`
	Title    string `json:"title"`
}

type TaskDownload struct {
	SaveDir        string  `json:"save_dir"`
	LocalPath      string  `json:"local_path"`
	Status         string  `json:"status"`
	TotalBytes     int64   `json:"total_bytes"`
	CompletedBytes int64   `json:"completed_bytes"`
	Progress       float64 `json:"progress"`
	Speed          int64   `json:"speed"`
}

type TaskUpload struct {
	Storage           string                `json:"storage"`
	FileID            string                `json:"file_id"`
	UploadURL         string                `json:"upload_url"`
	UploadType        string                `json:"upload_type"`
	MultipartSizeMin  int64                 `json:"multipart_size_min"`
	MultipartSizeMax  int64                 `json:"multipart_size_max"`
	MultipartPresigns []UploadMultipartPart `json:"multipart_presigns,omitempty"`
	MultipartParts    []UploadMultipartPart `json:"multipart_parts,omitempty"`
	MediaID           string                `json:"media_id"`
	TotalBytes        int64                 `json:"total_bytes"`
	UploadedBytes     int64                 `json:"uploaded_bytes"`
	Progress          float64               `json:"progress"`
	Speed             int64                 `json:"speed"`
	Status            string                `json:"status"`
	SaveRetryCount    int                   `json:"save_retry_count"`
	LastSaveError     string                `json:"last_save_error"`
}

type UploadMultipartPart struct {
	Number    int    `json:"number"`
	UploadURL string `json:"upload_url"`
	ETag      string `json:"etag,omitempty"`
}

type TaskResult struct {
	ErrorMessage string     `json:"error_message"`
	ErrorStage   string     `json:"error_stage"`
	ErrorCode    string     `json:"error_code"`
	LastErrorAt  *time.Time `json:"last_error_at,omitempty"`
}

type TaskLog struct {
	TaskID string        `json:"task_id"`
	Items  []TaskLogItem `json:"items"`
}

type TaskLogItem struct {
	ID      string    `json:"id"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

type TaskListResult struct {
	Items    []Task `json:"items"`
	Total    int    `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type TaskStats struct {
	Total     int `json:"total"`
	Queued    int `json:"queued"`
	Canceled  int `json:"canceled"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}
