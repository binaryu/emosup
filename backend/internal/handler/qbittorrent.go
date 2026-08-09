package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"emosup/backend/internal/service"
)

type QBittorrentHandler struct {
	service *service.QBittorrentService
}

func NewQBittorrentHandler(service *service.QBittorrentService) *QBittorrentHandler {
	return &QBittorrentHandler{service: service}
}

func (h *QBittorrentHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/qbittorrent/torrents", h.listTorrents)
	router.POST("/qbittorrent/torrents", h.addTorrents)
	router.GET("/qbittorrent/torrents/:hash/files", h.listTorrentFiles)
	router.POST("/qbittorrent/torrents/pause", h.pauseTorrents)
	router.POST("/qbittorrent/torrents/resume", h.resumeTorrents)
	router.DELETE("/qbittorrent/torrents", h.deleteTorrents)
	router.POST("/qbittorrent/torrents/:hash/scan", h.scanTorrent)
	router.POST("/qbittorrent/test", h.testConnection)
}

func (h *QBittorrentHandler) testConnection(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if err := h.service.TestConnection(ctx); err != nil {
		respondError(c, http.StatusBadGateway, err.Error())
		return
	}
	respondOK(c, gin.H{"ok": true})
}

func (h *QBittorrentHandler) listTorrents(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	torrents, err := h.service.Torrents(ctx)
	if err != nil {
		respondError(c, http.StatusBadGateway, err.Error())
		return
	}
	respondOK(c, torrents)
}

func (h *QBittorrentHandler) addTorrents(c *gin.Context) {
	var req struct {
		URLs []string `json:"urls"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()

	torrents, err := h.service.AddTorrents(ctx, req.URLs)
	if err != nil {
		respondError(c, http.StatusBadGateway, err.Error())
		return
	}
	respondOK(c, torrents)
}

func (h *QBittorrentHandler) listTorrentFiles(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	files, err := h.service.TorrentFiles(ctx, c.Param("hash"))
	if err != nil {
		respondError(c, http.StatusBadGateway, err.Error())
		return
	}
	respondOK(c, files)
}

func (h *QBittorrentHandler) pauseTorrents(c *gin.Context) {
	h.actionTorrents(c, "pause")
}

func (h *QBittorrentHandler) resumeTorrents(c *gin.Context) {
	h.actionTorrents(c, "resume")
}

func (h *QBittorrentHandler) actionTorrents(c *gin.Context, action string) {
	var req struct {
		Hashes []string `json:"hashes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	var err error
	if action == "pause" {
		err = h.service.Pause(ctx, req.Hashes)
	} else {
		err = h.service.Resume(ctx, req.Hashes)
	}
	if err != nil {
		respondError(c, http.StatusBadGateway, err.Error())
		return
	}
	respondOK(c, gin.H{"ok": true})
}

func (h *QBittorrentHandler) deleteTorrents(c *gin.Context) {
	var req struct {
		Hashes      []string `json:"hashes"`
		DeleteFiles bool     `json:"delete_files"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if err := h.service.Delete(ctx, req.Hashes, req.DeleteFiles); err != nil {
		respondError(c, http.StatusBadGateway, err.Error())
		return
	}
	respondOK(c, gin.H{"deleted": true})
}

func (h *QBittorrentHandler) scanTorrent(c *gin.Context) {
	var req struct {
		TMDBID    int64  `json:"tmdb_id"`
		VideoType string `json:"video_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	scan, err := h.service.ScanTorrent(ctx, c.Param("hash"), req.TMDBID, req.VideoType)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	respondOK(c, scan)
}
