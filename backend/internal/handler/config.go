package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"emosup/backend/internal/model"
	"emosup/backend/internal/service"
)

type ConfigHandler struct {
	service *service.ConfigService
}

func NewConfigHandler(service *service.ConfigService) *ConfigHandler {
	return &ConfigHandler{service: service}
}

func (h *ConfigHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/config", h.getConfig)
	router.PUT("/config", h.saveConfig)
	router.POST("/config/validate", h.validateConfig)
}

func (h *ConfigHandler) getConfig(c *gin.Context) {
	cfg, err := h.service.GetConfig(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, cfg)
}

func (h *ConfigHandler) saveConfig(c *gin.Context) {
	var req model.AppConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	cfg, err := h.service.SaveConfig(c.Request.Context(), req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, cfg)
}

func (h *ConfigHandler) validateConfig(c *gin.Context) {
	respondOK(c, gin.H{
		"openlist": "pending",
		"download": "pending",
		"emos":     "pending",
		"message":  "第一阶段仅保留接口与状态位，后续补充真实连通性检查。",
	})
}
