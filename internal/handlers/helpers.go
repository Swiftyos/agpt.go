package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/agpt-go/chatbot-api/internal/database"
	"github.com/go-playground/validator/v10"
)

type ErrorResponse struct {
	Error   string            `json:"error"`
	Details map[string]string `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}

func writeValidationError(w http.ResponseWriter, err error) {
	details := make(map[string]string)

	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			field := e.Field()
			switch e.Tag() {
			case "required":
				details[field] = field + " is required"
			case "email":
				details[field] = field + " must be a valid email"
			case "min":
				details[field] = field + " must be at least " + e.Param() + " characters"
			case "max":
				details[field] = field + " must be at most " + e.Param() + " characters"
			default:
				details[field] = field + " is invalid"
			}
		}
	}

	writeJSON(w, http.StatusBadRequest, ErrorResponse{
		Error:   "Validation failed",
		Details: details,
	})
}

func UserToResponse(user *database.User) *UserResponse {
	if user == nil {
		return nil
	}
	return &UserResponse{
		ID:            user.ID.String(),
		Email:         user.Email,
		Name:          user.Name,
		AvatarURL:     user.AvatarUrl,
		Provider:      user.Provider,
		EmailVerified: user.EmailVerified,
	}
}
