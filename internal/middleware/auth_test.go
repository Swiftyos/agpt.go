package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agpt-go/chatbot-api/internal/config"
	"github.com/agpt-go/chatbot-api/internal/services"
	"github.com/google/uuid"
)

func createTestAuthService(t *testing.T) *services.AuthService {
	t.Helper()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:           "test-secret-key-for-testing-minimum-32-chars",
			AccessExpiresIn:  15 * time.Minute,
			RefreshExpiresIn: 7 * 24 * time.Hour,
			Issuer:           "test-issuer",
		},
	}
	// Create auth service with nil queries - we only need token validation for middleware tests
	return services.NewAuthService(nil, cfg)
}

func TestRequireAuth(t *testing.T) {
	authService := createTestAuthService(t)
	middleware := NewAuthMiddleware(authService)

	t.Run("missing authorization header", func(t *testing.T) {
		handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called")
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("invalid authorization header format", func(t *testing.T) {
		handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called")
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called")
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})
}

func TestOptionalAuth(t *testing.T) {
	authService := createTestAuthService(t)
	middleware := NewAuthMiddleware(authService)

	t.Run("no authorization header - passes through", func(t *testing.T) {
		handlerCalled := false
		handler := middleware.OptionalAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			// Should not have user ID in context
			userID := GetUserID(r.Context())
			if userID != uuid.Nil {
				t.Error("userID should be nil when no auth header")
			}
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if !handlerCalled {
			t.Error("handler should be called")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("invalid auth header - passes through without user context", func(t *testing.T) {
		handlerCalled := false
		handler := middleware.OptionalAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			userID := GetUserID(r.Context())
			if userID != uuid.Nil {
				t.Error("userID should be nil for invalid token")
			}
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if !handlerCalled {
			t.Error("handler should be called")
		}
	})

	t.Run("invalid format - passes through", func(t *testing.T) {
		handlerCalled := false
		handler := middleware.OptionalAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Basic credentials")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if !handlerCalled {
			t.Error("handler should be called")
		}
	})
}

func TestGetUserID(t *testing.T) {
	t.Run("with user ID in context", func(t *testing.T) {
		expectedID := uuid.New()
		ctx := context.WithValue(context.Background(), UserIDKey, expectedID)

		got := GetUserID(ctx)
		if got != expectedID {
			t.Errorf("GetUserID() = %v, want %v", got, expectedID)
		}
	})

	t.Run("without user ID in context", func(t *testing.T) {
		ctx := context.Background()

		got := GetUserID(ctx)
		if got != uuid.Nil {
			t.Errorf("GetUserID() = %v, want %v", got, uuid.Nil)
		}
	})

	t.Run("with wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), UserIDKey, "not-a-uuid")

		got := GetUserID(ctx)
		if got != uuid.Nil {
			t.Errorf("GetUserID() = %v, want %v", got, uuid.Nil)
		}
	})
}

func TestGetEmail(t *testing.T) {
	t.Run("with email in context", func(t *testing.T) {
		expectedEmail := "test@example.com"
		ctx := context.WithValue(context.Background(), EmailKey, expectedEmail)

		got := GetEmail(ctx)
		if got != expectedEmail {
			t.Errorf("GetEmail() = %q, want %q", got, expectedEmail)
		}
	})

	t.Run("without email in context", func(t *testing.T) {
		ctx := context.Background()

		got := GetEmail(ctx)
		if got != "" {
			t.Errorf("GetEmail() = %q, want empty string", got)
		}
	})

	t.Run("with wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), EmailKey, 12345)

		got := GetEmail(ctx)
		if got != "" {
			t.Errorf("GetEmail() = %q, want empty string", got)
		}
	})
}

func TestContextKeys(t *testing.T) {
	// Ensure context keys are distinct
	if UserIDKey == EmailKey {
		t.Error("UserIDKey and EmailKey should be distinct")
	}
}
