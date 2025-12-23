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
	analytics   AnalyticsServicer
	validate    Validator
}

func NewAuthHandler(authService AuthServicer, analytics AnalyticsServicer) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		analytics:   analytics,
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
	User   *UserResponse       `json:"user"`
	Tokens *services.TokenPair `json:"tokens"`
}

type UserResponse struct {
	ID            string  `json:"id"`
	Email         string  `json:"email"`
	Name          string  `json:"name"`
	AvatarURL     *string `json:"avatar_url,omitempty"`
	Provider      string  `json:"provider"`
	EmailVerified bool    `json:"email_verified"`
}

// Register godoc
// @Summary Register a new user
// @Description Create a new user account with email and password
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration details"
// @Success 201 {object} AuthResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /auth/register [post]
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

	// Track signup event
	if h.analytics != nil {
		h.analytics.TrackUserSignedUp(user.ID, user.Email, user.Name, "email")
	}

	writeJSON(w, http.StatusCreated, AuthResponse{
		User:   UserToResponse(user),
		Tokens: tokens,
	})
}

// Login godoc
// @Summary Login user
// @Description Authenticate user with email and password
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /auth/login [post]
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

	// Track login event
	if h.analytics != nil {
		h.analytics.TrackUserLoggedIn(user.ID, "email")
	}

	writeJSON(w, http.StatusOK, AuthResponse{
		User:   UserToResponse(user),
		Tokens: tokens,
	})
}

// Refresh godoc
// @Summary Refresh access token
// @Description Get a new access token using a valid refresh token
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body RefreshRequest true "Refresh token"
// @Success 200 {object} services.TokenPair
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /auth/refresh [post]
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

// Logout godoc
// @Summary Logout user
// @Description Revoke the refresh token to log out
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body RefreshRequest true "Refresh token to revoke"
// @Success 200 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Router /auth/logout [post]
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

// GoogleLogin godoc
// @Summary Initiate Google OAuth
// @Description Redirect to Google OAuth consent screen
// @Tags Authentication
// @Success 307 "Redirect to Google OAuth"
// @Failure 500 {object} ErrorResponse
// @Router /auth/google [get]
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

// GoogleCallback godoc
// @Summary Google OAuth callback
// @Description Handle Google OAuth callback and create/login user
// @Tags Authentication
// @Produce json
// @Param code query string true "Authorization code from Google"
// @Param state query string true "CSRF protection state parameter"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /auth/google/callback [get]
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

	// Track Google signup/login - we track as signup since HandleGoogleCallback creates user if not exists
	if h.analytics != nil {
		h.analytics.TrackUserSignedUp(user.ID, user.Email, user.Name, "google")
	}

	writeJSON(w, http.StatusOK, AuthResponse{
		User:   UserToResponse(user),
		Tokens: tokens,
	})
}
