package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// NewCORS creates a CORS middleware with the specified allowed origins
func NewCORS(allowedOrigins []string) func(next http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"},
		ExposedHeaders:   []string{"Link", "X-Vercel-AI-Data-Stream"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}
