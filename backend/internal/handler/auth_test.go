package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"emosup/backend/internal/service"
	"emosup/backend/internal/store"
)

func TestAuthLoginAndProtectedRoute(t *testing.T) {
	root := t.TempDir()
	fileStore := store.NewFileStore(root)
	if err := fileStore.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}

	authService := service.NewAuthService(fileStore)
	if err := authService.EnsureBootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	authHandler := NewAuthHandler(authService)
	configHandler := NewConfigHandler(service.NewConfigService(fileStore))

	router := NewRouter(RouterDependencies{
		Health:   NewHealthHandler(),
		Auth:     authHandler,
		System:   NewSystemHandler(nil, nil),
		Config:   configHandler,
		OpenList: NewOpenListHandler(nil),
		Local:    NewLocalHandler(nil),
		Emos:     NewEmosHandler(nil),
		Events:   NewEventsHandler(nil),
		TMDB:     NewTMDBHandler(nil, nil),
		Proxy:    NewProxyHandler(nil, nil),
		Scan:     NewScanHandler(nil),
		Task:     NewTaskHandler(nil),
	})

	// Health stays public.
	healthReq := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	healthRec := httptest.NewRecorder()
	router.ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", healthRec.Code)
	}

	// Protected route without token.
	cfgReq := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	cfgRec := httptest.NewRecorder()
	router.ServeHTTP(cfgRec, cfgReq)
	if cfgRec.Code != http.StatusUnauthorized {
		t.Fatalf("config without token status = %d, want 401", cfgRec.Code)
	}

	// Login with default credentials.
	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "admin",
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", loginRec.Code, loginRec.Body.String())
	}

	var loginPayload struct {
		Success bool `json:"success"`
		Data    struct {
			Token    string `json:"token"`
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginPayload); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if !loginPayload.Success || loginPayload.Data.Token == "" {
		t.Fatalf("unexpected login payload: %s", loginRec.Body.String())
	}

	// Protected route with token.
	authCfgReq := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	authCfgReq.Header.Set("Authorization", "Bearer "+loginPayload.Data.Token)
	authCfgRec := httptest.NewRecorder()
	router.ServeHTTP(authCfgRec, authCfgReq)
	if authCfgRec.Code != http.StatusOK {
		t.Fatalf("config with token status = %d body=%s", authCfgRec.Code, authCfgRec.Body.String())
	}

	var cfgPayload struct {
		Success bool `json:"success"`
		Data    struct {
			Auth struct {
				Username  string `json:"username"`
				Password  string `json:"password"`
				JWTSecret string `json:"jwt_secret"`
			} `json:"auth"`
		} `json:"data"`
	}
	if err := json.Unmarshal(authCfgRec.Body.Bytes(), &cfgPayload); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfgPayload.Data.Auth.Username != "admin" {
		t.Fatalf("username = %q, want admin", cfgPayload.Data.Auth.Username)
	}
	if cfgPayload.Data.Auth.Password != "" || cfgPayload.Data.Auth.JWTSecret != "" {
		t.Fatalf("secrets must be redacted: %+v", cfgPayload.Data.Auth)
	}

	// /auth/me
	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginPayload.Data.Token)
	meRec := httptest.NewRecorder()
	router.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me status = %d body=%s", meRec.Code, meRec.Body.String())
	}
}
