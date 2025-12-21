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
	"github.com/agpt-go/chatbot-api/internal/handlers"
	"github.com/agpt-go/chatbot-api/internal/middleware"
	"github.com/agpt-go/chatbot-api/internal/services"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

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

	// Initialize services
	authService := services.NewAuthService(queries, cfg)
	llmService := services.NewLLMService(&cfg.OpenAI)
	chatService := services.NewChatService(queries, llmService)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authService)
	chatHandler := handlers.NewChatHandler(chatService)
	sessionHandler := handlers.NewSessionHandler(queries)

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(authService)

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
		w.Write([]byte(`{"status":"ok"}`))
	})

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
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
		log.Printf("Server starting on port %s", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}
