package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"emosup/backend/internal/model"
	"emosup/backend/internal/store"
)

const (
	jwtIssuer          = "emosup"
	defaultTokenTTL    = 72 * time.Hour
	bcryptCost         = bcrypt.DefaultCost
	passwordHashPrefix = "$2"
)

type AuthService struct {
	store *store.FileStore
}

type AuthServiceError struct {
	Code    int
	Message string
}

func (e *AuthServiceError) Error() string {
	return e.Message
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResult struct {
	Token     string    `json:"token"`
	TokenType string    `json:"token_type"`
	ExpiresAt time.Time `json:"expires_at"`
	Username  string    `json:"username"`
}

type AuthClaims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func NewAuthService(store *store.FileStore) *AuthService {
	return &AuthService{store: store}
}

func (s *AuthService) Login(_ context.Context, req LoginRequest) (LoginResult, error) {
	username := strings.TrimSpace(req.Username)
	password := req.Password
	if username == "" || password == "" {
		return LoginResult{}, newAuthServiceError(http.StatusBadRequest, "username and password are required")
	}

	cfg, err := s.store.LoadConfig()
	if err != nil {
		return LoginResult{}, err
	}

	if !constantTimeEqual(username, cfg.Auth.Username) || !verifyPassword(cfg.Auth.Password, password) {
		return LoginResult{}, newAuthServiceError(http.StatusUnauthorized, "invalid username or password")
	}

	secret, err := s.ensureJWTSecret(&cfg)
	if err != nil {
		return LoginResult{}, err
	}

	ttl := time.Duration(cfg.Auth.TokenTTLHours) * time.Hour
	if ttl <= 0 {
		ttl = defaultTokenTTL
	}
	expiresAt := time.Now().Add(ttl)

	claims := AuthClaims{
		Username: cfg.Auth.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   cfg.Auth.Username,
			Issuer:    jwtIssuer,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return LoginResult{}, fmt.Errorf("sign token: %w", err)
	}

	return LoginResult{
		Token:     signed,
		TokenType: "Bearer",
		ExpiresAt: expiresAt,
		Username:  cfg.Auth.Username,
	}, nil
}

func (s *AuthService) ParseToken(_ context.Context, tokenString string) (*AuthClaims, error) {
	tokenString = strings.TrimSpace(tokenString)
	if tokenString == "" {
		return nil, newAuthServiceError(http.StatusUnauthorized, "missing token")
	}

	cfg, err := s.store.LoadConfig()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		return nil, newAuthServiceError(http.StatusUnauthorized, "auth is not configured")
	}

	claims := &AuthClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.Auth.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, newAuthServiceError(http.StatusUnauthorized, "invalid or expired token")
	}

	if claims.Username == "" {
		claims.Username = claims.Subject
	}
	if claims.Username == "" {
		return nil, newAuthServiceError(http.StatusUnauthorized, "invalid token claims")
	}

	return claims, nil
}

func (s *AuthService) CurrentUser(ctx context.Context, tokenString string) (map[string]string, error) {
	claims, err := s.ParseToken(ctx, tokenString)
	if err != nil {
		return nil, err
	}
	return map[string]string{"username": claims.Username}, nil
}

// EnsureBootstrap prepares auth defaults (jwt secret, password hash) and persists if needed.
func (s *AuthService) EnsureBootstrap() error {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return err
	}

	changed := false

	if strings.TrimSpace(cfg.Auth.Username) == "" {
		cfg.Auth.Username = "admin"
		changed = true
	}
	if strings.TrimSpace(cfg.Auth.Password) == "" {
		cfg.Auth.Password = "admin"
		changed = true
	}
	if !isBcryptHash(cfg.Auth.Password) {
		hashed, err := hashPassword(cfg.Auth.Password)
		if err != nil {
			return err
		}
		cfg.Auth.Password = hashed
		changed = true
	}
	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		secret, err := generateJWTSecret()
		if err != nil {
			return err
		}
		cfg.Auth.JWTSecret = secret
		changed = true
	}
	if cfg.Auth.TokenTTLHours <= 0 {
		cfg.Auth.TokenTTLHours = 72
		changed = true
	}

	if changed {
		return s.store.SaveConfig(cfg)
	}
	return nil
}

func (s *AuthService) ensureJWTSecret(cfg *model.AppConfig) (string, error) {
	if strings.TrimSpace(cfg.Auth.JWTSecret) != "" {
		return cfg.Auth.JWTSecret, nil
	}
	secret, err := generateJWTSecret()
	if err != nil {
		return "", err
	}
	cfg.Auth.JWTSecret = secret
	if err := s.store.SaveConfig(*cfg); err != nil {
		return "", err
	}
	return secret, nil
}

// MergeAuthOnSave keeps existing password/jwt_secret when the client sends empty values,
// and hashes plaintext passwords before persistence.
func MergeAuthOnSave(existing, incoming model.AppConfig) (model.AppConfig, error) {
	result := incoming

	if strings.TrimSpace(result.Auth.Username) == "" {
		result.Auth.Username = existing.Auth.Username
	}
	if result.Auth.TokenTTLHours <= 0 {
		result.Auth.TokenTTLHours = existing.Auth.TokenTTLHours
		if result.Auth.TokenTTLHours <= 0 {
			result.Auth.TokenTTLHours = 72
		}
	}

	// Empty password from UI means "keep current".
	if strings.TrimSpace(result.Auth.Password) == "" {
		result.Auth.Password = existing.Auth.Password
	} else if !isBcryptHash(result.Auth.Password) {
		hashed, err := hashPassword(result.Auth.Password)
		if err != nil {
			return model.AppConfig{}, err
		}
		result.Auth.Password = hashed
	}

	// Empty jwt_secret from UI means "keep current"; never accept empty after merge.
	if strings.TrimSpace(result.Auth.JWTSecret) == "" {
		result.Auth.JWTSecret = existing.Auth.JWTSecret
	}
	if strings.TrimSpace(result.Auth.JWTSecret) == "" {
		secret, err := generateJWTSecret()
		if err != nil {
			return model.AppConfig{}, err
		}
		result.Auth.JWTSecret = secret
	}

	return result, nil
}

// RedactAuthForResponse strips secrets that must not leave the server.
func RedactAuthForResponse(cfg model.AppConfig) model.AppConfig {
	cfg.Auth.Password = ""
	cfg.Auth.JWTSecret = ""
	return cfg
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func verifyPassword(stored, provided string) bool {
	if isBcryptHash(stored) {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(provided)) == nil
	}
	// Legacy plaintext password in config (migrated on next bootstrap/save).
	return constantTimeEqual(stored, provided)
}

func isBcryptHash(value string) bool {
	return strings.HasPrefix(value, passwordHashPrefix)
}

func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func generateJWTSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func newAuthServiceError(code int, message string) *AuthServiceError {
	return &AuthServiceError{Code: code, Message: message}
}

func AsAuthServiceError(err error) (*AuthServiceError, bool) {
	var target *AuthServiceError
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}
