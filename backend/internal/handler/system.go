package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"emosup/backend/internal/scheduler"
)

type SystemHandler struct {
	manager *scheduler.Manager
}

func NewSystemHandler(manager *scheduler.Manager) *SystemHandler {
	return &SystemHandler{manager: manager}
}

func (h *SystemHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/system/runtime", h.getRuntime)
	router.GET("/system/recovery", h.getRecovery)
}

func (h *SystemHandler) getRuntime(c *gin.Context) {
	if h.manager == nil {
		respondError(c, http.StatusServiceUnavailable, "scheduler is not configured")
		return
	}

	respondOK(c, h.manager.RuntimeStatus())
}

func (h *SystemHandler) getRecovery(c *gin.Context) {
	if h.manager == nil {
		respondError(c, http.StatusServiceUnavailable, "scheduler is not configured")
		return
	}

	respondOK(c, h.manager.RecoverySummary())
}
