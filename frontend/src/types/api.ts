export type TaskStatus =
  | 'queued'
  | 'downloading'
  | 'download_failed'
  | 'download_completed'
  | 'upload_pending'
  | 'uploading'
  | 'saving'
  | 'upload_failed'
  | 'completed'
  | 'canceled'

export interface ApiResponse<T> {
  success: boolean
  message?: string
  data: T
}

export interface ServerConfig {
  host: string
  port: number
}

export interface OpenListConfig {
  base_url: string
  username: string
  password: string
  token: string
}

export interface Aria2Config {
  rpc_url: string
  secret: string
  download_dir: string
  poll_interval_seconds: number
  connect_timeout_seconds: number
}

export interface EmosConfig {
  base_url: string
  token: string
  storage: string
}

export interface WorkerConfig {
  poll_interval_seconds: number
  max_concurrency: number
  upload_chunk_size_mb: number
  save_retry_interval_seconds: number
  save_retry_max_attempts: number
  tmdb_api_key: string
  proxy_backends: string
}

export interface AppConfig {
  server: ServerConfig
  openlist: OpenListConfig
  aria2: Aria2Config
  emos: EmosConfig
  worker: WorkerConfig
}

export interface OpenListEntry {
  name: string
  path: string
  is_dir: boolean
  size: number
  modified_at?: string
}

export interface ParsedEpisodeInfo {
  season?: number
  episode?: number
  is_special: boolean
  raw_text?: string
}

export interface MatchCandidate {
  item_type: string
  item_id: number
  title: string
}

export interface ScanItem {
  id: string
  scan_session_id: string
  file_name: string
  openlist_path: string
  file_size: number
  raw_url: string
  is_video: boolean
  has_media?: boolean
  parsed: ParsedEpisodeInfo
  match_status: string
  match_reason: string
  match_candidates: MatchCandidate[]
  selected_item_type: string
  selected_item_id: number
  selected_title: string
  confirmed: boolean
  created_at: string
  updated_at: string
}

export interface ScanSession {
  id: string
  source: string
  path: string
  tmdb_id: number
  video_type: string
  status: string
  total_count: number
  matched_count: number
  unmatched_count: number
  created_at: string
  updated_at: string
  items: ScanItem[]
}

export interface Task {
  id: string
  scan_session_id: string
  scan_item_id: string
  status: TaskStatus
  retry_count: number
  paused: boolean
  created_at: string
  updated_at: string
  finished_at?: string
  parsed: {
    season?: number
    episode?: number
    is_special: boolean
  }
  source: {
    type: string
    path: string
    raw_url: string
    file_name: string
    file_size: number
  }
  target: {
    tmdb_id: number
    item_type: string
    item_id: number
    title: string
  }
  download: {
    aria2_gid: string
    save_dir: string
    local_path: string
    status: string
    total_bytes: number
    completed_bytes: number
    progress: number
    speed: number
  }
  upload: {
    storage: string
    file_id: string
    upload_url: string
    media_id: string
    status: string
    progress: number
    total_bytes: number
    uploaded_bytes: number
    speed: number
    save_retry_count: number
    last_save_error: string
  }
  result: {
    error_message: string
    error_stage: string
    error_code: string
    last_error_at?: string
  }
}

export interface TaskLogEntry {
  id: string
  level: string
  message: string
  time: string
}

export interface TaskLog {
  task_id: string
  items: TaskLogEntry[]
}

export interface TaskListResponse {
  items: Task[]
  total: number
  page: number
  page_size: number
}

export interface BatchCreateTasksResponse {
  created: Array<{
    task_id: string
    item_id: string
  }>
  failed: Array<{
    item_id: string
    reason: string
  }>
}

export interface TaskStats {
  total: number
  queued: number
  canceled: number
  completed: number
  failed: number
}

export interface RuntimeStatus {
  scheduler_running: boolean
  current_task_ids: string[]
  current_stage: string
  max_concurrency: number
  started_at?: string
}

export interface RecoverySummary {
  total: number
  queued: number
  downloading_recovered: number
  downloading_failed: number
  download_completed_recovered: number
  saving_resumed: number
  saving_failed: number
  uploading_failed: number
  upload_pending: number
}
