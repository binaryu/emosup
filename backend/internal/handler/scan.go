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
	router.DELETE("/scans/:id", h.deleteScan)
	router.PATCH("/scans/:id/items/:itemId", h.updateScanItem)
	router.DELETE("/scans/:id/items/:itemId", h.deleteScanItem)
	router.DELETE("/scans/:id/items", h.deleteScanItems)
}

type DeleteScanItemsRequest struct {
	ItemIDs []string `json:"item_ids"`
}

func (h *ScanHandler) deleteScanItems(c *gin.Context) {
	var req DeleteScanItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	scan, err := h.service.DeleteScanItems(c.Request.Context(), c.Param("id"), req.ItemIDs)
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}

	respondOK(c, scan)
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

func (h *ScanHandler) deleteScan(c *gin.Context) {
	err := h.service.DeleteScan(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}

	respondOK(c, gin.H{"deleted": true})
}

func (h *ScanHandler) updateScanItem(c *gin.Context) {
	var req service.UpdateScanItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	_, item, err := h.service.UpdateScanItem(c.Request.Context(), c.Param("id"), c.Param("itemId"), req)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondOK(c, item)
}

func (h *ScanHandler) deleteScanItem(c *gin.Context) {
	scan, err := h.service.DeleteScanItem(c.Request.Context(), c.Param("id"), c.Param("itemId"))
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}

	respondOK(c, scan)
}
