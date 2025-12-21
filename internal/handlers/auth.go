package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/agpt-go/chatbot-api/internal/services"
	"github.com/go-playground/validator/v10"
)

type AuthHandler struct {
	authService *services.AuthService
	validate    *validator.Validate
}

func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		validate:    validator.New(),
	}
}

type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name" validate:"required,min=2"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type AuthResponse struct {
	User   *UserResponse        `json:"user"`
	Tokens *services.TokenPair  `json:"tokens"`
}

type UserResponse struct {
	ID            string  `json:"id"`
	Email         string  `json:"email"`
	Name          string  `json:"name"`
	AvatarURL     *string `json:"avatar_url,omitempty"`
	Provider      string  `json:"provider"`
	EmailVerified bool    `json:"email_verified"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	user, tokens, err := h.authService.Register(r.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		if errors.Is(err, services.ErrUserExists) {
			writeError(w, http.StatusConflict, "User already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to register user")
		return
	}

	writeJSON(w, http.StatusCreated, AuthResponse{
		User:   toUserResponse(user),
		Tokens: tokens,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	user, tokens, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "Invalid email or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to login")
		return
	}

	writeJSON(w, http.StatusOK, AuthResponse{
		User:   toUserResponse(user),
		Tokens: tokens,
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	tokens, err := h.authService.RefreshTokens(r.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, services.ErrInvalidToken) || errors.Is(err, services.ErrTokenExpired) {
			writeError(w, http.StatusUnauthorized, "Invalid or expired refresh token")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to refresh tokens")
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	_ = h.authService.Logout(r.Context(), req.RefreshToken)
	writeJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

func (h *AuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	state := generateState()
	// In production, store state in session/cookie for verification
	url := h.authService.GetGoogleAuthURL(state)
	if url == "" {
		writeError(w, http.StatusNotImplemented, "Google OAuth not configured")
		return
	}
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "Missing authorization code")
		return
	}

	// In production, verify state parameter
	user, tokens, err := h.authService.HandleGoogleCallback(r.Context(), code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to authenticate with Google")
		return
	}

	writeJSON(w, http.StatusOK, AuthResponse{
		User:   toUserResponse(user),
		Tokens: tokens,
	})
}

func generateState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func toUserResponse(user interface{}) *UserResponse {
	// Use reflection-free approach with struct
	switch u := user.(type) {
	case interface{ GetID() interface{} }:
		_ = u // placeholder
	}

	// For now, use a simple type assertion approach
	// This will be properly typed when SQLC generates the models
	return &UserResponse{}
}
