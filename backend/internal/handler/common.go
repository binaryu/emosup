package handler

import "github.com/gin-gonic/gin"

type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func respondOK(c *gin.Context, data any) {
	c.JSON(200, APIResponse{
		Success: true,
		Data:    data,
	})
}

func respondError(c *gin.Context, code int, message string) {
	c.JSON(code, APIResponse{
		Success: false,
		Message: message,
	})
}
