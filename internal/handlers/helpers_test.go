package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agpt-go/chatbot-api/internal/database"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()

	data := map[string]string{"message": "hello"}
	writeJSON(rec, http.StatusOK, data)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want %q", rec.Header().Get("Content-Type"), "application/json")
	}

	var result map[string]string
	json.NewDecoder(rec.Body).Decode(&result)
	if result["message"] != "hello" {
		t.Errorf("body message = %q, want %q", result["message"], "hello")
	}
}

func TestWriteJSONWithDifferentStatusCodes(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"OK", http.StatusOK},
		{"Created", http.StatusCreated},
		{"BadRequest", http.StatusBadRequest},
		{"Unauthorized", http.StatusUnauthorized},
		{"NotFound", http.StatusNotFound},
		{"InternalServerError", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeJSON(rec, tt.status, map[string]string{})

			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d", rec.Code, tt.status)
			}
		})
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()

	writeError(rec, http.StatusBadRequest, "Something went wrong")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var result ErrorResponse
	json.NewDecoder(rec.Body).Decode(&result)
	if result.Error != "Something went wrong" {
		t.Errorf("error message = %q, want %q", result.Error, "Something went wrong")
	}
}

func TestWriteValidationError(t *testing.T) {
	validate := validator.New()

	type TestStruct struct {
		Email    string `validate:"required,email"`
		Password string `validate:"required,min=8"`
		Name     string `validate:"required,min=2,max=100"`
	}

	t.Run("required validation", func(t *testing.T) {
		rec := httptest.NewRecorder()
		err := validate.Struct(TestStruct{})
		writeValidationError(rec, err)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}

		var result ErrorResponse
		json.NewDecoder(rec.Body).Decode(&result)
		if result.Error != "Validation failed" {
			t.Errorf("error = %q, want %q", result.Error, "Validation failed")
		}
		if len(result.Details) == 0 {
			t.Error("expected validation details")
		}
	})

	t.Run("email validation", func(t *testing.T) {
		rec := httptest.NewRecorder()
		err := validate.Struct(TestStruct{Email: "invalid", Password: "password123", Name: "Test"})
		writeValidationError(rec, err)

		var result ErrorResponse
		json.NewDecoder(rec.Body).Decode(&result)
		if result.Details["Email"] == "" {
			t.Error("expected Email validation error")
		}
	})

	t.Run("min length validation", func(t *testing.T) {
		rec := httptest.NewRecorder()
		err := validate.Struct(TestStruct{Email: "test@example.com", Password: "short", Name: "Test"})
		writeValidationError(rec, err)

		var result ErrorResponse
		json.NewDecoder(rec.Body).Decode(&result)
		if result.Details["Password"] == "" {
			t.Error("expected Password validation error")
		}
	})

	t.Run("non-validation error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		err := errors.New("some random error")
		writeValidationError(rec, err)

		var result ErrorResponse
		json.NewDecoder(rec.Body).Decode(&result)
		if result.Error != "Validation failed" {
			t.Errorf("error = %q, want %q", result.Error, "Validation failed")
		}
	})
}

func TestErrorResponseStruct(t *testing.T) {
	resp := ErrorResponse{
		Error:   "Something went wrong",
		Details: map[string]string{"field": "error message"},
	}

	if resp.Error != "Something went wrong" {
		t.Errorf("ErrorResponse.Error = %q, want %q", resp.Error, "Something went wrong")
	}
	if resp.Details["field"] != "error message" {
		t.Errorf("ErrorResponse.Details[field] = %q, want %q", resp.Details["field"], "error message")
	}
}

func TestUserToResponse(t *testing.T) {
	t.Run("nil user", func(t *testing.T) {
		result := UserToResponse(nil)
		if result != nil {
			t.Error("UserToResponse(nil) should return nil")
		}
	})

	t.Run("user with all fields", func(t *testing.T) {
		avatarURL := "https://example.com/avatar.jpg"
		provider := "google"
		emailVerified := true
		user := &database.User{
			ID:            uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
			Email:         "test@example.com",
			Name:          "Test User",
			AvatarUrl:     &avatarURL,
			Provider:      &provider,
			EmailVerified: &emailVerified,
		}

		result := UserToResponse(user)

		if result == nil {
			t.Fatal("UserToResponse() returned nil")
		}
		if result.ID != "550e8400-e29b-41d4-a716-446655440000" {
			t.Errorf("ID = %q, want %q", result.ID, "550e8400-e29b-41d4-a716-446655440000")
		}
		if result.Email != "test@example.com" {
			t.Errorf("Email = %q, want %q", result.Email, "test@example.com")
		}
		if result.Name != "Test User" {
			t.Errorf("Name = %q, want %q", result.Name, "Test User")
		}
		if result.AvatarURL == nil || *result.AvatarURL != avatarURL {
			t.Errorf("AvatarURL = %v, want %q", result.AvatarURL, avatarURL)
		}
		if result.Provider != "google" {
			t.Errorf("Provider = %q, want %q", result.Provider, "google")
		}
		if !result.EmailVerified {
			t.Error("EmailVerified should be true")
		}
	})

	t.Run("user with nil optional fields", func(t *testing.T) {
		user := &database.User{
			ID:    uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
			Email: "test@example.com",
			Name:  "Test User",
		}

		result := UserToResponse(user)

		if result == nil {
			t.Fatal("UserToResponse() returned nil")
		}
		if result.AvatarURL != nil {
			t.Errorf("AvatarURL = %v, want nil", result.AvatarURL)
		}
		if result.Provider != "" {
			t.Errorf("Provider = %q, want empty string", result.Provider)
		}
		if result.EmailVerified {
			t.Error("EmailVerified should be false for nil value")
		}
	})
}
