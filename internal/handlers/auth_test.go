package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agpt-go/chatbot-api/internal/database"
	"github.com/agpt-go/chatbot-api/internal/services"
)

// mockAuthService implements AuthServicer for testing
type mockAuthService struct {
	registerFunc         func(ctx context.Context, email, password, name string) (*database.User, *services.TokenPair, error)
	loginFunc            func(ctx context.Context, email, password string) (*database.User, *services.TokenPair, error)
	refreshTokensFunc    func(ctx context.Context, refreshToken string) (*services.TokenPair, error)
	logoutFunc           func(ctx context.Context, refreshToken string) error
	validateAccessToken  func(tokenString string) (*services.Claims, error)
	generateOAuthState   func(ctx context.Context) (string, error)
	validateOAuthState   func(ctx context.Context, state string) error
	getGoogleAuthURL     func(state string) string
	handleGoogleCallback func(ctx context.Context, code string) (*database.User, *services.TokenPair, error)
}

func (m *mockAuthService) Register(ctx context.Context, email, password, name string) (*database.User, *services.TokenPair, error) {
	if m.registerFunc != nil {
		return m.registerFunc(ctx, email, password, name)
	}
	return nil, nil, errors.New("not implemented")
}

func (m *mockAuthService) Login(ctx context.Context, email, password string) (*database.User, *services.TokenPair, error) {
	if m.loginFunc != nil {
		return m.loginFunc(ctx, email, password)
	}
	return nil, nil, services.ErrInvalidCredentials
}

func (m *mockAuthService) RefreshTokens(ctx context.Context, refreshToken string) (*services.TokenPair, error) {
	if m.refreshTokensFunc != nil {
		return m.refreshTokensFunc(ctx, refreshToken)
	}
	return nil, services.ErrInvalidToken
}

func (m *mockAuthService) Logout(ctx context.Context, refreshToken string) error {
	if m.logoutFunc != nil {
		return m.logoutFunc(ctx, refreshToken)
	}
	return nil
}

func (m *mockAuthService) ValidateAccessToken(tokenString string) (*services.Claims, error) {
	if m.validateAccessToken != nil {
		return m.validateAccessToken(tokenString)
	}
	return nil, services.ErrInvalidToken
}

func (m *mockAuthService) GenerateOAuthState(ctx context.Context) (string, error) {
	if m.generateOAuthState != nil {
		return m.generateOAuthState(ctx)
	}
	return "mock-state-12345678901234567890123456789012", nil
}

func (m *mockAuthService) ValidateOAuthState(ctx context.Context, state string) error {
	if m.validateOAuthState != nil {
		return m.validateOAuthState(ctx, state)
	}
	return services.ErrInvalidOAuthState
}

func (m *mockAuthService) GetGoogleAuthURL(state string) string {
	if m.getGoogleAuthURL != nil {
		return m.getGoogleAuthURL(state)
	}
	return "" // Not configured
}

func (m *mockAuthService) HandleGoogleCallback(ctx context.Context, code string) (*database.User, *services.TokenPair, error) {
	if m.handleGoogleCallback != nil {
		return m.handleGoogleCallback(ctx, code)
	}
	return nil, nil, errors.New("google oauth not configured")
}

func createTestAuthHandler(t *testing.T) *AuthHandler {
	t.Helper()
	return NewAuthHandler(&mockAuthService{}, nil)
}

func TestNewAuthHandler(t *testing.T) {
	mockService := &mockAuthService{}
	handler := NewAuthHandler(mockService, nil)

	if handler == nil {
		t.Fatal("NewAuthHandler() returned nil")
	}
	if handler.authService != mockService {
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

// Note: TestLogoutHandler_Success is skipped because it requires a database connection
// to actually revoke tokens. This would be tested in integration tests.

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

// Note: OAuth state generation has moved to AuthService.GenerateOAuthState
// which is tested in services/auth_test.go
