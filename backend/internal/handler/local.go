package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"emosup/backend/internal/service"
)

type LocalHandler struct {
	service *service.LocalService
}

func NewLocalHandler(service *service.LocalService) *LocalHandler {
	return &LocalHandler{service: service}
}

func (h *LocalHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/local/list", h.browse)
}

func (h *LocalHandler) browse(c *gin.Context) {
	path := c.DefaultQuery("path", "/")
	displayPath, entries, err := h.service.Browse(c.Request.Context(), path)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, gin.H{
		"path":  displayPath,
		"items": entries,
	})
}
