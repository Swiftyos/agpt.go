package services

import (
	"context"
	"fmt"

	"github.com/agpt-go/chatbot-api/internal/database"
	"github.com/google/uuid"
)

type ChatService struct {
	queries      *database.Queries
	llmService   *LLMService
	toolService  *ToolService
	toolExecutor *ToolExecutor
}

func NewChatService(queries *database.Queries, llmService *LLMService) *ChatService {
	toolService := NewToolService(queries)
	toolExecutor := NewToolExecutor(toolService)
	return &ChatService{
		queries:      queries,
		llmService:   llmService,
		toolService:  toolService,
		toolExecutor: toolExecutor,
	}
}

// GetToolExecutor returns the tool executor for handling tool calls
func (s *ChatService) GetToolExecutor() *ToolExecutor {
	return s.toolExecutor
}

// GetToolService returns the tool service for direct access
func (s *ChatService) GetToolService() *ToolService {
	return s.toolService
}

// GetAvailableTools returns all available tool definitions
func (s *ChatService) GetAvailableTools() []ToolDefinition {
	return GetAllToolDefinitions()
}

type CreateSessionInput struct {
	Title        string
	Model        string
	SystemPrompt string
}

type SendMessageInput struct {
	SessionID uuid.UUID
	Content   string
}

