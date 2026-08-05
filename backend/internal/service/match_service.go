package service

import (
	"fmt"

	"emosup/backend/internal/client"
	"emosup/backend/internal/model"
)

type MatchService struct{}

type MatchResult struct {
	Status           model.MatchStatus
	Reason           string
	Candidates       []model.MatchCandidate
	SelectedItemType string
	SelectedItemID   int64
	SelectedTitle    string
	HasMedia         *bool
}

func NewMatchService() *MatchService {
	return &MatchService{}
}

// VideoTreeIndex pre-indexes an Emos video tree once so per-file matching and
// has_media lookups are O(1) instead of scanning every season/episode.
type VideoTreeIndex struct {
	videoType string
	itemType  string
	itemID    int64
	title     string
	episodes  map[int]map[int][]model.MatchCandidate
	hasMedia  map[int64]*bool
}

func (s *MatchService) BuildIndex(tree client.EmosVideoTree) *VideoTreeIndex {
	idx := &VideoTreeIndex{
		videoType: tree.VideoType,
		itemType:  tree.ItemType,
		itemID:    tree.ItemID,
		title:     tree.Title,
		episodes:  make(map[int]map[int][]model.MatchCandidate),
		hasMedia:  make(map[int64]*bool),
	}
	for _, season := range tree.Seasons {
		eps, ok := idx.episodes[season.SeasonNumber]
		if !ok {
			eps = make(map[int][]model.MatchCandidate)
			idx.episodes[season.SeasonNumber] = eps
		}
		for _, episode := range season.Episodes {
			eps[episode.EpisodeNumber] = append(eps[episode.EpisodeNumber], model.MatchCandidate{
				ItemType: episode.ItemType,
				ItemID:   episode.ItemID,
				Title:    buildEpisodeTitle(tree.Title, season.SeasonNumber, episode.EpisodeNumber, episode.EpisodeTitle),
			})
			hasMedia := episode.HasMedia
			idx.hasMedia[episode.ItemID] = &hasMedia
		}
	}
	return idx
}

func (s *MatchService) Match(tree client.EmosVideoTree, parsed model.ParsedEpisodeInfo) MatchResult {
	return s.BuildIndex(tree).Match(parsed)
}

func (idx *VideoTreeIndex) Match(parsed model.ParsedEpisodeInfo) MatchResult {
	if idx.videoType == "movie" {
		return MatchResult{
			Status:           model.MatchStatusMatched,
			Candidates:       []model.MatchCandidate{},
			SelectedItemType: idx.itemType,
			SelectedItemID:   idx.itemID,
			SelectedTitle:    idx.title,
		}
	}

	if parsed.Episode == nil {
		return MatchResult{
			Status:     model.MatchStatusUnmatched,
			Reason:     "未能从文件名或路径解析出集数信息。",
			Candidates: []model.MatchCandidate{},
		}
	}

	// Default season 1 when only episode is known (common for single-season packs).
	seasonNumber := 1
	if parsed.Season != nil {
		seasonNumber = *parsed.Season
	}
	if parsed.IsSpecial {
		seasonNumber = 0
	}

	candidates := idx.episodes[seasonNumber][*parsed.Episode]

	switch len(candidates) {
	case 0:
		return MatchResult{
			Status:     model.MatchStatusUnmatched,
			Reason:     fmt.Sprintf("未在 Emos 视频树中找到 S%02dE%02d 对应条目。", seasonNumber, *parsed.Episode),
			Candidates: []model.MatchCandidate{},
		}
	case 1:
		return MatchResult{
			Status:           model.MatchStatusMatched,
			Candidates:       candidates,
			SelectedItemType: candidates[0].ItemType,
			SelectedItemID:   candidates[0].ItemID,
			SelectedTitle:    candidates[0].Title,
		}
	default:
		return MatchResult{
			Status:     model.MatchStatusAmbiguous,
			Reason:     "检测到多个可选匹配结果，请人工确认。",
			Candidates: candidates,
		}
	}
}

func (idx *VideoTreeIndex) LookupHasMedia(itemID int64) *bool {
	return idx.hasMedia[itemID]
}

func buildEpisodeTitle(seriesTitle string, seasonNumber, episodeNumber int, episodeTitle string) string {
	if episodeTitle == "" {
		return fmt.Sprintf("%s - S%02dE%02d", seriesTitle, seasonNumber, episodeNumber)
	}
	return fmt.Sprintf("%s - S%02dE%02d - %s", seriesTitle, seasonNumber, episodeNumber, episodeTitle)
}
