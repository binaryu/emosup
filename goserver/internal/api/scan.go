package api

import (
	"encoding/json"
	"net/http"
	"time"

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

func (h *Handler) handleScanLocal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req domain.ScanLocalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.LocalPath == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "local_path is required"})
		return
	}

	files, err := h.scanService.WalkLocal(req.LocalPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

func (h *Handler) handleScanCombined(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req domain.ScanCombinedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.RootPath == "" && req.LocalPath == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "root_path or local_path is required"})
		return
	}

	remoteFiles := make([]domain.ScanItem, 0)
	localFiles := make([]domain.ScanItem, 0)
	var err error

	if req.RootPath != "" {
		remoteFiles, err = h.scanService.WalkVideos(domain.ScanRemoteRequest{
			RootPath:        req.RootPath,
			OpenListBaseURL: req.OpenListBaseURL,
			OpenListToken:   req.OpenListToken,
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
	}

	if req.LocalPath != "" {
		localFiles, err = h.scanService.WalkLocal(req.LocalPath)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	}

	files := h.scanService.MergeFiles(remoteFiles, localFiles)
	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

func (h *Handler) handlePrecheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req domain.PrecheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if len(req.Files) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "files is required"})
		return
	}
	if req.TMDBID == 0 || req.EmosAPIBase == "" || req.EmosToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tmdb_id, emos_api_base and emos_token are required"})
		return
	}

	tree, err := h.emosService.GetTreeByTMDB(req.EmosAPIBase, req.EmosToken, req.TMDBID, 180*time.Second)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if tree == nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "无法获取 video/tree，请确认 tmdb_id / token"})
		return
	}

	resp := h.scanService.Precheck(req, *tree)
	writeJSON(w, http.StatusOK, resp)
}
