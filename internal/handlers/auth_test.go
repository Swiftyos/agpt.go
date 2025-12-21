package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agpt-go/chatbot-api/internal/config"
	"github.com/agpt-go/chatbot-api/internal/services"
)

func createTestAuthHandler(t *testing.T) *AuthHandler {
	t.Helper()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:           "test-secret-key-for-testing-minimum-32-chars",
			AccessExpiresIn:  15 * time.Minute,
			RefreshExpiresIn: 7 * 24 * time.Hour,
			Issuer:           "test-issuer",
		},
	}
	authService := services.NewAuthService(nil, cfg)
	return NewAuthHandler(authService)
}

func TestNewAuthHandler(t *testing.T) {
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret: "test-secret",
		},
	}
	authService := services.NewAuthService(nil, cfg)
	handler := NewAuthHandler(authService)

	if handler == nil {
		t.Fatal("NewAuthHandler() returned nil")
	}
	if handler.authService != authService {
		t.Error("NewAuthHandler() did not set authService correctly")
	}
	if handler.validate == nil {
		t.Error("NewAuthHandler() did not set validator")
	}
}

func TestRegisterHandler_InvalidJSON(t *testing.T) {
	handler := createTestAuthHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRegisterHandler_ValidationErrors(t *testing.T) {
	handler := createTestAuthHandler(t)

	tests := []struct {
		name    string
		payload RegisterRequest
	}{
		{
			name:    "missing email",
			payload: RegisterRequest{Password: "password123", Name: "Test"},
		},
		{
			name:    "invalid email",
			payload: RegisterRequest{Email: "not-an-email", Password: "password123", Name: "Test"},
		},
		{
			name:    "password too short",
			payload: RegisterRequest{Email: "test@example.com", Password: "short", Name: "Test"},
		},
		{
			name:    "missing name",
			payload: RegisterRequest{Email: "test@example.com", Password: "password123"},
		},
		{
			name:    "name too short",
			payload: RegisterRequest{Email: "test@example.com", Password: "password123", Name: "A"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.Register(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestLoginHandler_InvalidJSON(t *testing.T) {
	handler := createTestAuthHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLoginHandler_ValidationErrors(t *testing.T) {
	handler := createTestAuthHandler(t)

	tests := []struct {
		name    string
		payload LoginRequest
	}{
		{
			name:    "missing email",
			payload: LoginRequest{Password: "password123"},
		},
		{
			name:    "invalid email",
			payload: LoginRequest{Email: "not-an-email", Password: "password123"},
		},
		{
			name:    "missing password",
			payload: LoginRequest{Email: "test@example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.Login(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestRefreshHandler_InvalidJSON(t *testing.T) {
	handler := createTestAuthHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRefreshHandler_MissingToken(t *testing.T) {
	handler := createTestAuthHandler(t)

	payload := RefreshRequest{RefreshToken: ""}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLogoutHandler_InvalidJSON(t *testing.T) {
	handler := createTestAuthHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Logout(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLogoutHandler_Success(t *testing.T) {
	handler := createTestAuthHandler(t)

	payload := RefreshRequest{RefreshToken: "some-token"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Logout(rec, req)

	// Logout should succeed even if token doesn't exist
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGoogleLoginHandler_NotConfigured(t *testing.T) {
	handler := createTestAuthHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google", nil)
	rec := httptest.NewRecorder()

	handler.GoogleLogin(rec, req)

	// Should return 501 when Google OAuth is not configured
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotImplemented)
	}
}

func TestGoogleCallbackHandler_MissingCode(t *testing.T) {
	handler := createTestAuthHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback", nil)
	rec := httptest.NewRecorder()

	handler.GoogleCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRegisterRequestStruct(t *testing.T) {
	req := RegisterRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "Test User",
	}

	if req.Email != "test@example.com" {
		t.Errorf("RegisterRequest.Email = %q, want %q", req.Email, "test@example.com")
	}
	if req.Password != "password123" {
		t.Errorf("RegisterRequest.Password = %q, want %q", req.Password, "password123")
	}
	if req.Name != "Test User" {
		t.Errorf("RegisterRequest.Name = %q, want %q", req.Name, "Test User")
	}
}

func TestLoginRequestStruct(t *testing.T) {
	req := LoginRequest{
		Email:    "test@example.com",
		Password: "password123",
	}

	if req.Email != "test@example.com" {
		t.Errorf("LoginRequest.Email = %q, want %q", req.Email, "test@example.com")
	}
	if req.Password != "password123" {
		t.Errorf("LoginRequest.Password = %q, want %q", req.Password, "password123")
	}
}

func TestRefreshRequestStruct(t *testing.T) {
	req := RefreshRequest{
		RefreshToken: "refresh-token-123",
	}

	if req.RefreshToken != "refresh-token-123" {
		t.Errorf("RefreshRequest.RefreshToken = %q, want %q", req.RefreshToken, "refresh-token-123")
	}
}

func TestUserResponseStruct(t *testing.T) {
	avatarURL := "https://example.com/avatar.jpg"
	resp := UserResponse{
		ID:            "user-123",
		Email:         "test@example.com",
		Name:          "Test User",
		AvatarURL:     &avatarURL,
		Provider:      "local",
		EmailVerified: true,
	}

	if resp.ID != "user-123" {
		t.Errorf("UserResponse.ID = %q, want %q", resp.ID, "user-123")
	}
	if resp.Email != "test@example.com" {
		t.Errorf("UserResponse.Email = %q, want %q", resp.Email, "test@example.com")
	}
	if resp.Name != "Test User" {
		t.Errorf("UserResponse.Name = %q, want %q", resp.Name, "Test User")
	}
	if *resp.AvatarURL != avatarURL {
		t.Errorf("UserResponse.AvatarURL = %q, want %q", *resp.AvatarURL, avatarURL)
	}
	if resp.Provider != "local" {
		t.Errorf("UserResponse.Provider = %q, want %q", resp.Provider, "local")
	}
	if !resp.EmailVerified {
		t.Error("UserResponse.EmailVerified should be true")
	}
}

func TestGenerateState(t *testing.T) {
	state1 := generateState()
	state2 := generateState()

	if state1 == "" {
		t.Error("generateState() returned empty string")
	}
	if len(state1) != 32 { // 16 bytes * 2 (hex encoding)
		t.Errorf("generateState() length = %d, want 32", len(state1))
	}
	if state1 == state2 {
		t.Error("generateState() should return unique values")
	}
}
