package handler

import "github.com/gin-gonic/gin"

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/health", h.getHealth)
}

func (h *HealthHandler) getHealth(c *gin.Context) {
	respondOK(c, gin.H{
		"status": "ok",
		"scope":  "phase1-skeleton",
	})
}
