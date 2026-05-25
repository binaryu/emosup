package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

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
}

func New() (*App, error) {
	dataRoot := resolveDataRoot()
	fileStore := store.NewFileStore(dataRoot)
	if err := fileStore.Init(); err != nil {
		return nil, err
	}

	configService := service.NewConfigService(fileStore)
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
	downloadExecutor := service.NewDownloadExecutor(taskService, aria2Client, eventBus)
	uploadExecutor := service.NewUploadExecutor(taskService, emosClient, eventBus)
	tmdbClient := client.NewTMDBClient()

	cfg, err := configService.GetConfig(context.Background())
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

	router := handler.NewRouter(handler.RouterDependencies{
		Health:       handler.NewHealthHandler(),
		System:       handler.NewSystemHandler(manager),
		Config:       handler.NewConfigHandler(configService),
		OpenList:     handler.NewOpenListHandler(openListService),
		Local:        handler.NewLocalHandler(localService),
		Emos:         handler.NewEmosHandler(emosService),
		Events:       handler.NewEventsHandler(eventBus),
		TMDB:         handler.NewTMDBHandler(tmdbClient, fileStore),
		Proxy:        handler.NewProxyHandler(fileStore, openListClient),
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
