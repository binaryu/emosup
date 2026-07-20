package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

type RouterDependencies struct {
	Health       *HealthHandler
	Auth         *AuthHandler
	System       *SystemHandler
	Config       *ConfigHandler
	OpenList     *OpenListHandler
	Local        *LocalHandler
	Emos         *EmosHandler
	Events       *EventsHandler
	TMDB         *TMDBHandler
	Proxy        *ProxyHandler
	Scan         *ScanHandler
	Task         *TaskHandler
	FrontendDist string
}

func NewRouter(deps RouterDependencies) *gin.Engine {
	router := gin.Default()

	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	api := router.Group("/api")
	// Public endpoints (no JWT required).
	deps.Health.RegisterRoutes(api)
	if deps.Auth != nil {
		deps.Auth.RegisterPublicRoutes(api)
		api.Use(deps.Auth.Middleware())
	}

	// Protected endpoints.
	if deps.Auth != nil {
		deps.Auth.RegisterRoutes(api)
	}
	deps.System.RegisterRoutes(api)
	deps.Config.RegisterRoutes(api)
	deps.OpenList.RegisterRoutes(api)
	deps.Local.RegisterRoutes(api)
	deps.Emos.RegisterRoutes(api)
	deps.Events.RegisterRoutes(api)
	deps.TMDB.RegisterRoutes(api)
	deps.Proxy.RegisterRoutes(api)
	deps.Scan.RegisterRoutes(api)
	deps.Task.RegisterRoutes(api)

	if deps.FrontendDist != "" {
		router.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
				return
			}

			cleanPath := filepath.Clean(strings.TrimPrefix(c.Request.URL.Path, "/"))
			if cleanPath == "." {
				c.File(filepath.Join(deps.FrontendDist, "index.html"))
				return
			}
			if cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
				c.JSON(http.StatusBadRequest, gin.H{"message": "invalid path"})
				return
			}

			target := filepath.Join(deps.FrontendDist, cleanPath)
			if info, err := os.Stat(target); err == nil && !info.IsDir() {
				c.File(target)
				return
			}

			c.File(filepath.Join(deps.FrontendDist, "index.html"))
		})
	}

	return router
}
