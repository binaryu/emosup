package handler

import (
	"log"
	"net/http"
	"os"
	"time"

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
}

func (h *UpgradeHandler) check(c *gin.Context) {
	result, err := h.service.Check(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c, result)
}

func (h *UpgradeHandler) run(c *gin.Context) {
	result, err := h.service.Run(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c, result)

	// The new files are already in place; exit shortly after the response
	// has been flushed. The detached restart script (spawned by Run) brings
	// the service back up via systemd or a direct re-exec.
	go func() {
		time.Sleep(1500 * time.Millisecond)
		log.Printf("upgrade to %s complete, exiting for restart", result.Version)
		os.Exit(0)
	}()
}
