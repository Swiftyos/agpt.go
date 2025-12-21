package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/agpt-go/chatbot-api/internal/services"
	"github.com/google/uuid"
)

type contextKey string

const (
	UserIDKey contextKey = "userID"
	EmailKey  contextKey = "email"
)

type AuthMiddleware struct {
	authService *services.AuthService
}

func NewAuthMiddleware(authService *services.AuthService) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
	}
}

// RequireAuth validates the JWT token and adds user info to context
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"Missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, `{"error":"Invalid authorization header format"}`, http.StatusUnauthorized)
			return
		}

		token := parts[1]
		claims, err := m.authService.ValidateAccessToken(token)
		if err != nil {
			http.Error(w, `{"error":"Invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		// Add user info to context
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, EmailKey, claims.Email)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth validates the JWT token if present, but doesn't require it
func (m *AuthMiddleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			next.ServeHTTP(w, r)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			next.ServeHTTP(w, r)
			return
		}

		token := parts[1]
		claims, err := m.authService.ValidateAccessToken(token)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		// Add user info to context
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, EmailKey, claims.Email)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserID retrieves the user ID from context
func GetUserID(ctx context.Context) uuid.UUID {
	userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return userID
}

// GetEmail retrieves the email from context
func GetEmail(ctx context.Context) string {
	email, ok := ctx.Value(EmailKey).(string)
	if !ok {
		return ""
	}
	return email
}
