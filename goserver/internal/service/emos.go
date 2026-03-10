package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"emosup/goserver/internal/domain"
)

type EmosService struct {
	client *http.Client
	mu     sync.Mutex
	cache  map[int]cachedTree
}

type cachedTree struct {
	At   time.Time
	Tree *domain.TreeInfo
}

func NewEmosService() *EmosService {
	return &EmosService{
		client: &http.Client{Timeout: 60 * time.Second},
		cache:  make(map[int]cachedTree),
	}
}

func (s *EmosService) GetTreeByTMDB(apiBase, token string, tmdbID int, cacheTTL time.Duration) (*domain.TreeInfo, error) {
	now := time.Now()
	s.mu.Lock()
	if cached, ok := s.cache[tmdbID]; ok && cached.Tree != nil && now.Sub(cached.At) < cacheTTL {
		treeCopy := *cached.Tree
		s.mu.Unlock()
		return &treeCopy, nil
	}
	s.mu.Unlock()

	base, err := url.Parse(apiBase)
	if err != nil {
		return nil, err
	}
	base.Path = "/api/video/tree"
	q := base.Query()
	q.Set("tmdb_id", fmt.Sprintf("%d", tmdbID))
	base.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, base.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "EMOS-PRO-PANEL/5.1")
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取 video/tree 失败 http %d", resp.StatusCode)
	}

	var payload []struct {
		VideoType string `json:"video_type"`
		ItemID    int    `json:"item_id"`
		Title     string `json:"title"`
		Seasons   []struct {
			SeasonNumber int `json:"season_number"`
			Episodes     []struct {
				EpisodeNumber int    `json:"episode_number"`
				ItemID        int    `json:"item_id"`
				HasMedia      bool   `json:"has_media"`
				EpisodeTitle  string `json:"episode_title"`
				DateAir       string `json:"date_air"`
			} `json:"episodes"`
		} `json:"seasons"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return nil, nil
	}

	root := payload[0]
	tree := &domain.TreeInfo{
		VideoType: root.VideoType,
		ItemID:    root.ItemID,
		Title:     root.Title,
		Seasons:   make([]domain.TreeSeason, 0, len(root.Seasons)),
	}
	for _, season := range root.Seasons {
		sea := domain.TreeSeason{
			SeasonNumber: season.SeasonNumber,
			Episodes:     make([]domain.TreeEpisode, 0, len(season.Episodes)),
		}
		for _, ep := range season.Episodes {
			sea.Episodes = append(sea.Episodes, domain.TreeEpisode{
				EpisodeNumber: ep.EpisodeNumber,
				ItemID:        ep.ItemID,
				HasMedia:      ep.HasMedia,
				EpisodeTitle:  ep.EpisodeTitle,
				DateAir:       ep.DateAir,
			})
		}
		tree.Seasons = append(tree.Seasons, sea)
	}

	s.mu.Lock()
	s.cache[tmdbID] = cachedTree{At: now, Tree: tree}
	s.mu.Unlock()

	return tree, nil
}
