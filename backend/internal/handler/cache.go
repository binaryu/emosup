package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"emosup/backend/internal/service"
)

type CacheHandler struct {
	service *service.CacheService
}

func NewCacheHandler(service *service.CacheService) *CacheHandler {
	return &CacheHandler{service: service}
}

func (h *CacheHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/cache", h.listCache)
	router.POST("/cache/delete", h.deleteCache)
}

func (h *CacheHandler) listCache(c *gin.Context) {
	result, err := h.service.List(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c, result)
}

func (h *CacheHandler) deleteCache(c *gin.Context) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Paths) == 0 {
		respondError(c, http.StatusBadRequest, "paths is required")
		return
	}

	deleted, failed, err := h.service.Delete(c.Request.Context(), req.Paths)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c, gin.H{"deleted": deleted, "failed": failed})
}
