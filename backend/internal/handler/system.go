package handler

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"

	"emosup/backend/internal/scheduler"
	"emosup/backend/internal/service"
	"emosup/backend/internal/store"
	"emosup/backend/internal/version"
)

type SystemHandler struct {
	manager *scheduler.Manager
	store   *store.FileStore
	logs    *service.LogService
}

func NewSystemHandler(manager *scheduler.Manager, store *store.FileStore) *SystemHandler {
	return &SystemHandler{manager: manager, store: store, logs: service.NewLogService()}
}

func (h *SystemHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/system/runtime", h.getRuntime)
	router.GET("/system/recovery", h.getRecovery)
	router.GET("/system/disk", h.getDiskUsage)
	router.GET("/system/version", h.getVersion)
	router.GET("/system/logs", h.getLogs)
}

// getLogs tails a log source: ?kind=service|upgrade&lines=N
func (h *SystemHandler) getLogs(c *gin.Context) {
	kind := c.DefaultQuery("kind", "service")
	lines := 200
	if raw := strings.TrimSpace(c.Query("lines")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			lines = n
		}
	}

	result, err := h.logs.Tail(c.Request.Context(), kind, lines)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c, result)
}

func (h *SystemHandler) getVersion(c *gin.Context) {
	respondOK(c, gin.H{
		"version": version.Version,
	})
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

func (h *SystemHandler) getDiskUsage(c *gin.Context) {
	// Prefer configured download cache (where free space actually matters).
	path := ""
	if h.store != nil {
		if cfg, err := h.store.LoadConfig(); err == nil && strings.TrimSpace(cfg.Download.Dir) != "" {
			path = cfg.Download.Dir
		} else if root := h.store.Root(); root != "" {
			path = root
		}
	}
	if path == "" {
		if envPath := strings.TrimSpace(os.Getenv("EMOSUP_DOWNLOADS_DIR")); envPath != "" {
			path = envPath
		} else if envPath := strings.TrimSpace(os.Getenv("EMOSUP_LOCAL_ROOT")); envPath != "" {
			path = envPath
		} else {
			path = "."
		}
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		respondError(c, http.StatusInternalServerError, "failed to get disk info")
		return
	}

	total := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bavail) * int64(stat.Bsize)
	used := total - free

	respondOK(c, gin.H{
		"path":       path,
		"total_bytes": total,
		"used_bytes":  used,
		"free_bytes":  free,
	})
}
