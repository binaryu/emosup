package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"emosup/backend/internal/service"
)

type EmosHandler struct {
	service *service.EmosService
}

func NewEmosHandler(service *service.EmosService) *EmosHandler {
	return &EmosHandler{service: service}
}

func (h *EmosHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/emos/video/base", h.getVideoBase)
	router.GET("/emos/video/tree", h.getVideoTree)
}

func (h *EmosHandler) getVideoBase(c *gin.Context) {
	itemType := c.Query("item_type")
	itemID, err := strconv.ParseInt(c.Query("item_id"), 10, 64)
	if err != nil || itemID <= 0 {
		respondError(c, http.StatusBadRequest, "item_id must be a positive integer")
		return
	}

	base, err := h.service.GetVideoBase(c.Request.Context(), itemType, itemID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, base)
}

func (h *EmosHandler) getVideoTree(c *gin.Context) {
	tmdbID, err := strconv.ParseInt(c.Query("tmdb_id"), 10, 64)
	if err != nil || tmdbID <= 0 {
		respondError(c, http.StatusBadRequest, "tmdb_id must be a positive integer")
		return
	}
	videoType := c.Query("type")

	tree, err := h.service.GetVideoTree(c.Request.Context(), tmdbID, videoType)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, tree)
}
