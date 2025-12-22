package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	OAuth    OAuthConfig
	OpenAI   OpenAIConfig
}

type ServerConfig struct {
	Port         string
	Environment  string
	AllowOrigins []string
}

type DatabaseConfig struct {
	Host        string
	Port        string
	User        string
	Password    string
	Name        string
	SSLMode     string
	MaxConns    int32
	MinConns    int32
	MaxConnLife time.Duration
	MaxConnIdle time.Duration
}

type JWTConfig struct {
	Secret           string
	AccessExpiresIn  time.Duration
	RefreshExpiresIn time.Duration
	Issuer           string
}

type OAuthConfig struct {
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
}

type OpenAIConfig struct {
	APIKey string
	Model  string
}

func (d DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode,
	)
}

// ParseDatabaseURL parses a DATABASE_URL into a DatabaseConfig with default pool settings
func ParseDatabaseURL(databaseURL string) (DatabaseConfig, error) {
	// Parse the URL
	u, err := url.Parse(databaseURL)
	if err != nil {
		return DatabaseConfig{}, fmt.Errorf("invalid database URL: %w", err)
	}

	password, _ := u.User.Password()
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "5432"
	}

	// Extract database name (remove leading /)
	dbName := u.Path
	if len(dbName) > 0 && dbName[0] == '/' {
		dbName = dbName[1:]
	}

	// Extract sslmode from query params, default to "disable"
	sslMode := u.Query().Get("sslmode")
	if sslMode == "" {
		sslMode = "disable"
	}

	return DatabaseConfig{
		Host:        host,
		Port:        port,
		User:        u.User.Username(),
		Password:    password,
		Name:        dbName,
		SSLMode:     sslMode,
		MaxConns:    25,
		MinConns:    5,
		MaxConnLife: 60 * time.Minute,
		MaxConnIdle: 30 * time.Minute,
	}, nil
}

func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	cfg := &Config{
		Server: ServerConfig{
			Port:         getEnv("PORT", "8080"),
			Environment:  getEnv("ENVIRONMENT", "development"),
			AllowOrigins: []string{getEnv("CORS_ORIGINS", "http://localhost:3000")},
		},
		Database: DatabaseConfig{
			Host:        getEnv("DB_HOST", "localhost"),
			Port:        getEnv("DB_PORT", "5432"),
			User:        getEnv("DB_USER", "postgres"),
			Password:    getEnv("DB_PASSWORD", "postgres"),
			Name:        getEnv("DB_NAME", "chatbot"),
			SSLMode:     getEnv("DB_SSLMODE", "disable"),
			MaxConns:    int32(getEnvAsInt("DB_MAX_CONNS", 25)),
			MinConns:    int32(getEnvAsInt("DB_MIN_CONNS", 5)),
			MaxConnLife: time.Duration(getEnvAsInt("DB_MAX_CONN_LIFE_MINUTES", 60)) * time.Minute,
			MaxConnIdle: time.Duration(getEnvAsInt("DB_MAX_CONN_IDLE_MINUTES", 30)) * time.Minute,
		},
		JWT: JWTConfig{
			Secret:           getEnv("JWT_SECRET", ""),
			AccessExpiresIn:  time.Duration(getEnvAsInt("JWT_ACCESS_EXPIRES_MINUTES", 15)) * time.Minute,
			RefreshExpiresIn: time.Duration(getEnvAsInt("JWT_REFRESH_EXPIRES_DAYS", 7)) * 24 * time.Hour,
			Issuer:           getEnv("JWT_ISSUER", "chatbot-api"),
		},
		OAuth: OAuthConfig{
			GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
			GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
			GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/auth/google/callback"),
		},
		OpenAI: OpenAIConfig{
			APIKey: getEnv("OPENAI_API_KEY", ""),
			Model:  getEnv("OPENAI_MODEL", "gpt-4o"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if c.OpenAI.APIKey == "" {
		return fmt.Errorf("OPENAI_API_KEY is required")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
