package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/agpt-go/chatbot-api/internal/logging"
	"github.com/agpt-go/chatbot-api/internal/services"
	"github.com/go-playground/validator/v10"
)

type AuthHandler struct {
	authService AuthServicer
	validate    Validator
}

func NewAuthHandler(authService AuthServicer) *AuthHandler {
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
		User:   UserToResponse(user),
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
		User:   UserToResponse(user),
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

	if err := h.authService.Logout(r.Context(), req.RefreshToken); err != nil {
		logging.Warn("failed to revoke refresh token during logout", "error", err)
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

func (h *AuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	// Generate and store OAuth state for CSRF protection
	state, err := h.authService.GenerateOAuthState(r.Context())
	if err != nil {
		logging.Error("failed to generate oauth state", err)
		writeError(w, http.StatusInternalServerError, "Failed to initiate OAuth flow")
		return
	}

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

	// Verify state parameter for CSRF protection
	state := r.URL.Query().Get("state")
	if err := h.authService.ValidateOAuthState(r.Context(), state); err != nil {
		if errors.Is(err, services.ErrInvalidOAuthState) {
			writeError(w, http.StatusBadRequest, "Invalid or expired OAuth state")
			return
		}
		logging.Error("oauth state validation error", err)
		writeError(w, http.StatusInternalServerError, "Failed to validate OAuth state")
		return
	}

	user, tokens, err := h.authService.HandleGoogleCallback(r.Context(), code)
	if err != nil {
		logging.Error("google oauth callback failed", err)
		writeError(w, http.StatusInternalServerError, "Failed to authenticate with Google")
		return
	}

	writeJSON(w, http.StatusOK, AuthResponse{
		User:   UserToResponse(user),
		Tokens: tokens,
	})
}

