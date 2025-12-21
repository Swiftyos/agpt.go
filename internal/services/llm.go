package services

import (
	"context"
	"fmt"
	"io"

	"github.com/agpt-go/chatbot-api/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

type LLMService struct {
	client *openai.Client
	model  string
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type StreamChunk struct {
	Content      string
	Done         bool
	FinishReason string
}

type CompletionUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

func NewLLMService(cfg *config.OpenAIConfig) *LLMService {
	client := openai.NewClient(cfg.APIKey)
	return &LLMService{
		client: client,
		model:  cfg.Model,
	}
}

// Chat performs a non-streaming chat completion
func (s *LLMService) Chat(ctx context.Context, messages []ChatMessage, systemPrompt string) (string, *CompletionUsage, error) {
	openaiMessages := s.toOpenAIMessages(messages, systemPrompt)

	resp, err := s.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    s.model,
		Messages: openaiMessages,
	})
	if err != nil {
		return "", nil, fmt.Errorf("chat completion failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", nil, fmt.Errorf("no response choices returned")
	}

	usage := &CompletionUsage{
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
	}

	return resp.Choices[0].Message.Content, usage, nil
}

// ChatStream performs a streaming chat completion
func (s *LLMService) ChatStream(ctx context.Context, messages []ChatMessage, systemPrompt string) (<-chan StreamChunk, error) {
	openaiMessages := s.toOpenAIMessages(messages, systemPrompt)

	stream, err := s.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    s.model,
		Messages: openaiMessages,
		Stream:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	chunks := make(chan StreamChunk)

	go func() {
		defer close(chunks)
		defer stream.Close()

		for {
			// Check for context cancellation before receiving
			select {
			case <-ctx.Done():
				return
			default:
			}

			response, err := stream.Recv()
			if err == io.EOF {
				select {
				case chunks <- StreamChunk{Done: true, FinishReason: "stop"}:
				case <-ctx.Done():
				}
				return
			}
			if err != nil {
				select {
				case chunks <- StreamChunk{Done: true, FinishReason: "error"}:
				case <-ctx.Done():
				}
				return
			}

			if len(response.Choices) > 0 {
				choice := response.Choices[0]
				chunk := StreamChunk{
					Content: choice.Delta.Content,
				}

				if choice.FinishReason != "" {
					chunk.Done = true
					chunk.FinishReason = string(choice.FinishReason)
				}

				// Use select to prevent blocking when context is cancelled
				select {
				case chunks <- chunk:
				case <-ctx.Done():
					return
				}

				if chunk.Done {
					return
				}
			}
		}
	}()

	return chunks, nil
}

func (s *LLMService) toOpenAIMessages(messages []ChatMessage, systemPrompt string) []openai.ChatCompletionMessage {
	var openaiMessages []openai.ChatCompletionMessage

	if systemPrompt != "" {
		openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		})
	}

	for _, msg := range messages {
		role := msg.Role
		// Map role names
		switch role {
		case "user":
			role = openai.ChatMessageRoleUser
		case "assistant":
			role = openai.ChatMessageRoleAssistant
		case "system":
			role = openai.ChatMessageRoleSystem
		}

		openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	return openaiMessages
}

// EstimateTokens provides a rough token count estimate
// For accurate counts, use tiktoken library
func (s *LLMService) EstimateTokens(text string) int {
	// Rough estimate: ~4 characters per token for English
	return len(text) / 4
}
