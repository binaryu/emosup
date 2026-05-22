package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"emosup/backend/internal/client"
	"emosup/backend/internal/store"
)

type TMDBHandler struct {
	client *client.TMDBClient
	store  *store.FileStore
}

func NewTMDBHandler(client *client.TMDBClient, store *store.FileStore) *TMDBHandler {
	return &TMDBHandler{client: client, store: store}
}

func (h *TMDBHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/tmdb/search", h.search)
}

func (h *TMDBHandler) search(c *gin.Context) {
	query := c.Query("query")
	mediaType := c.DefaultQuery("type", "tv")
	if query == "" {
		respondError(c, http.StatusBadRequest, "query is required")
		return
	}

	cfg, err := h.store.LoadConfig()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	apiKey := cfg.Worker.TMDBAPIKey
	if apiKey == "" {
		respondError(c, http.StatusBadRequest, "tmdb_api_key is not configured")
		return
	}

	results, err := h.client.Search(c.Request.Context(), apiKey, query, mediaType)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	type searchItem struct {
		TMDBID     int64  `json:"tmdb_id"`
		Title      string `json:"title"`
		Year       string `json:"year"`
		Type       string `json:"type"`
		PosterPath string `json:"poster_path"`
	}

	items := make([]searchItem, 0, len(results))
	for _, r := range results {
		year := ""
		if len(r.Year) >= 4 {
			year = r.Year[:4]
		}
		name := r.Title
		if name == "" {
			name = r.Name
		}
		items = append(items, searchItem{
			TMDBID:     r.ID,
			Title:      name,
			Year:       year,
			Type:       mediaType,
			PosterPath: r.PosterPath,
		})
	}

	respondOK(c, items)
}
