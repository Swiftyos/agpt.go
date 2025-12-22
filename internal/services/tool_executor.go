package services

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// ToolExecutor handles the execution of tool calls from the LLM
// It delegates to the ToolRegistry for actual execution
type ToolExecutor struct {
	toolService *ToolService
}

// NewToolExecutor creates a new tool executor
func NewToolExecutor(toolService *ToolService) *ToolExecutor {
	return &ToolExecutor{
		toolService: toolService,
	}
}

// ToolExecutionResult represents the result of executing a tool
// This wraps ToolResult for backward compatibility
type ToolExecutionResult struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ExecuteTool executes a tool call via the registry
func (e *ToolExecutor) ExecuteTool(ctx context.Context, userID uuid.UUID, toolName string, arguments string) (*ToolExecutionResult, error) {
	// Delegate to the registry - no switch statement needed!
	result, err := e.toolService.GetRegistry().Execute(ctx, userID, toolName, arguments)
	if err != nil {
		return &ToolExecutionResult{Success: false, Error: err.Error()}, nil
	}

	return &ToolExecutionResult{
		Success: result.Success,
		Result:  result,
		Error:   result.Error,
	}, nil
}

// ExecuteToolCall is a convenience method that takes a ToolCall directly
func (e *ToolExecutor) ExecuteToolCall(ctx context.Context, userID uuid.UUID, toolCall ToolCall) (*ToolExecutionResult, error) {
	return e.ExecuteTool(ctx, userID, toolCall.Function.Name, toolCall.Function.Arguments)
}

// ToToolResultMessage converts a tool execution result to a chat message
// that can be sent back to the LLM
func (e *ToolExecutor) ToToolResultMessage(toolCallID, toolName string, result *ToolExecutionResult) ChatMessage {
	content, _ := json.Marshal(result)
	return ChatMessage{
		Role:       "tool",
		Content:    string(content),
		ToolCallID: toolCallID,
		Name:       toolName,
	}
}

// GetToolService returns the underlying tool service for direct access
func (e *ToolExecutor) GetToolService() *ToolService {
	return e.toolService
}
