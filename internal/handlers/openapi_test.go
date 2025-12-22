package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAPIHandler_ServeSpec(t *testing.T) {
	handler := NewOpenAPIHandler()

	t.Run("returns OpenAPI spec", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
		rec := httptest.NewRecorder()

		handler.ServeSpec(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
			t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
		}

		if cacheControl := rec.Header().Get("Cache-Control"); cacheControl != "public, max-age=3600" {
			t.Errorf("Cache-Control = %q, want %q", cacheControl, "public, max-age=3600")
		}

		// Verify it's valid JSON
		var spec map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&spec); err != nil {
			t.Fatalf("failed to decode JSON: %v", err)
		}

		// Check required OpenAPI fields
		if spec["openapi"] == nil {
			t.Error("missing 'openapi' field in spec")
		}
		if spec["info"] == nil {
			t.Error("missing 'info' field in spec")
		}
		if spec["paths"] == nil {
			t.Error("missing 'paths' field in spec")
		}
	})

	t.Run("contains expected endpoints", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
		rec := httptest.NewRecorder()

		handler.ServeSpec(rec, req)

		var spec map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&spec); err != nil {
			t.Fatalf("failed to decode JSON: %v", err)
		}

		paths, ok := spec["paths"].(map[string]interface{})
		if !ok {
			t.Fatal("paths is not a map")
		}

		expectedPaths := []string{
			"/health",
			"/api/v1/auth/register",
			"/api/v1/auth/login",
			"/api/v1/auth/refresh",
			"/api/v1/auth/logout",
			"/api/v1/me",
			"/api/v1/sessions",
			"/api/v1/sessions/{sessionID}",
			"/api/v1/sessions/{sessionID}/messages",
			"/api/v1/sessions/{sessionID}/messages/stream",
		}

		for _, path := range expectedPaths {
			if paths[path] == nil {
				t.Errorf("missing path: %s", path)
			}
		}
	})

	t.Run("has correct OpenAPI version", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
		rec := httptest.NewRecorder()

		handler.ServeSpec(rec, req)

		var spec map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&spec); err != nil {
			t.Fatalf("failed to decode JSON: %v", err)
		}

		version, ok := spec["openapi"].(string)
		if !ok {
			t.Fatal("openapi version is not a string")
		}

		if version != "3.0.3" {
			t.Errorf("openapi version = %q, want %q", version, "3.0.3")
		}
	})

	t.Run("has security scheme defined", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
		rec := httptest.NewRecorder()

		handler.ServeSpec(rec, req)

		var spec map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&spec); err != nil {
			t.Fatalf("failed to decode JSON: %v", err)
		}

		components, ok := spec["components"].(map[string]interface{})
		if !ok {
			t.Fatal("components is not a map")
		}

		securitySchemes, ok := components["securitySchemes"].(map[string]interface{})
		if !ok {
			t.Fatal("securitySchemes is not a map")
		}

		if securitySchemes["bearerAuth"] == nil {
			t.Error("missing bearerAuth security scheme")
		}
	})
}
