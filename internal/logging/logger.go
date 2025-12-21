package logging

import (
	"context"
	"log/slog"
	"os"
)

// Logger is the global structured logger
var Logger *slog.Logger

// Initialize sets up the global logger with the specified level
func Initialize(env string) {
	var level slog.Level
	switch env {
	case "production":
		level = slog.LevelInfo
	case "development":
		level = slog.LevelDebug
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	Logger = slog.New(handler)
	slog.SetDefault(Logger)
}

// WithContext returns a logger with context values
func WithContext(ctx context.Context) *slog.Logger {
	return Logger
}

// Error logs an error with structured fields
func Error(msg string, err error, args ...any) {
	if Logger == nil {
		return
	}
	allArgs := append([]any{"error", err}, args...)
	Logger.Error(msg, allArgs...)
}

// Info logs an info message with structured fields
func Info(msg string, args ...any) {
	if Logger == nil {
		return
	}
	Logger.Info(msg, args...)
}

// Debug logs a debug message with structured fields
func Debug(msg string, args ...any) {
	if Logger == nil {
		return
	}
	Logger.Debug(msg, args...)
}

// Warn logs a warning message with structured fields
func Warn(msg string, args ...any) {
	if Logger == nil {
		return
	}
	Logger.Warn(msg, args...)
}
