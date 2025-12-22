package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/agpt-go/chatbot-api/internal/database"
	"github.com/agpt-go/chatbot-api/internal/logging"
	"github.com/agpt-go/chatbot-api/internal/middleware"
	"github.com/agpt-go/chatbot-api/internal/services"
	"github.com/agpt-go/chatbot-api/internal/streaming"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type ChatHandler struct {
	chatService ChatServicer
	validate    Validator
}

func NewChatHandler(chatService ChatServicer) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		validate:    validator.New(),
	}
}

type CreateSessionRequest struct {
	Title        string `json:"title"`
	Model        string `json:"model"`
	SystemPrompt string `json:"system_prompt"`
}

type UpdateSessionRequest struct {
	Title        *string `json:"title"`
	SystemPrompt *string `json:"system_prompt"`
}

type SendMessageRequest struct {
	Content string `json:"content" validate:"required"`
}

type SessionResponse struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Model        string  `json:"model"`
	SystemPrompt *string `json:"system_prompt,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

type MessageResponse struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type ChatResponse struct {
	UserMessage      *MessageResponse `json:"user_message"`
	AssistantMessage *MessageResponse `json:"assistant_message"`
}

// Helper functions for type conversions
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func formatTimestamp(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.Format(time.RFC3339)
}

func sessionToResponse(session *database.ChatSession) SessionResponse {
	return SessionResponse{
		ID:           session.ID.String(),
		Title:        derefString(session.Title),
		Model:        derefString(session.Model),
		SystemPrompt: session.SystemPrompt,
		CreatedAt:    formatTimestamp(session.CreatedAt),
		UpdatedAt:    formatTimestamp(session.UpdatedAt),
	}
}

func messageToResponse(msg *database.ChatMessage) MessageResponse {
	return MessageResponse{
		ID:        msg.ID.String(),
		Role:      msg.Role,
		Content:   msg.Content,
		CreatedAt: formatTimestamp(msg.CreatedAt),
	}
}

// CreateSession creates a new chat session
func (h *ChatHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Empty body is OK, use defaults
		req = CreateSessionRequest{}
	}

	session, err := h.chatService.CreateSession(r.Context(), userID, services.CreateSessionInput{
		Title:        req.Title,
		Model:        req.Model,
		SystemPrompt: req.SystemPrompt,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	writeJSON(w, http.StatusCreated, sessionToResponse(session))
}

// ListSessions returns all sessions for the authenticated user
func (h *ChatHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	limit := int32(20)
	offset := int32(0)

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = int32(parsed)
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = int32(parsed)
		}
	}

	sessions, err := h.chatService.ListSessions(r.Context(), userID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list sessions")
		return
	}

	response := make([]SessionResponse, len(sessions))
	for i, s := range sessions {
		response[i] = sessionToResponse(&s)
	}

	writeJSON(w, http.StatusOK, response)
}

// GetSession returns a specific session
func (h *ChatHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid session ID")
		return
	}

	session, err := h.chatService.GetSession(r.Context(), sessionID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Session not found")
		return
	}

	writeJSON(w, http.StatusOK, sessionToResponse(session))
}

// UpdateSession updates a session's title or system prompt
func (h *ChatHandler) UpdateSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid session ID")
		return
	}

	// Verify ownership
	_, err = h.chatService.GetSession(r.Context(), sessionID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Session not found")
		return
	}

	var req UpdateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	session, err := h.chatService.UpdateSession(r.Context(), sessionID, req.Title, req.SystemPrompt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update session")
		return
	}

	writeJSON(w, http.StatusOK, sessionToResponse(session))
}

// DeleteSession deletes a session and all its messages
func (h *ChatHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid session ID")
		return
	}

	if err := h.chatService.DeleteSession(r.Context(), sessionID, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete session")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetMessages returns messages in a session with optional limit
func (h *ChatHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid session ID")
		return
	}

	// Parse limit from query params (default: 100, max: 500)
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
			if limit > 500 {
				limit = 500
			}
		}
	}

	// Verify ownership
	_, err = h.chatService.GetSession(r.Context(), sessionID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Session not found")
		return
	}

	messages, err := h.chatService.GetMessages(r.Context(), sessionID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get messages")
		return
	}

	response := make([]MessageResponse, len(messages))
	for i, m := range messages {
		response[i] = messageToResponse(&m)
	}

	writeJSON(w, http.StatusOK, response)
}

// SendMessage sends a message and returns the response (non-streaming)
func (h *ChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid session ID")
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	userMsg, assistantMsg, err := h.chatService.SendMessage(r.Context(), sessionID, userID, req.Content)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "Session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to send message")
		return
	}

	userMsgResp := messageToResponse(userMsg)
	response := ChatResponse{
		UserMessage: &userMsgResp,
	}

	if assistantMsg != nil {
		assistantMsgResp := messageToResponse(assistantMsg)
		response.AssistantMessage = &assistantMsgResp
	}

	writeJSON(w, http.StatusOK, response)
}

// SendMessageStream sends a message and streams the response
// Implements AI SDK Data Stream Protocol with tool calling support
func (h *ChatHandler) SendMessageStream(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid session ID")
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	userMsg, chunks, err := h.chatService.SendMessageStream(r.Context(), sessionID, userID, req.Content)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "Session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to send message")
		return
	}

	// Initialize stream writer
	sw, err := streaming.NewStreamWriter(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Streaming not supported")
		return
	}
	defer sw.Close()

	// Write message start
	messageID := uuid.New().String()
	if err := sw.WriteStart(messageID); err != nil {
		return
	}

	// Track tool call streaming state
	streamedToolCalls := make(map[int]bool) // Track which tool calls have had their start written

	// Stream response
	var fullContent strings.Builder
	for chunk := range chunks {
		// Handle text content
		if chunk.Content != "" {
			fullContent.WriteString(chunk.Content)
			if err := sw.WriteText(chunk.Content); err != nil {
				return
			}
		}

		// Handle tool call deltas (streaming tool call arguments)
		for _, delta := range chunk.ToolCallDeltas {
			// Write tool call start if this is the first delta for this tool call
			if delta.ID != "" && delta.Name != "" && !streamedToolCalls[delta.Index] {
				streamedToolCalls[delta.Index] = true
				if err := sw.WriteToolCallStart(delta.ID, delta.Name); err != nil {
					return
				}
			}

			// Write argument delta if present
			if delta.ArgDelta != "" {
				if err := sw.WriteToolCallArgDelta(delta.ID, delta.ArgDelta); err != nil {
					return
				}
			}
		}

		// Handle completed tool calls - execute them and write results
		for _, tc := range chunk.ToolCalls {
			// Parse arguments to interface{} for proper JSON encoding
			var args interface{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = tc.Function.Arguments // Fallback to string
			}

			if err := sw.WriteToolCall(tc.ID, tc.Function.Name, args); err != nil {
				return
			}

			// Execute the tool and write result
			toolExecutor := h.chatService.GetToolExecutor()
			result, err := toolExecutor.ExecuteToolCall(r.Context(), userID, services.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
			if err != nil {
				logging.Error("failed to execute tool", err, "tool", tc.Function.Name)
				// Write error result
				if err := sw.WriteToolResult(tc.ID, map[string]interface{}{
					"success": false,
					"error":   "Failed to execute tool: " + err.Error(),
				}); err != nil {
					return
				}
				continue
			}

			// Write successful tool result
			if err := sw.WriteToolResult(tc.ID, result); err != nil {
				return
			}
		}

		if chunk.Done {
			// Determine finish reason type
			finishReason := streaming.FinishReasonStop
			switch chunk.FinishReason {
			case "stop":
				finishReason = streaming.FinishReasonStop
			case "length":
				finishReason = streaming.FinishReasonLength
			case "tool_calls":
				finishReason = streaming.FinishReasonToolCalls
			case "content_filter":
				finishReason = streaming.FinishReasonContentFilter
			case "error":
				finishReason = streaming.FinishReasonError
			default:
				if chunk.FinishReason != "" {
					finishReason = streaming.FinishReasonType(chunk.FinishReason)
				}
			}

			// Convert usage if available
			var usage *streaming.Usage
			if chunk.Usage != nil {
				usage = &streaming.Usage{
					PromptTokens:     chunk.Usage.PromptTokens,
					CompletionTokens: chunk.Usage.CompletionTokens,
					TotalTokens:      chunk.Usage.TotalTokens,
				}
			}

			// Write finish step (per LLM call)
			isContinued := finishReason == streaming.FinishReasonToolCalls
			if err := sw.WriteFinishStep(finishReason, usage, isContinued); err != nil {
				return
			}

			// If not continuing (no tool calls), write final finish message
			if !isContinued {
				if err := sw.WriteFinishMessage(finishReason, usage); err != nil {
					return
				}
			}
			break
		}
	}

	// Save the complete response to database
	if _, err := h.chatService.SaveStreamedResponse(r.Context(), sessionID, fullContent.String()); err != nil {
		logging.Error("failed to save streamed response", err, "sessionID", sessionID.String())
	}

	// Include user message ID in annotations
	if err := sw.WriteAnnotation(map[string]string{
		"userMessageId": userMsg.ID.String(),
		"messageId":     messageID,
	}); err != nil {
		logging.Warn("failed to write annotation", "error", err)
	}
}
