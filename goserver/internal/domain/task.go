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
)

const (
	TaskStageIdle     TaskStage = "idle"
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
	EventSnapshot     EventType = "snapshot"
)

type Progress struct {
	Percent float64 `json:"percent"`
	Speed   string  `json:"speed"`
	ETA     string  `json:"eta"`
	Done    bool    `json:"done"`
}

type Task struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Status      TaskStatus `json:"status"`
	Stage       TaskStage  `json:"stage"`
	StatusText  string     `json:"status_text"`
	LastError   string     `json:"last_error"`
	CurrentFile string     `json:"current_file"`
	Download    Progress   `json:"download"`
	Upload      Progress   `json:"upload"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

type Snapshot struct {
	WorkerRunning bool       `json:"worker_running"`
	QueueSize     int        `json:"queue_size"`
	CurrentTaskID string     `json:"current_task_id"`
	Tasks         []Task     `json:"tasks"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type Event struct {
	Type      EventType `json:"type"`
	TaskID    string    `json:"task_id,omitempty"`
	Task      *Task     `json:"task,omitempty"`
	Snapshot  *Snapshot `json:"snapshot,omitempty"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
