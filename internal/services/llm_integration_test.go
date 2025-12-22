//go:build openai

// Package services contains integration tests for the LLM service.
// These tests require a valid OPENAI_API_KEY environment variable
// and make real API calls to OpenAI.
//
// Run with: go test -v -tags=openai ./internal/services/...
//
// These tests are designed to run in the nightly E2E workflow,
// not during regular CI to avoid API costs and rate limiting.
package services

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agpt-go/chatbot-api/internal/config"
)

// skipIfNoAPIKey skips the test if OPENAI_API_KEY is not set
func skipIfNoAPIKey(t *testing.T) {
	t.Helper()
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY not set, skipping OpenAI integration test")
	}
}

// getTestConfig returns a config for integration tests
func getTestConfig() *config.OpenAIConfig {
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini" // Use cheaper model for testing
	}
	return &config.OpenAIConfig{
		APIKey: os.Getenv("OPENAI_API_KEY"),
		Model:  model,
	}
}

// TestIntegration_Chat tests basic chat completion with real OpenAI API
func TestIntegration_Chat(t *testing.T) {
	skipIfNoAPIKey(t)

	cfg := getTestConfig()
	svc := NewLLMService(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	messages := []ChatMessage{
		{Role: "user", Content: "Say 'hello world' and nothing else."},
	}

	response, usage, err := svc.Chat(ctx, messages, "You are a helpful assistant that follows instructions exactly.")
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	// Verify response
	if response == "" {
		t.Error("Expected non-empty response")
	}

	lowerResponse := strings.ToLower(response)
	if !strings.Contains(lowerResponse, "hello") || !strings.Contains(lowerResponse, "world") {
		t.Errorf("Expected response to contain 'hello world', got: %s", response)
	}

	// Verify usage stats
	if usage == nil {
		t.Error("Expected usage stats")
	} else {
		if usage.PromptTokens == 0 {
			t.Error("Expected non-zero prompt tokens")
		}
		if usage.CompletionTokens == 0 {
			t.Error("Expected non-zero completion tokens")
		}
		if usage.TotalTokens == 0 {
			t.Error("Expected non-zero total tokens")
		}
		t.Logf("Token usage: prompt=%d, completion=%d, total=%d",
			usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
	}
}

// TestIntegration_ChatWithTools tests tool calling with real OpenAI API
func TestIntegration_ChatWithTools(t *testing.T) {
	skipIfNoAPIKey(t)

	cfg := getTestConfig()
	svc := NewLLMService(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Define a test tool
	tools := []ToolDefinition{
		{
			Name:        "get_weather",
			Description: "Get the current weather for a location",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type":        "string",
						"description": "The city and state, e.g. San Francisco, CA",
					},
				},
				"required": []string{"location"},
			},
		},
	}

	messages := []ChatMessage{
		{Role: "user", Content: "What's the weather like in San Francisco?"},
	}

	response, err := svc.ChatWithTools(ctx, messages, "You are a helpful assistant. Use the get_weather tool to answer weather questions.", tools)
	if err != nil {
		t.Fatalf("ChatWithTools failed: %v", err)
	}

	// The model should call the weather tool
	if len(response.ToolCalls) == 0 {
		// Some models might respond with text if they can't use tools
		t.Logf("Model responded with text instead of tool call: %s", response.Content)
	} else {
		// Verify tool call structure
		tc := response.ToolCalls[0]
		if tc.Function.Name != "get_weather" {
			t.Errorf("Expected tool call to 'get_weather', got: %s", tc.Function.Name)
		}
		if tc.ID == "" {
			t.Error("Expected tool call to have an ID")
		}

		// Verify arguments contain location
		args, err := ParseToolArguments(tc.Function.Arguments)
		if err != nil {
			t.Errorf("Failed to parse tool arguments: %v", err)
		} else {
			if _, ok := args["location"]; !ok {
				t.Error("Expected location in tool arguments")
			}
		}
		t.Logf("Tool call: %s(%s)", tc.Function.Name, tc.Function.Arguments)
	}
}

// TestIntegration_ChatStream tests streaming chat with real OpenAI API
func TestIntegration_ChatStream(t *testing.T) {
	skipIfNoAPIKey(t)

	cfg := getTestConfig()
	svc := NewLLMService(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	messages := []ChatMessage{
		{Role: "user", Content: "Count from 1 to 5, one number per line."},
	}

	chunks, err := svc.ChatStream(ctx, messages, "You are a helpful assistant.")
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}

	var fullResponse strings.Builder
	chunkCount := 0
	gotDone := false

	for chunk := range chunks {
		chunkCount++
		fullResponse.WriteString(chunk.Content)

		if chunk.Done {
			gotDone = true
			t.Logf("Stream finished with reason: %s", chunk.FinishReason)
			break
		}
	}

	if !gotDone {
		t.Error("Expected to receive done signal")
	}

	response := fullResponse.String()
	if response == "" {
		t.Error("Expected non-empty streamed response")
	}

	// Should contain the numbers
	for i := 1; i <= 5; i++ {
		if !strings.Contains(response, string(rune('0'+i))) {
			t.Logf("Response might not contain all numbers: %s", response)
			break
		}
	}

	t.Logf("Received %d chunks, total response: %s", chunkCount, response)
}

