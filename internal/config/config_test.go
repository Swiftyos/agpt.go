package config

import (
	"os"
	"testing"
	"time"
)

func TestDatabaseConfigConnectionString(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     "5432",
		User:     "testuser",
		Password: "testpass",
		Name:     "testdb",
		SSLMode:  "disable",
	}

	expected := "postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable"
	if got := cfg.ConnectionString(); got != expected {
		t.Errorf("ConnectionString() = %q, want %q", got, expected)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: &Config{
				JWT:    JWTConfig{Secret: "test-secret"},
				OpenAI: OpenAIConfig{APIKey: "test-key"},
			},
			wantErr: false,
		},
		{
			name: "missing JWT secret",
			cfg: &Config{
				JWT:    JWTConfig{Secret: ""},
				OpenAI: OpenAIConfig{APIKey: "test-key"},
			},
			wantErr: true,
			errMsg:  "JWT_SECRET is required",
		},
		{
			name: "missing OpenAI API key",
			cfg: &Config{
				JWT:    JWTConfig{Secret: "test-secret"},
				OpenAI: OpenAIConfig{APIKey: ""},
			},
			wantErr: true,
			errMsg:  "OPENAI_API_KEY is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && err.Error() != tt.errMsg {
				t.Errorf("Validate() error = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	// Test with existing env var
	t.Setenv("TEST_VAR", "test-value")

	if got := getEnv("TEST_VAR", "default"); got != "test-value" {
		t.Errorf("getEnv() = %q, want %q", got, "test-value")
	}

	// Test with non-existing env var
	if got := getEnv("NON_EXISTING_VAR", "default"); got != "default" {
		t.Errorf("getEnv() = %q, want %q", got, "default")
	}
}

func TestGetEnvAsInt(t *testing.T) {
	// Test with valid int
	t.Setenv("TEST_INT", "42")

	if got := getEnvAsInt("TEST_INT", 0); got != 42 {
		t.Errorf("getEnvAsInt() = %d, want %d", got, 42)
	}

	// Test with invalid int
	t.Setenv("TEST_INVALID", "not-a-number")

	if got := getEnvAsInt("TEST_INVALID", 10); got != 10 {
		t.Errorf("getEnvAsInt() = %d, want %d", got, 10)
	}

	// Test with non-existing var
	if got := getEnvAsInt("NON_EXISTING", 99); got != 99 {
		t.Errorf("getEnvAsInt() = %d, want %d", got, 99)
	}
}

func TestLoad(t *testing.T) {
	// Clear any pre-existing env vars that might override defaults
	os.Unsetenv("OPENAI_MODEL")
	os.Unsetenv("PORT")
	os.Unsetenv("ENVIRONMENT")
	os.Unsetenv("DB_HOST")

	// Set required env vars
	os.Setenv("JWT_SECRET", "test-jwt-secret")
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("OPENAI_API_KEY")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check defaults
	if cfg.Server.Port != "8080" {
		t.Errorf("Server.Port = %q, want %q", cfg.Server.Port, "8080")
	}

	if cfg.Server.Environment != "development" {
		t.Errorf("Server.Environment = %q, want %q", cfg.Server.Environment, "development")
	}

	if cfg.Database.Host != "localhost" {
		t.Errorf("Database.Host = %q, want %q", cfg.Database.Host, "localhost")
	}

	if cfg.JWT.AccessExpiresIn != 15*time.Minute {
		t.Errorf("JWT.AccessExpiresIn = %v, want %v", cfg.JWT.AccessExpiresIn, 15*time.Minute)
	}

	if cfg.OpenAI.Model != "gpt-5-mini-2025-08-07" {
		t.Errorf("OpenAI.Model = %q, want %q", cfg.OpenAI.Model, "gpt-5-mini-2025-08-07")
	}
}

func TestLoadWithCustomValues(t *testing.T) {
	// Set custom env vars
	os.Setenv("JWT_SECRET", "custom-secret")
	os.Setenv("OPENAI_API_KEY", "custom-key")
	os.Setenv("PORT", "9000")
	os.Setenv("ENVIRONMENT", "production")
	os.Setenv("DB_HOST", "db.example.com")
	os.Setenv("DB_MAX_CONNS", "50")
	os.Setenv("OPENAI_MODEL", "gpt-4-turbo")

	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("PORT")
		os.Unsetenv("ENVIRONMENT")
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_MAX_CONNS")
		os.Unsetenv("OPENAI_MODEL")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != "9000" {
		t.Errorf("Server.Port = %q, want %q", cfg.Server.Port, "9000")
	}

	if cfg.Server.Environment != "production" {
		t.Errorf("Server.Environment = %q, want %q", cfg.Server.Environment, "production")
	}

	if cfg.Database.Host != "db.example.com" {
		t.Errorf("Database.Host = %q, want %q", cfg.Database.Host, "db.example.com")
	}

	if cfg.Database.MaxConns != 50 {
		t.Errorf("Database.MaxConns = %d, want %d", cfg.Database.MaxConns, 50)
	}

	if cfg.OpenAI.Model != "gpt-4-turbo" {
		t.Errorf("OpenAI.Model = %q, want %q", cfg.OpenAI.Model, "gpt-4-turbo")
	}
}

func TestLoadMissingRequired(t *testing.T) {
	// Ensure required vars are not set
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("OPENAI_API_KEY")

	_, err := Load()
	if err == nil {
		t.Error("Load() expected error for missing required env vars")
	}
}
