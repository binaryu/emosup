package domain

import "time"

type TaskStatus string

type TaskStage string

type EventType string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusSuccess   TaskStatus = "success"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
	TaskStatusSkipped   TaskStatus = "skipped"
)

const (
	TaskStageIdle     TaskStage = "idle"
	TaskStageScan     TaskStage = "scan"
	TaskStagePrecheck TaskStage = "precheck"
	TaskStageQueued   TaskStage = "queued"
	TaskStageRunning  TaskStage = "running"
	TaskStageDownload TaskStage = "download"
	TaskStageUpload   TaskStage = "upload"
)

const (
	EventTaskQueued   EventType = "task.queued"
	EventTaskStarted  EventType = "task.started"
	EventTaskUpdated  EventType = "task.updated"
	EventTaskFinished EventType = "task.finished"
	EventLog          EventType = "log"
	EventSnapshot     EventType = "snapshot"
)

type Progress struct {
	Percent float64 `json:"percent"`
	Speed   string  `json:"speed"`
	ETA     string  `json:"eta"`
	Done    bool    `json:"done"`
}

type Task struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Source           string     `json:"source,omitempty"`
	OLPath           string     `json:"ol_path,omitempty"`
	LocalPath        string     `json:"local_path,omitempty"`
	SizeBytes        int64      `json:"size_bytes,omitempty"`
	Season           *int       `json:"season,omitempty"`
	Episode          *int       `json:"episode,omitempty"`
	MatchStatus      string     `json:"match_status,omitempty"`
	MatchText        string     `json:"match_text,omitempty"`
	ServerItemType   string     `json:"server_item_type,omitempty"`
	ServerItemID     *int       `json:"server_item_id,omitempty"`
	ServerHasMedia   *bool      `json:"server_has_media,omitempty"`
	EpisodeTitle     string     `json:"server_episode_title,omitempty"`
	ServerDateAir    string     `json:"server_date_air,omitempty"`
	Status           TaskStatus `json:"status"`
	Stage            TaskStage  `json:"stage"`
	StatusText       string     `json:"status_text"`
	LastError        string     `json:"last_error"`
	CurrentFile      string     `json:"current_file"`
	Download         Progress   `json:"download"`
	Upload           Progress   `json:"upload"`
	CreatedAt        time.Time  `json:"created_at"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	EmosToken        string     `json:"-"`
	EmosAPIBase      string     `json:"-"`
	TMDBID           int        `json:"-"`
	Storage          string     `json:"-"`
	ForceUpload      bool       `json:"-"`
	MatchMode        string     `json:"-"`
	OpenListBaseURL  string     `json:"-"`
	OpenListToken    string     `json:"-"`
	CacheDir         string     `json:"-"`
	Aria2RPCURL      string     `json:"-"`
	Aria2RPCSecret   string     `json:"-"`
	ChunkSizeMB      int        `json:"-"`
	ParallelTasks    int        `json:"-"`
	DownloadThreads  int        `json:"-"`
}

type TaskCounts struct {
	Pending   int `json:"pending"`
	Running   int `json:"running"`
	Success   int `json:"success"`
	Failed    int `json:"failed"`
	Skipped   int `json:"skipped"`
	Cancelled int `json:"cancelled"`
}

type Snapshot struct {
	WorkerRunning   bool       `json:"worker_running"`
	CancelRequested bool       `json:"cancel"`
	Stage           TaskStage  `json:"stage"`
	TotalFiles      int        `json:"total_files"`
	CompletedFiles  int        `json:"completed_files"`
	QueueSize       int        `json:"queue_size"`
	CurrentTaskID   string     `json:"current_task_id"`
	CurrentFile     string     `json:"current_file"`
	StatusText      string     `json:"status_text"`
	LastError       string     `json:"last_error"`
	Download        Progress   `json:"download"`
	Upload          Progress   `json:"upload"`
	RecentTasks     []Task     `json:"recent_tasks"`
	Counts          TaskCounts `json:"counts"`
	Tasks           []Task     `json:"tasks"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Event struct {
	Type      EventType `json:"type"`
	TaskID    string    `json:"task_id,omitempty"`
	Task      *Task     `json:"task,omitempty"`
	Snapshot  *Snapshot `json:"snapshot,omitempty"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