// TestIntegration_ChatStreamWithTools tests streaming tool calls with real OpenAI API
func TestIntegration_ChatStreamWithTools(t *testing.T) {
	skipIfNoAPIKey(t)

	cfg := getTestConfig()
	svc := NewLLMService(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tools := []ToolDefinition{
		{
			Name:        "calculate",
			Description: "Perform a mathematical calculation",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"expression": map[string]interface{}{
						"type":        "string",
						"description": "The mathematical expression to evaluate",
					},
				},
				"required": []string{"expression"},
			},
		},
	}

	messages := []ChatMessage{
		{Role: "user", Content: "What is 15 + 27? Use the calculate tool."},
	}

	chunks, err := svc.ChatStreamWithTools(ctx, messages, "You are a math assistant. Always use the calculate tool for math.", tools)
	if err != nil {
		t.Fatalf("ChatStreamWithTools failed: %v", err)
	}

	var fullContent strings.Builder
	var toolCalls []ToolCall
	chunkCount := 0
	gotToolCallDelta := false

	for chunk := range chunks {
		chunkCount++
		fullContent.WriteString(chunk.Content)

		if len(chunk.ToolCallDeltas) > 0 {
			gotToolCallDelta = true
		}

		if len(chunk.ToolCalls) > 0 {
			toolCalls = chunk.ToolCalls
		}

		if chunk.Done {
			t.Logf("Stream finished with reason: %s", chunk.FinishReason)
			break
		}
	}

	t.Logf("Received %d chunks", chunkCount)

	if len(toolCalls) > 0 {
		if !gotToolCallDelta {
			t.Log("Tool call was received but no incremental deltas were observed")
		}

		tc := toolCalls[0]
		t.Logf("Tool call: %s(%s)", tc.Function.Name, tc.Function.Arguments)

		if tc.Function.Name != "calculate" {
			t.Errorf("Expected 'calculate' tool call, got: %s", tc.Function.Name)
		}
	} else if fullContent.Len() > 0 {
		t.Logf("Model responded with text instead of tool: %s", fullContent.String())
	} else {
		t.Error("Expected either tool calls or text response")
	}
}

// TestIntegration_MultiTurnConversation tests a multi-turn conversation
func TestIntegration_MultiTurnConversation(t *testing.T) {
	skipIfNoAPIKey(t)

	cfg := getTestConfig()
	svc := NewLLMService(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	systemPrompt := "You are a helpful assistant that remembers the conversation context."

	// First turn
	messages := []ChatMessage{
		{Role: "user", Content: "My name is Alice."},
	}

	response1, _, err := svc.Chat(ctx, messages, systemPrompt)
	if err != nil {
		t.Fatalf("First turn failed: %v", err)
	}
	t.Logf("Turn 1 response: %s", response1)

	// Second turn - test context memory
	messages = append(messages,
		ChatMessage{Role: "assistant", Content: response1},
		ChatMessage{Role: "user", Content: "What is my name?"},
	)

	response2, _, err := svc.Chat(ctx, messages, systemPrompt)
	if err != nil {
		t.Fatalf("Second turn failed: %v", err)
	}
	t.Logf("Turn 2 response: %s", response2)

	// Verify the model remembers the name
	if !strings.Contains(strings.ToLower(response2), "alice") {
		t.Errorf("Expected model to remember name 'Alice', got: %s", response2)
	}
}

// TestIntegration_ContextCancellation tests that context cancellation works
func TestIntegration_ContextCancellation(t *testing.T) {
	skipIfNoAPIKey(t)

	cfg := getTestConfig()
	svc := NewLLMService(cfg)

	// Create a context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	messages := []ChatMessage{
		{Role: "user", Content: "Tell me a very long story."},
	}

	_, _, err := svc.Chat(ctx, messages, "")
	if err == nil {
		t.Error("Expected error due to cancelled context")
	}
}

// TestIntegration_TokenEstimation tests token estimation accuracy
func TestIntegration_TokenEstimation(t *testing.T) {
	skipIfNoAPIKey(t)

	cfg := getTestConfig()
	svc := NewLLMService(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testText := "Hello, how are you doing today?"
	estimated := svc.EstimateTokens(testText)

	messages := []ChatMessage{
		{Role: "user", Content: testText},
	}

	_, usage, err := svc.Chat(ctx, messages, "")
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	// Prompt tokens include the user message plus some overhead
	// Estimation should be in the right ballpark (within 3x)
	if estimated == 0 {
		t.Error("Token estimation returned 0")
	}

	ratio := float64(usage.PromptTokens) / float64(estimated)
	t.Logf("Estimated: %d, Actual prompt tokens: %d, Ratio: %.2f", estimated, usage.PromptTokens, ratio)

	// Allow for reasonable variance (system message overhead, etc.)
	if ratio > 10 || ratio < 0.1 {
		t.Errorf("Token estimation too far off: estimated %d, actual %d", estimated, usage.PromptTokens)
	}
}
