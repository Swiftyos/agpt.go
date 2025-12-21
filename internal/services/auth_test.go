package services

import (
	"testing"
	"time"

	"github.com/agpt-go/chatbot-api/internal/config"
	"github.com/google/uuid"
)

func createTestConfig() *config.Config {
	return &config.Config{
		JWT: config.JWTConfig{
			Secret:           "test-secret-key-for-testing-minimum-32-characters-long",
			AccessExpiresIn:  15 * time.Minute,
			RefreshExpiresIn: 7 * 24 * time.Hour,
			Issuer:           "test-issuer",
		},
		OAuth: config.OAuthConfig{
			GoogleClientID:     "",
			GoogleClientSecret: "",
			GoogleRedirectURL:  "",
		},
	}
}

func TestNewAuthService(t *testing.T) {
	cfg := createTestConfig()
	svc := NewAuthService(nil, cfg)

	if svc == nil {
		t.Fatal("NewAuthService() returned nil")
	}
	if svc.config != cfg {
		t.Error("NewAuthService() did not set config correctly")
	}
}

func TestNewAuthServiceWithGoogleOAuth(t *testing.T) {
	cfg := createTestConfig()
	cfg.OAuth.GoogleClientID = "test-client-id"
	cfg.OAuth.GoogleClientSecret = "test-client-secret"
	cfg.OAuth.GoogleRedirectURL = "http://localhost:8080/callback"

	svc := NewAuthService(nil, cfg)

	if svc == nil {
		t.Fatal("NewAuthService() returned nil")
	}
	if svc.googleOAuth == nil {
		t.Error("NewAuthService() should set googleOAuth when credentials are provided")
	}
}

func TestValidateAccessToken(t *testing.T) {
	cfg := createTestConfig()
	svc := NewAuthService(nil, cfg)

	t.Run("invalid token format", func(t *testing.T) {
		_, err := svc.ValidateAccessToken("not-a-valid-token")
		if err != ErrInvalidToken {
			t.Errorf("ValidateAccessToken() error = %v, want %v", err, ErrInvalidToken)
		}
	})

	t.Run("token with wrong signature", func(t *testing.T) {
		// Token signed with a different secret
		wrongToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMTIzNDU2Nzg5MCIsImVtYWlsIjoidGVzdEBleGFtcGxlLmNvbSJ9.wrong-signature"
		_, err := svc.ValidateAccessToken(wrongToken)
		if err != ErrInvalidToken {
			t.Errorf("ValidateAccessToken() error = %v, want %v", err, ErrInvalidToken)
		}
	})

	t.Run("empty token", func(t *testing.T) {
		_, err := svc.ValidateAccessToken("")
		if err != ErrInvalidToken {
			t.Errorf("ValidateAccessToken() error = %v, want %v", err, ErrInvalidToken)
		}
	})
}

func TestGetGoogleAuthURL(t *testing.T) {
	t.Run("without google oauth configured", func(t *testing.T) {
		cfg := createTestConfig()
		svc := NewAuthService(nil, cfg)

		url := svc.GetGoogleAuthURL("test-state")
		if url != "" {
			t.Errorf("GetGoogleAuthURL() = %q, want empty string", url)
		}
	})

	t.Run("with google oauth configured", func(t *testing.T) {
		cfg := createTestConfig()
		cfg.OAuth.GoogleClientID = "test-client-id"
		cfg.OAuth.GoogleClientSecret = "test-client-secret"
		cfg.OAuth.GoogleRedirectURL = "http://localhost:8080/callback"

		svc := NewAuthService(nil, cfg)

		url := svc.GetGoogleAuthURL("test-state")
		if url == "" {
			t.Error("GetGoogleAuthURL() returned empty string")
		}
	})
}

func TestHashToken(t *testing.T) {
	token := "test-token-12345"
	hash := hashToken(token)

	if hash == "" {
		t.Error("hashToken() returned empty string")
	}
	if hash == token {
		t.Error("hashToken() returned same value as input")
	}

	// Hash should be deterministic
	hash2 := hashToken(token)
	if hash != hash2 {
		t.Error("hashToken() is not deterministic")
	}

	// Different tokens should have different hashes
	hash3 := hashToken("different-token")
	if hash == hash3 {
		t.Error("hashToken() returned same hash for different tokens")
	}
}

func TestTokenPairStruct(t *testing.T) {
	tp := TokenPair{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresIn:    900,
		TokenType:    "Bearer",
	}

	if tp.AccessToken != "access-token" {
		t.Errorf("TokenPair.AccessToken = %q, want %q", tp.AccessToken, "access-token")
	}
	if tp.RefreshToken != "refresh-token" {
		t.Errorf("TokenPair.RefreshToken = %q, want %q", tp.RefreshToken, "refresh-token")
	}
	if tp.ExpiresIn != 900 {
		t.Errorf("TokenPair.ExpiresIn = %d, want %d", tp.ExpiresIn, 900)
	}
	if tp.TokenType != "Bearer" {
		t.Errorf("TokenPair.TokenType = %q, want %q", tp.TokenType, "Bearer")
	}
}

func TestClaimsStruct(t *testing.T) {
	userID := uuid.New()
	claims := Claims{
		UserID: userID,
		Email:  "test@example.com",
	}

	if claims.UserID != userID {
		t.Errorf("Claims.UserID = %v, want %v", claims.UserID, userID)
	}
	if claims.Email != "test@example.com" {
		t.Errorf("Claims.Email = %q, want %q", claims.Email, "test@example.com")
	}
}

func TestErrorVariables(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"ErrInvalidCredentials", ErrInvalidCredentials, "invalid credentials"},
		{"ErrUserExists", ErrUserExists, "user already exists"},
		{"ErrInvalidToken", ErrInvalidToken, "invalid token"},
		{"ErrTokenExpired", ErrTokenExpired, "token expired"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.want {
				t.Errorf("%s.Error() = %q, want %q", tt.name, tt.err.Error(), tt.want)
			}
		})
	}
}

func TestGoogleUserInfoStruct(t *testing.T) {
	userInfo := GoogleUserInfo{
		ID:            "google-id-123",
		Email:         "test@gmail.com",
		Name:          "Test User",
		Picture:       "https://example.com/photo.jpg",
		EmailVerified: true,
	}

	if userInfo.ID != "google-id-123" {
		t.Errorf("GoogleUserInfo.ID = %q, want %q", userInfo.ID, "google-id-123")
	}
	if userInfo.Email != "test@gmail.com" {
		t.Errorf("GoogleUserInfo.Email = %q, want %q", userInfo.Email, "test@gmail.com")
	}
	if userInfo.Name != "Test User" {
		t.Errorf("GoogleUserInfo.Name = %q, want %q", userInfo.Name, "Test User")
	}
	if userInfo.Picture != "https://example.com/photo.jpg" {
		t.Errorf("GoogleUserInfo.Picture = %q, want %q", userInfo.Picture, "https://example.com/photo.jpg")
	}
	if !userInfo.EmailVerified {
		t.Error("GoogleUserInfo.EmailVerified should be true")
	}
}
