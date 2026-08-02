package client

// DownloadStatus is a snapshot of a download's progress, shared by the
// download executor and the task store.
type DownloadStatus struct {
	Status          string         `json:"status"`
	TotalLength     int64          `json:"total_length"`
	CompletedLength int64          `json:"completed_length"`
	DownloadSpeed   int64          `json:"download_speed"`
	Files           []DownloadFile `json:"files"`
	ErrorMessage    string         `json:"error_message"`
}

type DownloadFile struct {
	Path string `json:"path"`
}
