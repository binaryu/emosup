package service

import (
	"context"
	"path/filepath"
	"testing"

	"emosup/backend/internal/model"
	"emosup/backend/internal/store"
)

func TestAuthLoginAndParseToken(t *testing.T) {
	root := t.TempDir()
	fileStore := store.NewFileStore(root)
	if err := fileStore.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}

	auth := NewAuthService(fileStore)
	if err := auth.EnsureBootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	// Default credentials after bootstrap.
	result, err := auth.Login(context.Background(), LoginRequest{
		Username: "admin",
		Password: "admin",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if result.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if result.Username != "admin" {
		t.Fatalf("username = %q, want admin", result.Username)
	}

	claims, err := auth.ParseToken(context.Background(), result.Token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.Username != "admin" {
		t.Fatalf("claims username = %q, want admin", claims.Username)
	}
}

func TestAuthLoginRejectsBadPassword(t *testing.T) {
	root := t.TempDir()
	fileStore := store.NewFileStore(filepath.Join(root, "data"))
	if err := fileStore.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}

	auth := NewAuthService(fileStore)
	if err := auth.EnsureBootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	_, err := auth.Login(context.Background(), LoginRequest{
		Username: "admin",
		Password: "wrong",
	})
	if err == nil {
		t.Fatal("expected error for bad password")
	}
	authErr, ok := AsAuthServiceError(err)
	if !ok || authErr.Code != 401 {
		t.Fatalf("expected 401 auth error, got %v", err)
	}
}

func TestMergeAuthOnSaveKeepsSecretsAndHashesPassword(t *testing.T) {
	existing := model.DefaultAppConfig()
	existing.Auth.Password = "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012"
	existing.Auth.JWTSecret = "existing-secret"
	existing.Auth.Username = "admin"

	incoming := model.DefaultAppConfig()
	incoming.Auth.Username = "ops"
	incoming.Auth.Password = "" // keep
	incoming.Auth.JWTSecret = "" // keep
	incoming.Auth.TokenTTLHours = 24

	merged, err := MergeAuthOnSave(existing, incoming)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if merged.Auth.Username != "ops" {
		t.Fatalf("username = %q, want ops", merged.Auth.Username)
	}
	if merged.Auth.Password != existing.Auth.Password {
		t.Fatal("password should be preserved when empty")
	}
	if merged.Auth.JWTSecret != existing.Auth.JWTSecret {
		t.Fatal("jwt secret should be preserved when empty")
	}
	if merged.Auth.TokenTTLHours != 24 {
		t.Fatalf("ttl = %d, want 24", merged.Auth.TokenTTLHours)
	}

	incoming.Auth.Password = "new-password"
	merged, err = MergeAuthOnSave(existing, incoming)
	if err != nil {
		t.Fatalf("merge with new password: %v", err)
	}
	if !isBcryptHash(merged.Auth.Password) {
		t.Fatalf("expected bcrypt hash, got %q", merged.Auth.Password)
	}
	if !verifyPassword(merged.Auth.Password, "new-password") {
		t.Fatal("hashed password should verify")
	}
}

func TestRedactAuthForResponse(t *testing.T) {
	cfg := model.DefaultAppConfig()
	cfg.Auth.Password = "secret"
	cfg.Auth.JWTSecret = "jwt-secret"
	redacted := RedactAuthForResponse(cfg)
	if redacted.Auth.Password != "" || redacted.Auth.JWTSecret != "" {
		t.Fatalf("secrets should be redacted: %+v", redacted.Auth)
	}
}
