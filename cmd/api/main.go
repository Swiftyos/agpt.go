package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agpt-go/chatbot-api/internal/config"
	"github.com/agpt-go/chatbot-api/internal/database"
	_ "github.com/agpt-go/chatbot-api/internal/docs" // Swagger docs
	"github.com/agpt-go/chatbot-api/internal/handlers"
	"github.com/agpt-go/chatbot-api/internal/logging"
	"github.com/agpt-go/chatbot-api/internal/middleware"
	"github.com/agpt-go/chatbot-api/internal/services"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title AGPT Chatbot API
// @version 1.0
// @description A Go backend API for a ChatGPT-like chatbot application with Next.js frontend integration.
// @description Features include user authentication, chat sessions, streaming responses, and AI-powered tools.

// @contact.name AGPT Team
// @license.name MIT

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT access token. Format: "Bearer {token}"

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize structured logging
	logging.Initialize(cfg.Server.Environment)

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to database
	pool, err := database.NewPool(ctx, cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Initialize queries
	queries := database.New(pool)

	// Initialize analytics service
	analyticsService := services.NewAnalyticsService(&cfg.Analytics)
	defer analyticsService.Close()

	// Initialize services
	authService := services.NewAuthService(queries, cfg)
	llmService := services.NewLLMService(&cfg.OpenAI)
	chatService := services.NewChatService(queries, llmService, analyticsService)
	referralService := services.NewReferralService(queries, analyticsService, cfg.Server.BaseURL, cfg.Referral.IPSalt)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authService, analyticsService, referralService)
	chatHandler := handlers.NewChatHandler(chatService, analyticsService)
	sessionHandler := handlers.NewSessionHandler(queries)
	openapiHandler := handlers.NewOpenAPIHandler()
	referralHandler := handlers.NewReferralHandler(referralService)

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(authService)
	// Rate limiter for public endpoints (Troy Hunt: prevent abuse)
	// 60 requests per minute per IP for public referral endpoints
	publicRateLimiter := middleware.NewRateLimiter(60, time.Minute)

	// Setup router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(60 * time.Second))
	r.Use(middleware.NewCORS(cfg.Server.AllowOrigins))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Swagger documentation
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// OpenAPI specification (public)
		r.Get("/openapi.json", openapiHandler.ServeSpec)

		// Auth routes (public)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.Refresh)
			r.Post("/logout", authHandler.Logout)

			// OAuth routes
			r.Get("/google", authHandler.GoogleLogin)
			r.Get("/google/callback", authHandler.GoogleCallback)
		})

		// Public referral routes (for tracking clicks and validation)
		// Rate limited to prevent abuse (Troy Hunt recommendation)
		r.Route("/referral", func(r chi.Router) {
			r.Use(publicRateLimiter.Limit)
			r.Post("/click", referralHandler.RecordClick)
			r.Get("/validate/{code}", referralHandler.ValidateCode)
		})

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.RequireAuth)

			// User routes
			r.Get("/me", sessionHandler.GetCurrentUser)

			// Chat session routes
			r.Route("/sessions", func(r chi.Router) {
				r.Post("/", chatHandler.CreateSession)
				r.Get("/", chatHandler.ListSessions)
				r.Get("/{sessionID}", chatHandler.GetSession)
				r.Patch("/{sessionID}", chatHandler.UpdateSession)
				r.Delete("/{sessionID}", chatHandler.DeleteSession)

				// Messages
				r.Get("/{sessionID}/messages", chatHandler.GetMessages)
				r.Post("/{sessionID}/messages", chatHandler.SendMessage)
				r.Post("/{sessionID}/messages/stream", chatHandler.SendMessageStream)
			})

			// Protected referral routes (for authenticated users)
			r.Route("/referral", func(r chi.Router) {
				r.Get("/code", referralHandler.GetReferralCode)
				r.Post("/share", referralHandler.RecordShare)
				r.Get("/stats", referralHandler.GetReferralStats)
				r.Get("/referred", referralHandler.GetReferredUsers)
				r.Get("/shares", referralHandler.GetShareHistory)
			})
		})
	})

	// Create server
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second, // Longer for streaming
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logging.Info("server starting", "port", cfg.Server.Port, "environment", cfg.Server.Environment)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.Error("server error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logging.Info("shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logging.Error("server forced to shutdown", err)
		os.Exit(1)
	}

	logging.Info("server stopped")
}
