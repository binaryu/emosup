package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"emosup/backend/internal/client"
	"emosup/backend/internal/eventbus"
	"emosup/backend/internal/handler"
	"emosup/backend/internal/scheduler"
	"emosup/backend/internal/service"
	"emosup/backend/internal/store"
)

type App struct {
	server    *http.Server
	scheduler *scheduler.Manager
	addr      string
	frontend  string
	dataRoot  string
}

func New() (*App, error) {
	// Production default: quiet Gin logs unless user overrides GIN_MODE.
	if strings.TrimSpace(os.Getenv("GIN_MODE")) == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	dataRoot := resolveDataRoot()
	fileStore := store.NewFileStore(dataRoot)
	if err := fileStore.Init(); err != nil {
		return nil, err
	}

	configService := service.NewConfigService(fileStore)
	authService := service.NewAuthService(fileStore)
	if err := authService.EnsureBootstrap(); err != nil {
		return nil, fmt.Errorf("bootstrap auth: %w", err)
	}

	openListClient := client.NewHTTPOpenListClient()
	emosClient := client.NewHTTPEmosClient()
	openListService := service.NewOpenListService(fileStore, openListClient)
	localService := service.NewLocalService(fileStore)
	emosService := service.NewEmosService(fileStore, emosClient)
	matchService := service.NewMatchService()
	scanService := service.NewScanService(fileStore, openListService, localService, emosService, matchService)

	aria2Client := client.NewHTTPAria2Client()
	taskService := service.NewTaskService(fileStore, aria2Client, openListClient)
	eventBus := eventbus.New()
	downloadExecutor := service.NewDownloadExecutor(taskService, aria2Client, openListClient, eventBus)
	uploadExecutor := service.NewUploadExecutor(taskService, emosClient, eventBus)
	tmdbClient := client.NewTMDBClient()

	// Load full config from store (not redacted) for scheduler settings.
	cfg, err := fileStore.LoadConfig()
	if err != nil {
		return nil, err
	}

	manager := scheduler.NewManager(
		taskService,
		downloadExecutor,
		uploadExecutor,
		time.Duration(cfg.Worker.PollIntervalSeconds)*time.Second,
		cfg.Worker.MaxConcurrency,
		eventBus,
	)

	frontendDist := findFrontendDistDir()
	router := handler.NewRouter(handler.RouterDependencies{
		Health:       handler.NewHealthHandler(),
		Auth:         handler.NewAuthHandler(authService),
		System:       handler.NewSystemHandler(manager, fileStore),
		Config:       handler.NewConfigHandler(configService),
		OpenList:     handler.NewOpenListHandler(openListService),
		Local:        handler.NewLocalHandler(localService),
		Emos:         handler.NewEmosHandler(emosService),
		Events:       handler.NewEventsHandler(eventBus),
		TMDB:         handler.NewTMDBHandler(tmdbClient, fileStore),
		Proxy:        handler.NewProxyHandler(fileStore, openListClient),
		Scan:         handler.NewScanHandler(scanService),
		Task:         handler.NewTaskHandler(taskService),
		FrontendDist: frontendDist,
	})

	host := strings.TrimSpace(cfg.Server.Host)
	if envHost := strings.TrimSpace(os.Getenv("EMOSUP_HOST")); envHost != "" {
		host = envHost
	}
	if host == "" {
		host = "0.0.0.0"
	}

	port := cfg.Server.Port
	if envPort := os.Getenv("EMOSUP_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil && p > 0 {
			port = p
		}
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	server := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return &App{
		server:    server,
		scheduler: manager,
		addr:      addr,
		frontend:  frontendDist,
		dataRoot:  dataRoot,
	}, nil
}

func (a *App) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go a.scheduler.Start(ctx)

	log.Printf("listening on http://%s  data=%s  frontend=%s", a.addr, a.dataRoot, a.frontendOrNone())
	err := a.server.ListenAndServe()
	cancel()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (a *App) frontendOrNone() string {
	if strings.TrimSpace(a.frontend) == "" {
		return "(not found — API only)"
	}
	return a.frontend
}
