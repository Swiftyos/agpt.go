package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// ToolExecutor handles the execution of tool calls from the LLM
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
type ToolExecutionResult struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ExecuteTool executes a tool call and returns the result
func (e *ToolExecutor) ExecuteTool(ctx context.Context, userID uuid.UUID, toolName string, arguments string) (*ToolExecutionResult, error) {
	switch toolName {
	case "add_understanding":
		return e.executeAddUnderstanding(ctx, userID, arguments)
	case "generate_business_report":
		return e.executeGenerateBusinessReport(ctx, userID, arguments)
	default:
		return &ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", toolName),
		}, nil
	}
}

// ExecuteToolCall is a convenience method that takes a ToolCall directly
func (e *ToolExecutor) ExecuteToolCall(ctx context.Context, userID uuid.UUID, toolCall ToolCall) (*ToolExecutionResult, error) {
	return e.ExecuteTool(ctx, userID, toolCall.Function.Name, toolCall.Function.Arguments)
}

func (e *ToolExecutor) executeAddUnderstanding(ctx context.Context, userID uuid.UUID, arguments string) (*ToolExecutionResult, error) {
	var input AddUnderstandingInput
	if err := json.Unmarshal([]byte(arguments), &input); err != nil {
		return &ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("invalid arguments: %v", err),
		}, nil
	}

	response, err := e.toolService.ExecuteAddUnderstanding(ctx, userID, input)
	if err != nil {
		return &ToolExecutionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &ToolExecutionResult{
		Success: true,
		Result:  response,
	}, nil
}

func (e *ToolExecutor) executeGenerateBusinessReport(ctx context.Context, userID uuid.UUID, arguments string) (*ToolExecutionResult, error) {
	var input GenerateBusinessReportInput
	if err := json.Unmarshal([]byte(arguments), &input); err != nil {
		return &ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("invalid arguments: %v", err),
		}, nil
	}

	response, err := e.toolService.ExecuteGenerateBusinessReport(ctx, userID, input)
	if err != nil {
		return &ToolExecutionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &ToolExecutionResult{
		Success: true,
		Result:  response,
	}, nil
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
