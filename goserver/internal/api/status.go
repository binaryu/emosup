package api

import "net/http"

func (h *Handler) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"logs": h.manager.LogsTail(80),
		"task": h.manager.Snapshot(),
	})
}

func (h *Handler) handleQueueStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.manager.Snapshot())
}
