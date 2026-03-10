package api

import (
	"encoding/json"
	"net/http"

	"emosup/goserver/internal/config"
	"emosup/goserver/internal/events"
	"emosup/goserver/internal/queue"
	"emosup/goserver/internal/service"
)

type Handler struct {
	cfg         config.Config
	manager     *queue.Manager
	bus         *events.Bus
	scanService *service.ScanService
}

func NewHandler(cfg config.Config, manager *queue.Manager, bus *events.Bus) *Handler {
	return &Handler{
		cfg:         cfg,
		manager:     manager,
		bus:         bus,
		scanService: service.NewScanService(),
	}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.handleIndex)
	mux.HandleFunc("/healthz", h.handleHealth)
	mux.HandleFunc("/api/tasks", h.handleTasks)
	mux.HandleFunc("/api/queue/add", h.handleEnqueue)
	mux.HandleFunc("/api/scan_remote", h.handleScanRemote)
	mux.HandleFunc("/api/precheck", h.handlePrecheck)
	mux.HandleFunc("/api/events", h.handleEvents)
	return mux
}

func (h *Handler) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"addr":   h.cfg.HTTPAddr,
	})
}

func (h *Handler) handleTasks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.manager.Snapshot())
}

func (h *Handler) handleEnqueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req queue.EnqueueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	task := h.manager.Enqueue(req)
	writeJSON(w, http.StatusAccepted, task)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
