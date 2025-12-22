package services

import (
	"context"
	"encoding/json"
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
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolCall represents a tool call made by the assistant
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ToolDefinition defines a tool that can be called by the LLM
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolCallDelta represents incremental tool call data from streaming
type ToolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Name     string `json:"name,omitempty"`
	ArgDelta string `json:"arg_delta,omitempty"`
}

// StreamChunk represents a chunk of streaming response data
type StreamChunk struct {
	// Text content delta
	Content string

	// Tool call deltas (streamed incrementally)
	ToolCallDeltas []ToolCallDelta

	// Completed tool calls (when fully assembled)
	ToolCalls []ToolCall

	// Stream status
	Done         bool
	FinishReason string

	// Usage statistics (available at end of stream)
	Usage *CompletionUsage
}

// ChunkType indicates what type of content the chunk contains
type ChunkType int

const (
	ChunkTypeText ChunkType = iota
	ChunkTypeToolCallStart
	ChunkTypeToolCallDelta
	ChunkTypeToolCallComplete
	ChunkTypeDone
)

// GetChunkType returns the type of content in this chunk
func (c *StreamChunk) GetChunkType() ChunkType {
	if c.Done {
		return ChunkTypeDone
	}
	if len(c.ToolCalls) > 0 {
		return ChunkTypeToolCallComplete
	}
	if len(c.ToolCallDeltas) > 0 {
		for _, delta := range c.ToolCallDeltas {
			if delta.ID != "" && delta.Name != "" {
				return ChunkTypeToolCallStart
			}
		}
		return ChunkTypeToolCallDelta
	}
	return ChunkTypeText
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

// ChatResponse contains the full response from a chat completion
type ChatResponse struct {
	Content   string
	ToolCalls []ToolCall
	Usage     *CompletionUsage
}

// Chat performs a non-streaming chat completion
func (s *LLMService) Chat(ctx context.Context, messages []ChatMessage, systemPrompt string) (string, *CompletionUsage, error) {
	resp, err := s.ChatWithTools(ctx, messages, systemPrompt, nil)
	if err != nil {
		return "", nil, err
	}
	return resp.Content, resp.Usage, nil
}

// ChatWithTools performs a non-streaming chat completion with tool support
func (s *LLMService) ChatWithTools(ctx context.Context, messages []ChatMessage, systemPrompt string, tools []ToolDefinition) (*ChatResponse, error) {
	openaiMessages := s.toOpenAIMessages(messages, systemPrompt)
	openaiTools := s.toOpenAITools(tools)

	req := openai.ChatCompletionRequest{
		Model:    s.model,
		Messages: openaiMessages,
	}

	if len(openaiTools) > 0 {
		req.Tools = openaiTools
	}

	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("chat completion failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned")
	}

	choice := resp.Choices[0]
	result := &ChatResponse{
		Content: choice.Message.Content,
		Usage: &CompletionUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	// Convert tool calls
	if len(choice.Message.ToolCalls) > 0 {
		result.ToolCalls = make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			result.ToolCalls[i] = ToolCall{
				ID:   tc.ID,
				Type: string(tc.Type),
			}
			result.ToolCalls[i].Function.Name = tc.Function.Name
			result.ToolCalls[i].Function.Arguments = tc.Function.Arguments
		}
	}

	return result, nil
}

// ChatStream performs a streaming chat completion
func (s *LLMService) ChatStream(ctx context.Context, messages []ChatMessage, systemPrompt string) (<-chan StreamChunk, error) {
	return s.ChatStreamWithTools(ctx, messages, systemPrompt, nil)
}

// toolCallAccumulator tracks tool call data as it streams in
type toolCallAccumulator struct {
	id        string
	toolType  string
	name      string
	arguments string
}

