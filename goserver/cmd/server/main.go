package main

import (
	"log"
	"net/http"

	"emosup/goserver/internal/api"
	"emosup/goserver/internal/config"
	"emosup/goserver/internal/events"
	"emosup/goserver/internal/queue"
	"emosup/goserver/internal/store"
)

func main() {
	cfg := config.Load()
	eventBus := events.NewBus(128)
	taskStore := store.NewMemoryTaskStore()
	manager := queue.NewManager(taskStore, eventBus)
	handler := api.NewHandler(cfg, manager, eventBus)

	log.Printf("goserver listening on %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, handler.Routes()); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
