package handlers

import (
	"net/http"

	"github.com/agpt-go/chatbot-api/internal/database"
	"github.com/agpt-go/chatbot-api/internal/middleware"
	"github.com/google/uuid"
)

type SessionHandler struct {
	queries *database.Queries
}

func NewSessionHandler(queries *database.Queries) *SessionHandler {
	return &SessionHandler{
		queries: queries,
	}
}

// GetCurrentUser godoc
// @Summary Get current user
// @Description Get the authenticated user's profile
// @Tags User
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UserResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /me [get]
func (h *SessionHandler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "User not found")
		return
	}

	writeJSON(w, http.StatusOK, UserToResponse(&user))
}
