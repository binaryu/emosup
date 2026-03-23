package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"emosup/backend/internal/service"
)

type ScanHandler struct {
	service *service.ScanService
}

func NewScanHandler(service *service.ScanService) *ScanHandler {
	return &ScanHandler{service: service}
}

func (h *ScanHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/scans", h.listScans)
	router.POST("/scans", h.createScan)
	router.GET("/scans/:id", h.getScan)
	router.PATCH("/scans/:scanId/items/:itemId", h.updateScanItem)
}

func (h *ScanHandler) listScans(c *gin.Context) {
	scans, err := h.service.ListScans(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, scans)
}

func (h *ScanHandler) createScan(c *gin.Context) {
	var req service.CreateScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	scan, err := h.service.CreateScan(c.Request.Context(), req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, scan)
}

func (h *ScanHandler) getScan(c *gin.Context) {
	scan, err := h.service.GetScan(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}

	respondOK(c, scan)
}

func (h *ScanHandler) updateScanItem(c *gin.Context) {
	var req service.UpdateScanItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	_, item, err := h.service.UpdateScanItem(c.Request.Context(), c.Param("scanId"), c.Param("itemId"), req)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondOK(c, item)
}
