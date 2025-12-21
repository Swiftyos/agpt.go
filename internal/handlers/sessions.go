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

// GetCurrentUser returns the authenticated user's profile
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