func (s *ChatService) CreateSession(ctx context.Context, userID uuid.UUID, input CreateSessionInput) (*database.ChatSession, error) {
	title := input.Title
	if title == "" {
		title = "New Chat"
	}

	model := input.Model
	if model == "" {
		model = "gpt-4o"
	}

	session, err := s.queries.CreateChatSession(ctx, database.CreateChatSessionParams{
		UserID:       userID,
		Title:        &title,
		Model:        &model,
		SystemPrompt: &input.SystemPrompt,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &session, nil
}

func (s *ChatService) GetSession(ctx context.Context, sessionID, userID uuid.UUID) (*database.ChatSession, error) {
	session, err := s.queries.GetChatSessionByUser(ctx, database.GetChatSessionByUserParams{
		ID:     sessionID,
		UserID: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	return &session, nil
}

func (s *ChatService) ListSessions(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]database.ChatSession, error) {
	sessions, err := s.queries.ListChatSessions(ctx, database.ListChatSessionsParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	return sessions, nil
}

func (s *ChatService) UpdateSession(ctx context.Context, sessionID uuid.UUID, title, systemPrompt *string) (*database.ChatSession, error) {
	session, err := s.queries.UpdateChatSession(ctx, database.UpdateChatSessionParams{
		ID:           sessionID,
		Title:        title,
		SystemPrompt: systemPrompt,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}
	return &session, nil
}

func (s *ChatService) DeleteSession(ctx context.Context, sessionID, userID uuid.UUID) error {
	return s.queries.DeleteChatSession(ctx, database.DeleteChatSessionParams{
		ID:     sessionID,
		UserID: userID,
	})
}

// MaxMessageHistoryLimit is the maximum number of messages to include in LLM context
const MaxMessageHistoryLimit = 100

// GetMessages returns messages for a session with an optional limit
// If limit <= 0, returns all messages (use with caution for large histories)
func (s *ChatService) GetMessages(ctx context.Context, sessionID uuid.UUID, limit int) ([]database.ChatMessage, error) {
	if limit <= 0 {
		// Return all messages (for backward compatibility, but discouraged)
		messages, err := s.queries.GetChatMessages(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to get messages: %w", err)
		}
		return messages, nil
	}

	// Get recent messages with limit (returns DESC order)
	messages, err := s.queries.GetRecentChatMessages(ctx, database.GetRecentChatMessagesParams{
		SessionID: sessionID,
		Limit:     int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	// Reverse to chronological order (ASC)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

func (s *ChatService) SaveMessage(ctx context.Context, sessionID uuid.UUID, role, content string, tokensUsed int) (*database.ChatMessage, error) {
	tokens := int32(tokensUsed)
	message, err := s.queries.CreateChatMessage(ctx, database.CreateChatMessageParams{
		SessionID:  sessionID,
		Role:       role,
		Content:    content,
		TokensUsed: &tokens,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to save message: %w", err)
	}
	return &message, nil
}

// SendMessage saves the user message and generates a response
func (s *ChatService) SendMessage(ctx context.Context, sessionID, userID uuid.UUID, content string) (*database.ChatMessage, *database.ChatMessage, error) {
	// Verify session ownership
	session, err := s.GetSession(ctx, sessionID, userID)
	if err != nil {
		return nil, nil, err
	}

	// Save user message
	userTokens := s.llmService.EstimateTokens(content)
	userMsg, err := s.SaveMessage(ctx, sessionID, "user", content, userTokens)
	if err != nil {
		return nil, nil, err
	}

	// Get chat history (limit to recent messages for LLM context window)
	messages, err := s.GetMessages(ctx, sessionID, MaxMessageHistoryLimit)
	if err != nil {
		return nil, nil, err
	}

	// Convert to LLM format
	llmMessages := make([]ChatMessage, len(messages))
	for i, msg := range messages {
		llmMessages[i] = ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Build system prompt with business context
	systemPrompt := s.buildEnhancedSystemPrompt(ctx, userID, session.SystemPrompt)

	// Get available tools
	tools := s.GetAvailableTools()

	// Generate response with tools
	chatResp, err := s.llmService.ChatWithTools(ctx, llmMessages, systemPrompt, tools)
	if err != nil {
		return userMsg, nil, fmt.Errorf("failed to generate response: %w", err)
	}

	// Save assistant message
	assistantTokens := 0
	if chatResp.Usage != nil {
		assistantTokens = chatResp.Usage.CompletionTokens
	}
	assistantMsg, err := s.SaveMessage(ctx, sessionID, "assistant", chatResp.Content, assistantTokens)
	if err != nil {
		return userMsg, nil, err
	}

	return userMsg, assistantMsg, nil
}

// SendMessageStream saves the user message and streams the response
func (s *ChatService) SendMessageStream(ctx context.Context, sessionID, userID uuid.UUID, content string) (*database.ChatMessage, <-chan StreamChunk, error) {
	// Verify session ownership
	session, err := s.GetSession(ctx, sessionID, userID)
	if err != nil {
		return nil, nil, err
	}

	// Save user message
	userTokens := s.llmService.EstimateTokens(content)
	userMsg, err := s.SaveMessage(ctx, sessionID, "user", content, userTokens)
	if err != nil {
		return nil, nil, err
	}

	// Get chat history (limit to recent messages for LLM context window)
	messages, err := s.GetMessages(ctx, sessionID, MaxMessageHistoryLimit)
	if err != nil {
		return nil, nil, err
	}

	// Convert to LLM format
	llmMessages := make([]ChatMessage, len(messages))
	for i, msg := range messages {
		llmMessages[i] = ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Build system prompt with business context
	systemPrompt := s.buildEnhancedSystemPrompt(ctx, userID, session.SystemPrompt)

	// Get available tools
	tools := s.GetAvailableTools()

	// Start streaming with tools
	chunks, err := s.llmService.ChatStreamWithTools(ctx, llmMessages, systemPrompt, tools)
	if err != nil {
		return userMsg, nil, err
	}

	return userMsg, chunks, nil
}

// buildEnhancedSystemPrompt builds a system prompt enhanced with business context
func (s *ChatService) buildEnhancedSystemPrompt(ctx context.Context, userID uuid.UUID, basePrompt *string) string {
	var prompt string
	if basePrompt != nil {
		prompt = *basePrompt
	}

	// Add business context if available
	businessContext, err := s.toolService.BuildSystemPromptContext(ctx, userID)
	if err == nil && businessContext != "" {
		if prompt != "" {
			prompt = prompt + "\n" + businessContext
		} else {
			prompt = businessContext
		}
	}

	return prompt
}

// SaveStreamedResponse saves the accumulated response after streaming completes
func (s *ChatService) SaveStreamedResponse(ctx context.Context, sessionID uuid.UUID, content string) (*database.ChatMessage, error) {
	tokens := s.llmService.EstimateTokens(content)
	return s.SaveMessage(ctx, sessionID, "assistant", content, tokens)
}
