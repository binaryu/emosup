package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"emosup/backend/internal/service"
)

const contextUsernameKey = "auth_username"

type AuthHandler struct {
	service *service.AuthService
}

func NewAuthHandler(service *service.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

func (h *AuthHandler) RegisterPublicRoutes(router *gin.RouterGroup) {
	router.POST("/auth/login", h.login)
}

func (h *AuthHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/auth/me", h.me)
}

func (h *AuthHandler) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.service == nil {
			respondError(c, http.StatusServiceUnavailable, "auth is not configured")
			c.Abort()
			return
		}

		token := extractBearerToken(c)
		if token == "" {
			// EventSource cannot set Authorization headers; allow ?token= for SSE.
			token = strings.TrimSpace(c.Query("token"))
		}
		if token == "" {
			respondError(c, http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}

		claims, err := h.service.ParseToken(c.Request.Context(), token)
		if err != nil {
			if authErr, ok := service.AsAuthServiceError(err); ok {
				respondError(c, authErr.Code, authErr.Message)
			} else {
				respondError(c, http.StatusUnauthorized, "unauthorized")
			}
			c.Abort()
			return
		}

		c.Set(contextUsernameKey, claims.Username)
		c.Next()
	}
}

func (h *AuthHandler) login(c *gin.Context) {
	if h.service == nil {
		respondError(c, http.StatusServiceUnavailable, "auth is not configured")
		return
	}

	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		if authErr, ok := service.AsAuthServiceError(err); ok {
			respondError(c, authErr.Code, authErr.Message)
			return
		}
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, result)
}

func (h *AuthHandler) me(c *gin.Context) {
	username, _ := c.Get(contextUsernameKey)
	respondOK(c, gin.H{
		"username": username,
	})
}

func extractBearerToken(c *gin.Context) string {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if header == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
