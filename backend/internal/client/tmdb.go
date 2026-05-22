package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type TMDBClient struct {
	httpClient *http.Client
}

func NewTMDBClient() *TMDBClient {
	return &TMDBClient{httpClient: &http.Client{Timeout: 10 * time.Second}}
}

type TMDBSearchResult struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	Name       string `json:"name"`
	Year       string `json:"release_date"`
	Overview   string `json:"overview"`
	PosterPath string `json:"poster_path"`
}

func (r TMDBSearchResult) DisplayTitle() string {
	name := r.Title
	if name == "" {
		name = r.Name
	}
	year := ""
	if len(r.Year) >= 4 {
		year = " (" + r.Year[:4] + ")"
	}
	return name + year
}

func (c *TMDBClient) Search(ctx context.Context, apiKey, query, mediaType string) ([]TMDBSearchResult, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("tmdb api key is required")
	}
	if query == "" {
		return nil, nil
	}

	endpoint := "https://api.themoviedb.org/3/search/"
	if mediaType == "movie" {
		endpoint += "movie"
	} else {
		endpoint += "tv"
	}

	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Set("api_key", apiKey)
	q.Set("query", query)
	q.Set("language", "zh-CN")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Results []TMDBSearchResult `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

func (c *TMDBClient) GetExternalID(ctx context.Context, apiKey string, tmdbID int64, mediaType string) (int64, error) {
	endpoint := fmt.Sprintf("https://api.themoviedb.org/3/%s/%d/external_ids", mediaType, strconv.FormatInt(tmdbID, 10))
	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Set("api_key", apiKey)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		TVDBID int64 `json:"tvdb_id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.TVDBID, nil
}
