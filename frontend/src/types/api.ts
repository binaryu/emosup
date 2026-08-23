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
  /** Custom site title shown in the browser tab and sidebar. */
  web_title: string
}

export interface LocalConfig {
  /** Absolute path for「本地媒体」browse root. Empty = download_dir / data/downloads. */
  root: string
}

export interface AuthConfig {
  username: string
  password: string
  jwt_secret: string
  token_ttl_hours: number
}

export interface LoginResponse {
  token: string
  token_type: string
  expires_at: string
  username: string
}

export interface OpenListConfig {
  base_url: string
  username: string
  password: string
  token: string
}

export interface DownloadConfig {
  dir: string
}

export interface EmosConfig {
  base_url: string
  token: string
  storage: string
}

export interface QBittorrentConfig {
  base_url: string
  username: string
  password: string
  save_path: string
}

export interface QBittorrentTorrent {
  hash: string
  name: string
  state: string
  progress: number
  size: number
  downloaded: number
  uploaded: number
  ratio: number
  save_path: string
  content_path: string
  category: string
  added_on: number
  completion_on: number
  dlspeed: number
  upspeed: number
}

export interface QBittorrentFile {
  index: number
  name: string
  size: number
  progress: number
  priority: number
  is_seed: boolean
}

export interface CacheEntry {
  path: string
  name: string
  size: number
  modified_at: string
  is_temp: boolean
  referenced: boolean
  task_id?: string
  task_status?: string
  task_file_name?: string
  keep_local_file?: boolean
}

export interface CacheListResult {
  dir: string
  entries: CacheEntry[]
  total_size: number
  orphan_count: number
  active_ref_count: number
}

export interface WorkerConfig {
  poll_interval_seconds: number
  max_concurrency: number
  download_threads: number
  upload_chunk_size_mb: number
  upload_part_concurrency: number
  save_retry_interval_seconds: number
  save_retry_max_attempts: number
  tmdb_api_key: string
  proxy_backends: string
  auto_tune?: boolean
}

export interface AppConfig {
  server: ServerConfig
  auth: AuthConfig
  local: LocalConfig
  openlist: OpenListConfig
  download: DownloadConfig
  emos: EmosConfig
  worker: WorkerConfig
  qbittorrent: QBittorrentConfig
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
  keep_local_file?: boolean
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
  active: number
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

export interface UpgradeCheck {
  current: string
  latest: string
  has_update: boolean
  name: string
  body: string
  published_at: string
}

export interface UpgradeResult {
  version: string
}
