package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"emosup/backend/internal/service"
)

type OpenListHandler struct {
	service *service.OpenListService
}

func NewOpenListHandler(service *service.OpenListService) *OpenListHandler {
	return &OpenListHandler{service: service}
}

func (h *OpenListHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/openlist/list", h.browse)
	router.GET("/openlist/tree", h.browse)
}

func (h *OpenListHandler) browse(c *gin.Context) {
	path := c.DefaultQuery("path", "/")
	entries, err := h.service.Browse(c.Request.Context(), path)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, gin.H{
		"path":  path,
		"items": entries,
	})
}
