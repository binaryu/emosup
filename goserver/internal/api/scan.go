package api

import (
	"encoding/json"
	"net/http"

	"emosup/goserver/internal/domain"
)

func (h *Handler) handleScanRemote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req domain.ScanRemoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.RootPath == "" || req.OpenListBaseURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "root_path and openlist_base_url are required"})
		return
	}

	files, err := h.scanService.WalkVideos(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

func (h *Handler) handlePrecheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		domain.PrecheckRequest
		Tree domain.TreeInfo `json:"tree"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if len(req.Files) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "files is required"})
		return
	}

	resp := h.scanService.Precheck(req.PrecheckRequest, req.Tree)
	writeJSON(w, http.StatusOK, resp)
}
