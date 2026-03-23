package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRouterServesFrontendIndexForSpaRoutes(t *testing.T) {
	distDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(distDir, "index.html"), []byte("<!doctype html><html><body>emosup</body></html>"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.Mkdir(filepath.Join(distDir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(distDir, "assets", "app.js"), []byte("console.log('ok')"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	router := NewRouter(RouterDependencies{
		Health:       NewHealthHandler(),
		System:       NewSystemHandler(nil),
		Config:       NewConfigHandler(nil),
		OpenList:     NewOpenListHandler(nil),
		Scan:         NewScanHandler(nil),
		Task:         NewTaskHandler(nil),
		FrontendDist: distDir,
	})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/tasks", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "emosup") {
		t.Fatalf("body = %q, want frontend index", recorder.Body.String())
	}

	assetReq := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	assetRecorder := httptest.NewRecorder()
	router.ServeHTTP(assetRecorder, assetReq)

	if assetRecorder.Code != http.StatusOK {
		t.Fatalf("asset status = %d, want %d", assetRecorder.Code, http.StatusOK)
	}
	if !strings.Contains(assetRecorder.Body.String(), "console.log") {
		t.Fatalf("asset body = %q, want asset content", assetRecorder.Body.String())
	}
}

func TestRouterRejectsPathTraversalOutsideFrontendDist(t *testing.T) {
	distDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(distDir, "index.html"), []byte("<!doctype html>"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.Mkdir(filepath.Join(distDir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}

	router := NewRouter(RouterDependencies{
		Health:       NewHealthHandler(),
		System:       NewSystemHandler(nil),
		Config:       NewConfigHandler(nil),
		OpenList:     NewOpenListHandler(nil),
		Scan:         NewScanHandler(nil),
		Task:         NewTaskHandler(nil),
		FrontendDist: distDir,
	})

	req := httptest.NewRequest(http.MethodGet, "/../../secret.txt", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