// ChatStreamWithTools performs a streaming chat completion with tool support
func (s *LLMService) ChatStreamWithTools(ctx context.Context, messages []ChatMessage, systemPrompt string, tools []ToolDefinition) (<-chan StreamChunk, error) {
	openaiMessages := s.toOpenAIMessages(messages, systemPrompt)
	openaiTools := s.toOpenAITools(tools)

	req := openai.ChatCompletionRequest{
		Model:    s.model,
		Messages: openaiMessages,
		Stream:   true,
	}

	if len(openaiTools) > 0 {
		req.Tools = openaiTools
	}

	stream, err := s.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	// Buffer size prevents blocking on slow consumers and reduces goroutine leak risk
	chunks := make(chan StreamChunk, 10)

	go func() {
		defer close(chunks)
		defer stream.Close()

		// toolCalls accumulates streaming tool call data (goroutine-local, no sync needed)
		toolCalls := make(map[int]*toolCallAccumulator)

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

				// Process tool calls from delta
				if len(choice.Delta.ToolCalls) > 0 {
					for _, tc := range choice.Delta.ToolCalls {
						// Get the index (default to 0 if nil)
						idx := 0
						if tc.Index != nil {
							idx = *tc.Index
						}

						// Initialize accumulator if this is a new tool call
						if _, exists := toolCalls[idx]; !exists {
							toolCalls[idx] = &toolCallAccumulator{}
						}

						acc := toolCalls[idx]

						// Update accumulator with new data
						if tc.ID != "" {
							acc.id = tc.ID
						}
						if tc.Type != "" {
							acc.toolType = string(tc.Type)
						}
						if tc.Function.Name != "" {
							acc.name = tc.Function.Name
						}
						if tc.Function.Arguments != "" {
							acc.arguments += tc.Function.Arguments
						}

						// Create delta for streaming to client
						delta := ToolCallDelta{
							Index:    idx,
							ID:       tc.ID,
							Type:     string(tc.Type),
							Name:     tc.Function.Name,
							ArgDelta: tc.Function.Arguments,
						}
						chunk.ToolCallDeltas = append(chunk.ToolCallDeltas, delta)
					}
				}

				// Check for finish reason
				if choice.FinishReason != "" {
					chunk.Done = true
					chunk.FinishReason = string(choice.FinishReason)

					// If finishing with tool_calls, assemble complete tool calls
					if choice.FinishReason == openai.FinishReasonToolCalls {
						for _, acc := range toolCalls {
							tc := ToolCall{
								ID:   acc.id,
								Type: acc.toolType,
							}
							tc.Function.Name = acc.name
							tc.Function.Arguments = acc.arguments
							chunk.ToolCalls = append(chunk.ToolCalls, tc)
						}
					}
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
		case "tool":
			role = openai.ChatMessageRoleTool
		}

		openaiMsg := openai.ChatCompletionMessage{
			Role:    role,
			Content: msg.Content,
		}

		// Handle tool call results
		if msg.ToolCallID != "" {
			openaiMsg.ToolCallID = msg.ToolCallID
		}
		if msg.Name != "" {
			openaiMsg.Name = msg.Name
		}

		// Handle assistant messages with tool calls
		if len(msg.ToolCalls) > 0 {
			openaiMsg.ToolCalls = make([]openai.ToolCall, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				openaiMsg.ToolCalls[i] = openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolType(tc.Type),
					Function: openai.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}

		openaiMessages = append(openaiMessages, openaiMsg)
	}

	return openaiMessages
}

// toOpenAITools converts tool definitions to OpenAI format
func (s *LLMService) toOpenAITools(tools []ToolDefinition) []openai.Tool {
	if len(tools) == 0 {
		return nil
	}

	openaiTools := make([]openai.Tool, len(tools))
	for i, tool := range tools {
		openaiTools[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		}
	}
	return openaiTools
}

// ParseToolArguments parses JSON arguments string into a map
func ParseToolArguments(arguments string) (map[string]interface{}, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
	}
	return args, nil
}

// EstimateTokens provides a rough token count estimate
// For accurate counts, use tiktoken library
func (s *LLMService) EstimateTokens(text string) int {
	// Rough estimate: ~4 characters per token for English
	return len(text) / 4
}
