package model

import "time"

type ScanSessionStatus string

const (
	ScanSessionStatusProcessing ScanSessionStatus = "processing"
	ScanSessionStatusCompleted  ScanSessionStatus = "completed"
	ScanSessionStatusFailed     ScanSessionStatus = "failed"
)

type MatchStatus string

const (
	MatchStatusMatched   MatchStatus = "matched"
	MatchStatusUnmatched MatchStatus = "unmatched"
	MatchStatusAmbiguous MatchStatus = "ambiguous"
	MatchStatusInvalid   MatchStatus = "invalid"
)

type ScanSession struct {
	ID             string            `json:"id"`
	Source         string            `json:"source"`
	Path           string            `json:"path"`
	TMDBID         int64             `json:"tmdb_id"`
	VideoType      string            `json:"video_type"`
	Status         ScanSessionStatus `json:"status"`
	TotalCount     int               `json:"total_count"`
	MatchedCount   int               `json:"matched_count"`
	UnmatchedCount int               `json:"unmatched_count"`
	Items          []ScanItem        `json:"items"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type ParsedEpisodeInfo struct {
	Season    *int   `json:"season,omitempty"`
	Episode   *int   `json:"episode,omitempty"`
	IsSpecial bool   `json:"is_special"`
	RawText   string `json:"raw_text,omitempty"`
}

type MatchCandidate struct {
	ItemType string `json:"item_type"`
	ItemID   int64  `json:"item_id"`
	Title    string `json:"title"`
}

type ScanItem struct {
	ID               string            `json:"id"`
	ScanSessionID    string            `json:"scan_session_id"`
	OpenListPath     string            `json:"openlist_path"`
	FileName         string            `json:"file_name"`
	FileSize         int64             `json:"file_size"`
	RawURL           string            `json:"raw_url"`
	IsVideo          bool              `json:"is_video"`
	Parsed           ParsedEpisodeInfo `json:"parsed"`
	MatchStatus      MatchStatus       `json:"match_status"`
	MatchReason      string            `json:"match_reason"`
	MatchCandidates  []MatchCandidate  `json:"match_candidates"`
	SelectedItemType string            `json:"selected_item_type"`
	SelectedItemID   int64             `json:"selected_item_id"`
	SelectedTitle    string            `json:"selected_title"`
	Confirmed        bool              `json:"confirmed"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}
