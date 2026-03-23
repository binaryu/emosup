package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"emosup/backend/internal/client"
	"emosup/backend/internal/handler"
	"emosup/backend/internal/scheduler"
	"emosup/backend/internal/service"
	"emosup/backend/internal/store"
)

type App struct {
	server    *http.Server
	scheduler *scheduler.Manager
}

func findFrontendDistDir() string {
	candidates := []string{
		os.Getenv("EMOSUP_FRONTEND_DIST"),
		filepath.Join("..", "frontend", "dist"),
		filepath.Join("frontend", "dist"),
		filepath.Join("..", "frontend"),
		filepath.Join("frontend"),
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		indexInfo, indexErr := os.Stat(filepath.Join(candidate, "index.html"))
		assetsInfo, assetsErr := os.Stat(filepath.Join(candidate, "assets"))
		if indexErr == nil && assetsErr == nil && !indexInfo.IsDir() && assetsInfo.IsDir() {
			return candidate
		}
	}

	return ""
}

func New() (*App, error) {
	dataRoot := filepath.Join(".", "data")
	fileStore := store.NewFileStore(dataRoot)
	if err := fileStore.Init(); err != nil {
		return nil, err
	}

	configService := service.NewConfigService(fileStore)
	openListClient := client.NewHTTPOpenListClient()
	emosClient := client.NewHTTPEmosClient()
	openListService := service.NewOpenListService(fileStore, openListClient)
	emosService := service.NewEmosService(fileStore, emosClient)
	matchService := service.NewMatchService()
	scanService := service.NewScanService(fileStore, openListService, emosService, matchService)

	aria2Client := client.NewHTTPAria2Client()
	taskService := service.NewTaskService(fileStore, aria2Client, openListClient)
	downloadExecutor := service.NewDownloadExecutor(taskService, aria2Client)
	uploadExecutor := service.NewUploadExecutor(taskService, emosClient)

	cfg, err := configService.GetConfig(context.Background())
	if err != nil {
		return nil, err
	}

	manager := scheduler.NewManager(
		taskService,
		downloadExecutor,
		uploadExecutor,
		time.Duration(cfg.Worker.PollIntervalSeconds)*time.Second,
	)

	router := handler.NewRouter(handler.RouterDependencies{
		Health:       handler.NewHealthHandler(),
		System:       handler.NewSystemHandler(manager),
		Config:       handler.NewConfigHandler(configService),
		OpenList:     handler.NewOpenListHandler(openListService),
		Scan:         handler.NewScanHandler(scanService),
		Task:         handler.NewTaskHandler(taskService),
		FrontendDist: findFrontendDistDir(),
	})

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return &App{
		server:    server,
		scheduler: manager,
	}, nil
}

func (a *App) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go a.scheduler.Start(ctx)

	err := a.server.ListenAndServe()
	cancel()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
