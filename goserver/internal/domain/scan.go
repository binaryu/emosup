package domain

type MatchMode string

const (
	MatchModeStrict               MatchMode = "strict"
	MatchModeSingleSeasonAutofill MatchMode = "single_season_autofill"
)

type ScanItem struct {
	Name               string `json:"name"`
	Source             string `json:"source"`
	OLPath             string `json:"ol_path,omitempty"`
	LocalPath          string `json:"local_path,omitempty"`
	SizeBytes          int64  `json:"size_bytes"`
	Size               string `json:"size,omitempty"`
	Season             *int   `json:"season,omitempty"`
	Episode            *int   `json:"episode,omitempty"`
	Selected           bool   `json:"selected"`
	ManualID           string `json:"manual_id,omitempty"`
	MatchStatus        string `json:"match_status"`
	MatchText          string `json:"match_text"`
	ServerItemType     string `json:"server_item_type"`
	ServerItemID       *int   `json:"server_item_id,omitempty"`
	ServerHasMedia     *bool  `json:"server_has_media,omitempty"`
	ServerEpisodeTitle string `json:"server_episode_title"`
	ServerDateAir      string `json:"server_date_air"`
}

type ScanRemoteRequest struct {
	RootPath        string `json:"root_path"`
	OpenListBaseURL string `json:"openlist_base_url"`
	OpenListToken   string `json:"openlist_token"`
}

type ScanLocalRequest struct {
	LocalPath string `json:"local_path"`
}

type PrecheckRequest struct {
	TMDBID      int        `json:"tmdb_id"`
	MatchMode   MatchMode  `json:"match_mode"`
	EmosToken   string     `json:"emos_token"`
	EmosAPIBase string     `json:"emos_api_base"`
	Files       []ScanItem `json:"files"`
}

type TreeEpisode struct {
	EpisodeNumber int    `json:"episode_number"`
	ItemID        int    `json:"item_id"`
	HasMedia      bool   `json:"has_media"`
	EpisodeTitle  string `json:"episode_title"`
	DateAir       string `json:"date_air"`
}

type TreeSeason struct {
	SeasonNumber int           `json:"season_number"`
	Episodes     []TreeEpisode `json:"episodes"`
}

type TreeInfo struct {
	VideoType string       `json:"video_type"`
	ItemID    int          `json:"item_id"`
	Title     string       `json:"title"`
	Seasons   []TreeSeason `json:"seasons"`
}

type PrecheckResponse struct {
	Title         string     `json:"title"`
	VLID          int        `json:"vl_id"`
	DefaultSeason *int       `json:"default_season,omitempty"`
	VideoType     string     `json:"video_type"`
	Conflicts     []string   `json:"conflicts"`
	Files         []ScanItem `json:"files"`
}
