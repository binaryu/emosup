package scheduler

type RecoverySummary struct {
	Total                int `json:"total"`
	Queued               int `json:"queued"`
	DownloadingRecovered int `json:"downloading_recovered"`
	DownloadingFailed    int `json:"downloading_failed"`
	DownloadCompleted    int `json:"download_completed_recovered"`
	SavingResumed        int `json:"saving_resumed"`
	SavingFailed         int `json:"saving_failed"`
	UploadingFailed      int `json:"uploading_failed"`
	UploadPending        int `json:"upload_pending"`
}
