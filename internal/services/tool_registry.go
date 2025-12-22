package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// Tool represents a callable tool with its definition and handler
type Tool struct {
	Definition ToolDefinition
	Handler    ToolHandler
}

// ToolHandler is the function signature for tool execution
type ToolHandler func(ctx context.Context, userID uuid.UUID, arguments string) (*ToolResult, error)

// ToolResult is the unified result type for all tools
type ToolResult struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// ToolRegistry manages tool registration and execution
type ToolRegistry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry
func (r *ToolRegistry) Register(name string, definition ToolDefinition, handler ToolHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[name] = Tool{
		Definition: definition,
		Handler:    handler,
	}
}

// GetDefinitions returns all tool definitions for the LLM
func (r *ToolRegistry) GetDefinitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	definitions := make([]ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		definitions = append(definitions, tool.Definition)
	}
	return definitions
}

// Execute runs a tool by name
func (r *ToolRegistry) Execute(ctx context.Context, userID uuid.UUID, toolName, arguments string) (*ToolResult, error) {
	r.mu.RLock()
	tool, exists := r.tools[toolName]
	r.mu.RUnlock()

	if !exists {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", toolName),
		}, nil
	}

	return tool.Handler(ctx, userID, arguments)
}

// ExecuteToolCall is a convenience method for ToolCall structs
func (r *ToolRegistry) ExecuteToolCall(ctx context.Context, userID uuid.UUID, tc ToolCall) (*ToolResult, error) {
	return r.Execute(ctx, userID, tc.Function.Name, tc.Function.Arguments)
}

// Helper to parse JSON arguments into a struct
func ParseArgs[T any](arguments string) (T, error) {
	var args T
	err := json.Unmarshal([]byte(arguments), &args)
	return args, err
}
