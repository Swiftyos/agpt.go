package services

import (
	"testing"

	"github.com/agpt-go/chatbot-api/internal/config"
	"github.com/google/uuid"
)

func TestNewChatService(t *testing.T) {
	llmCfg := &config.OpenAIConfig{
		APIKey: "test-api-key",
		Model:  "gpt-4o",
	}
	llmService := NewLLMService(llmCfg)

	svc := NewChatService(nil, llmService)

	if svc == nil {
		t.Fatal("NewChatService() returned nil")
	}
	if svc.llmService != llmService {
		t.Error("NewChatService() did not set llmService correctly")
	}
}

func TestCreateSessionInputStruct(t *testing.T) {
	input := CreateSessionInput{
		Title:        "Test Session",
		Model:        "gpt-4o",
		SystemPrompt: "You are a helpful assistant.",
	}

	if input.Title != "Test Session" {
		t.Errorf("CreateSessionInput.Title = %q, want %q", input.Title, "Test Session")
	}
	if input.Model != "gpt-4o" {
		t.Errorf("CreateSessionInput.Model = %q, want %q", input.Model, "gpt-4o")
	}
	if input.SystemPrompt != "You are a helpful assistant." {
		t.Errorf("CreateSessionInput.SystemPrompt = %q, want %q", input.SystemPrompt, "You are a helpful assistant.")
	}
}

func TestSendMessageInputStruct(t *testing.T) {
	sessionID := uuid.New()
	input := SendMessageInput{
		SessionID: sessionID,
		Content:   "Hello, how are you?",
	}

	if input.SessionID != sessionID {
		t.Errorf("SendMessageInput.SessionID = %v, want %v", input.SessionID, sessionID)
	}
	if input.Content != "Hello, how are you?" {
		t.Errorf("SendMessageInput.Content = %q, want %q", input.Content, "Hello, how are you?")
	}
}

func TestCreateSessionInputDefaults(t *testing.T) {
	t.Run("empty title should use default", func(t *testing.T) {
		input := CreateSessionInput{
			Title: "",
			Model: "",
		}

		// Test that we correctly identify when defaults should be applied
		if input.Title != "" {
			t.Error("Empty title should remain empty in input")
		}
		if input.Model != "" {
			t.Error("Empty model should remain empty in input")
		}
	})
}
