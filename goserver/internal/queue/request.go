package queue

import (
	"fmt"
	"time"

	"emosup/goserver/internal/domain"
)

type EnqueueItem struct {
	Name               string `json:"name"`
	Source             string `json:"source"`
	OLPath             string `json:"ol_path"`
	LocalPath          string `json:"local_path"`
	SizeBytes          int64  `json:"size_bytes"`
	Season             *int   `json:"season"`
	Episode            *int   `json:"episode"`
	Selected           bool   `json:"selected"`
	ManualID           string `json:"manual_id"`
	MatchStatus        string `json:"match_status"`
	MatchText          string `json:"match_text"`
	ServerItemType     string `json:"server_item_type"`
	ServerItemID       *int   `json:"server_item_id"`
	ServerHasMedia     *bool  `json:"server_has_media"`
	ServerEpisodeTitle string `json:"server_episode_title"`
	ServerDateAir      string `json:"server_date_air"`
}

type EnqueueRequest struct {
	EmosToken       string        `json:"emos_token"`
	EmosAPIBase     string        `json:"emos_api_base"`
	TMDBID          int           `json:"tmdb_id"`
	Storage         string        `json:"storage"`
	ForceUpload     bool          `json:"force_upload"`
	MatchMode       string        `json:"match_mode"`
	OpenListBaseURL string        `json:"openlist_base_url"`
	OpenListToken   string        `json:"openlist_token"`
	CacheDir        string        `json:"cache_dir"`
	Aria2RPCURL     string        `json:"aria2_rpc_url"`
	Aria2RPCSecret  string        `json:"aria2_rpc_secret"`
	ChunkSizeMB     int           `json:"chunk_size_mb"`
	ParallelTasks   int           `json:"parallel_tasks"`
	DownloadThreads int           `json:"download_threads"`
	Files           []EnqueueItem `json:"files"`
}

type QueueAddResult struct {
	Added     int      `json:"added"`
	Skipped   int      `json:"skipped"`
	QueuedIDs []string `json:"queued_ids"`
}

func buildTaskID(now time.Time, idx int) string {
	return fmt.Sprintf("task-%d-%d", now.UnixNano(), idx)
}

func (i EnqueueItem) toTask(id string, req EnqueueRequest, createdAt time.Time) domain.Task {
	source := i.Source
	if source == "" {
		if i.LocalPath != "" {
			source = "local"
		} else {
			source = "openlist"
		}
	}
	return domain.Task{
		ID:               id,
		Name:             i.Name,
		Source:           source,
		OLPath:           i.OLPath,
		LocalPath:        i.LocalPath,
		SizeBytes:        i.SizeBytes,
		Season:           i.Season,
		Episode:          i.Episode,
		MatchStatus:      i.MatchStatus,
		MatchText:        i.MatchText,
		ServerItemType:   i.ServerItemType,
		ServerItemID:     i.ServerItemID,
		ServerHasMedia:   i.ServerHasMedia,
		EpisodeTitle:     i.ServerEpisodeTitle,
		ServerDateAir:    i.ServerDateAir,
		Status:           domain.TaskStatusPending,
		Stage:            domain.TaskStageQueued,
		StatusText:       "等待执行",
		CurrentFile:      i.Name,
		Download:         domain.Progress{Speed: "0 MB/s", ETA: "N/A"},
		Upload:           domain.Progress{Speed: "0 MB/s", ETA: "N/A"},
		CreatedAt:        createdAt,
		EmosToken:        req.EmosToken,
		EmosAPIBase:      req.EmosAPIBase,
		TMDBID:           req.TMDBID,
		Storage:          req.Storage,
		ForceUpload:      req.ForceUpload,
		MatchMode:        req.MatchMode,
		OpenListBaseURL:  req.OpenListBaseURL,
		OpenListToken:    req.OpenListToken,
		CacheDir:         req.CacheDir,
		Aria2RPCURL:      req.Aria2RPCURL,
		Aria2RPCSecret:   req.Aria2RPCSecret,
		ChunkSizeMB:      req.ChunkSizeMB,
		ParallelTasks:    req.ParallelTasks,
		DownloadThreads:  req.DownloadThreads,
	}
}
