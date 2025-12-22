//go:build e2e

// Package e2e contains end-to-end tests for the API.
// These tests require a running application instance.
//
// Run with: go test -v -tags=e2e ./tests/e2e/...
//
// Environment variables:
//   - E2E_BASE_URL: Base URL of the running application (default: http://localhost:8080)
//   - E2E_TIMEOUT: Request timeout in seconds (default: 30)
//
// These tests are designed to run in the nightly E2E workflow,
// not during regular CI as they require full application stack.
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

var (
	baseURL    string
	httpClient *http.Client
)

func TestMain(m *testing.M) {
	// Setup
	baseURL = os.Getenv("E2E_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	timeout := 30
	if t := os.Getenv("E2E_TIMEOUT"); t != "" {
		if parsed, err := strconv.Atoi(t); err == nil {
			timeout = parsed
		}
	}

	httpClient = &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	// Check if server is running - fail early if not
	checkClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := checkClient.Get(baseURL + "/health")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Server not running at %s\n", baseURL)
		fmt.Fprintf(os.Stderr, "E2E tests require a running server. Start the application first.\n")
		fmt.Fprintf(os.Stderr, "Connection error: %v\n", err)
		os.Exit(1)
	}
	resp.Body.Close()

	// Run tests
	code := m.Run()

	os.Exit(code)
}

// TestE2E_HealthCheck tests the health endpoint
func TestE2E_HealthCheck(t *testing.T) {
	resp, err := httpClient.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

// TestE2E_AuthFlow tests the complete authentication flow
func TestE2E_AuthFlow(t *testing.T) {
	// Test user registration
	registerPayload := map[string]string{
		"username": fmt.Sprintf("testuser_%d", time.Now().UnixNano()),
		"password": "testpassword123",
		"email":    fmt.Sprintf("test_%d@example.com", time.Now().UnixNano()),
	}

	body, _ := json.Marshal(registerPayload)
	resp, err := httpClient.Post(baseURL+"/api/v1/auth/register", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Registration request failed: %v", err)
	}
	defer resp.Body.Close()

	// Registration might return 201 (created) or 409 (conflict if user exists)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Registration failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Test login
	loginPayload := map[string]string{
		"username": registerPayload["username"],
		"password": registerPayload["password"],
	}

	body, _ = json.Marshal(loginPayload)
	resp, err = httpClient.Post(baseURL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Login failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse login response
	var loginResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		t.Fatalf("Failed to parse login response: %v", err)
	}

	if loginResp.AccessToken == "" {
		t.Error("Expected access token in login response")
	}

	if loginResp.RefreshToken == "" {
		t.Error("Expected refresh token in login response")
	}

	t.Logf("Successfully authenticated, received access token")
}

// TestE2E_UnauthorizedAccess tests that protected endpoints require auth
func TestE2E_UnauthorizedAccess(t *testing.T) {
	// Try to access a protected endpoint without auth
	resp, err := httpClient.Get(baseURL + "/api/v1/sessions")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should return 401 Unauthorized
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

// TestE2E_InvalidCredentials tests login with wrong password
func TestE2E_InvalidCredentials(t *testing.T) {
	loginPayload := map[string]string{
		"username": "nonexistent_user_12345",
		"password": "wrongpassword",
	}

	body, _ := json.Marshal(loginPayload)
	resp, err := httpClient.Post(baseURL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Login request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should return 401 Unauthorized
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

// TestE2E_MalformedRequest tests API response to malformed JSON
func TestE2E_MalformedRequest(t *testing.T) {
	resp, err := httpClient.Post(baseURL+"/api/v1/auth/login", "application/json", bytes.NewBufferString("not valid json"))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should return 400 Bad Request
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

// TestE2E_ContentTypeValidation tests that API validates content type
func TestE2E_ContentTypeValidation(t *testing.T) {
	resp, err := httpClient.Post(baseURL+"/api/v1/auth/login", "text/plain", bytes.NewBufferString("username=test&password=test"))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should return 400 or 415 for wrong content type
	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("Expected status 400 or 415, got %d", resp.StatusCode)
	}
}

// TestE2E_RateLimiting tests that rate limiting works (if implemented)
func TestE2E_RateLimiting(t *testing.T) {
	// Make many requests quickly
	rateLimited := false
	for i := 0; i < 100; i++ {
		resp, err := httpClient.Get(baseURL + "/health")
		if err != nil {
			t.Logf("Request %d failed: %v", i, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			rateLimited = true
			t.Logf("Rate limited after %d requests", i+1)
			break
		}
	}

	// Rate limiting might not be implemented - just log
	if !rateLimited {
		t.Log("Rate limiting not triggered (might not be implemented)")
	}
}

// TestE2E_CORS tests CORS headers
func TestE2E_CORS(t *testing.T) {
	req, err := http.NewRequest("OPTIONS", baseURL+"/api/v1/auth/login", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("CORS preflight request failed: %v", err)
	}
	defer resp.Body.Close()

	// Check for CORS headers
	if resp.Header.Get("Access-Control-Allow-Origin") == "" {
		t.Log("CORS headers not present (might not be configured)")
	} else {
		t.Logf("CORS origin: %s", resp.Header.Get("Access-Control-Allow-Origin"))
	}
}
