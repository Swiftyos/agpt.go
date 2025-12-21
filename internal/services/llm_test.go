package services

import (
	"testing"

	"github.com/agpt-go/chatbot-api/internal/config"
)

func TestNewLLMService(t *testing.T) {
	cfg := &config.OpenAIConfig{
		APIKey: "test-api-key",
		Model:  "gpt-4o",
	}

	svc := NewLLMService(cfg)

	if svc == nil {
		t.Fatal("NewLLMService() returned nil")
	}
	if svc.model != "gpt-4o" {
		t.Errorf("LLMService.model = %q, want %q", svc.model, "gpt-4o")
	}
	if svc.client == nil {
		t.Error("LLMService.client should not be nil")
	}
}

func TestEstimateTokens(t *testing.T) {
	cfg := &config.OpenAIConfig{
		APIKey: "test-api-key",
		Model:  "gpt-4o",
	}
	svc := NewLLMService(cfg)

	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "empty string",
			text:     "",
			expected: 0,
		},
		{
			name:     "short text",
			text:     "Hello",
			expected: 1, // 5 chars / 4 = 1
		},
		{
			name:     "medium text",
			text:     "Hello, world! This is a test.",
			expected: 7, // 29 chars / 4 = 7
		},
		{
			name:     "long text",
			text:     "The quick brown fox jumps over the lazy dog. This is a longer sentence to test token estimation.",
			expected: 24, // 97 chars / 4 = 24
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.EstimateTokens(tt.text)
			if got != tt.expected {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tt.text, got, tt.expected)
			}
		})
	}
}

func TestToOpenAIMessages(t *testing.T) {
	cfg := &config.OpenAIConfig{
		APIKey: "test-api-key",
		Model:  "gpt-4o",
	}
	svc := NewLLMService(cfg)

	t.Run("without system prompt", func(t *testing.T) {
		messages := []ChatMessage{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		}

		result := svc.toOpenAIMessages(messages, "")

		if len(result) != 2 {
			t.Errorf("toOpenAIMessages() returned %d messages, want 2", len(result))
		}
	})

	t.Run("with system prompt", func(t *testing.T) {
		messages := []ChatMessage{
			{Role: "user", Content: "Hello"},
		}

		result := svc.toOpenAIMessages(messages, "You are a helpful assistant.")

		if len(result) != 2 {
			t.Errorf("toOpenAIMessages() returned %d messages, want 2", len(result))
		}
		if result[0].Role != "system" {
			t.Errorf("First message role = %q, want %q", result[0].Role, "system")
		}
		if result[0].Content != "You are a helpful assistant." {
			t.Errorf("First message content = %q, want %q", result[0].Content, "You are a helpful assistant.")
		}
	})

	t.Run("role mapping", func(t *testing.T) {
		messages := []ChatMessage{
			{Role: "user", Content: "User message"},
			{Role: "assistant", Content: "Assistant message"},
			{Role: "system", Content: "System message"},
		}

		result := svc.toOpenAIMessages(messages, "")

		if len(result) != 3 {
			t.Errorf("toOpenAIMessages() returned %d messages, want 3", len(result))
		}
		if result[0].Role != "user" {
			t.Errorf("First message role = %q, want %q", result[0].Role, "user")
		}
		if result[1].Role != "assistant" {
			t.Errorf("Second message role = %q, want %q", result[1].Role, "assistant")
		}
		if result[2].Role != "system" {
			t.Errorf("Third message role = %q, want %q", result[2].Role, "system")
		}
	})

	t.Run("empty messages", func(t *testing.T) {
		result := svc.toOpenAIMessages([]ChatMessage{}, "")

		if len(result) != 0 {
			t.Errorf("toOpenAIMessages() returned %d messages, want 0", len(result))
		}
	})

	t.Run("empty messages with system prompt", func(t *testing.T) {
		result := svc.toOpenAIMessages([]ChatMessage{}, "System prompt")

		if len(result) != 1 {
			t.Errorf("toOpenAIMessages() returned %d messages, want 1", len(result))
		}
	})
}

func TestChatMessageStruct(t *testing.T) {
	msg := ChatMessage{
		Role:    "user",
		Content: "Hello, world!",
	}

	if msg.Role != "user" {
		t.Errorf("ChatMessage.Role = %q, want %q", msg.Role, "user")
	}
	if msg.Content != "Hello, world!" {
		t.Errorf("ChatMessage.Content = %q, want %q", msg.Content, "Hello, world!")
	}
}

func TestStreamChunkStruct(t *testing.T) {
	chunk := StreamChunk{
		Content:      "Hello",
		Done:         false,
		FinishReason: "",
	}

	if chunk.Content != "Hello" {
		t.Errorf("StreamChunk.Content = %q, want %q", chunk.Content, "Hello")
	}
	if chunk.Done {
		t.Error("StreamChunk.Done should be false")
	}
	if chunk.FinishReason != "" {
		t.Errorf("StreamChunk.FinishReason = %q, want empty string", chunk.FinishReason)
	}

	// Test done chunk
	doneChunk := StreamChunk{
		Content:      "",
		Done:         true,
		FinishReason: "stop",
	}

	if !doneChunk.Done {
		t.Error("StreamChunk.Done should be true")
	}
	if doneChunk.FinishReason != "stop" {
		t.Errorf("StreamChunk.FinishReason = %q, want %q", doneChunk.FinishReason, "stop")
	}
}

func TestCompletionUsageStruct(t *testing.T) {
	usage := CompletionUsage{
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
	}

	if usage.PromptTokens != 10 {
		t.Errorf("CompletionUsage.PromptTokens = %d, want %d", usage.PromptTokens, 10)
	}
	if usage.CompletionTokens != 20 {
		t.Errorf("CompletionUsage.CompletionTokens = %d, want %d", usage.CompletionTokens, 20)
	}
	if usage.TotalTokens != 30 {
		t.Errorf("CompletionUsage.TotalTokens = %d, want %d", usage.TotalTokens, 30)
	}
}
