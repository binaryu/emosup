package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"emosup/backend/internal/service"
)

type UpgradeHandler struct {
	service *service.UpgradeService
}

func NewUpgradeHandler(service *service.UpgradeService) *UpgradeHandler {
	return &UpgradeHandler{service: service}
}

func (h *UpgradeHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/upgrade/check", h.check)
	router.POST("/upgrade/run", h.run)
	router.GET("/upgrade/status", h.status)
}

func (h *UpgradeHandler) check(c *gin.Context) {
	result, err := h.service.Check(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c, result)
}

// run starts the upgrade in the background and returns immediately; the
// download/install continues even if the browser times out or disconnects.
// Progress and errors are read via GET /upgrade/status.
func (h *UpgradeHandler) run(c *gin.Context) {
	if err := h.service.Start(); err != nil {
		respondError(c, http.StatusConflict, err.Error())
		return
	}
	respondOK(c, gin.H{"started": true})
}

func (h *UpgradeHandler) status(c *gin.Context) {
	respondOK(c, h.service.Status())
}
