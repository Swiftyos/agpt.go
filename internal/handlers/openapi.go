package handlers

import (
	"embed"
	"net/http"
)

//go:embed openapi.json
var openapiSpec embed.FS

// OpenAPIHandler serves the OpenAPI specification
type OpenAPIHandler struct{}

// NewOpenAPIHandler creates a new OpenAPI handler
func NewOpenAPIHandler() *OpenAPIHandler {
	return &OpenAPIHandler{}
}

// ServeSpec returns the OpenAPI JSON specification
func (h *OpenAPIHandler) ServeSpec(w http.ResponseWriter, r *http.Request) {
	data, err := openapiSpec.ReadFile("openapi.json")
	if err != nil {
		http.Error(w, "OpenAPI spec not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
