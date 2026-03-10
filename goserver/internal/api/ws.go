package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"emosup/goserver/internal/domain"
)

func (h *Handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := h.bus.Subscribe()
	defer h.bus.Unsubscribe(ch)

	h.writeSSE(w, domain.Event{
		Type:      domain.EventSnapshot,
		Snapshot:  ptrSnapshot(h.manager.Snapshot()),
		Timestamp: h.manager.Snapshot().UpdatedAt,
	})
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-ch:
			h.writeSSE(w, event)
			flusher.Flush()
		}
	}
}

func (h *Handler) writeSSE(w http.ResponseWriter, event domain.Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}

func ptrSnapshot(s domain.Snapshot) *domain.Snapshot {
	return &s
}
