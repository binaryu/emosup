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
}

func NewMatchService() *MatchService {
	return &MatchService{}
}

func (s *MatchService) Match(tree client.EmosVideoTree, parsed model.ParsedEpisodeInfo) MatchResult {
	if tree.VideoType == "movie" {
		return MatchResult{
			Status:           model.MatchStatusMatched,
			Candidates:       []model.MatchCandidate{},
			SelectedItemType: tree.ItemType,
			SelectedItemID:   tree.ItemID,
			SelectedTitle:    tree.Title,
		}
	}

	if parsed.Episode == nil {
		return MatchResult{
			Status:     model.MatchStatusUnmatched,
			Reason:     "未能从文件名或路径解析出集数信息。",
			Candidates: []model.MatchCandidate{},
		}
	}

	seasonNumber := 1
	if parsed.Season != nil {
		seasonNumber = *parsed.Season
	}
	if parsed.IsSpecial {
		seasonNumber = 0
	}

	candidates := make([]model.MatchCandidate, 0, 1)
	for _, season := range tree.Seasons {
		if season.SeasonNumber != seasonNumber {
			continue
		}

		for _, episode := range season.Episodes {
			if episode.EpisodeNumber != *parsed.Episode {
				continue
			}

			candidates = append(candidates, model.MatchCandidate{
				ItemType: episode.ItemType,
				ItemID:   episode.ItemID,
				Title:    buildEpisodeTitle(tree.Title, season.SeasonNumber, episode.EpisodeNumber, episode.EpisodeTitle),
			})
		}
	}

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

func buildEpisodeTitle(seriesTitle string, seasonNumber, episodeNumber int, episodeTitle string) string {
	if episodeTitle == "" {
		return fmt.Sprintf("%s - S%02dE%02d", seriesTitle, seasonNumber, episodeNumber)
	}

	return fmt.Sprintf("%s - S%02dE%02d - %s", seriesTitle, seasonNumber, episodeNumber, episodeTitle)
}
