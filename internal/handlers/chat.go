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
	analytics   AnalyticsServicer
	validate    Validator
}

func NewChatHandler(chatService ChatServicer, analytics AnalyticsServicer) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		analytics:   analytics,
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

// CreateSession godoc
// @Summary Create a new chat session
// @Description Create a new chat session for the authenticated user
// @Tags Sessions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateSessionRequest false "Session options"
// @Success 201 {object} SessionResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /sessions [post]
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

	// Track session created event
	if h.analytics != nil {
		// Get session count for this user to determine if returning
		sessions, _ := h.chatService.ListSessions(r.Context(), userID, 1, 0)
		isReturning := len(sessions) > 1
		sessionCount := len(sessions)
		h.analytics.TrackSessionCreated(userID, session.ID, isReturning, sessionCount)
	}

	writeJSON(w, http.StatusCreated, sessionToResponse(session))
}

// ListSessions godoc
// @Summary List chat sessions
// @Description List all chat sessions for the authenticated user
// @Tags Sessions
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Number of sessions (default: 20, max: 100)"
// @Param offset query int false "Offset for pagination (default: 0)"
// @Success 200 {array} SessionResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /sessions [get]
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

// GetSession godoc
// @Summary Get a chat session
// @Description Get details of a specific chat session
// @Tags Sessions
// @Produce json
// @Security BearerAuth
// @Param sessionID path string true "Session UUID"
// @Success 200 {object} SessionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /sessions/{sessionID} [get]
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

// UpdateSession godoc
// @Summary Update a chat session
// @Description Update the title or system prompt of a chat session
// @Tags Sessions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param sessionID path string true "Session UUID"
// @Param request body UpdateSessionRequest true "Update fields"
// @Success 200 {object} SessionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /sessions/{sessionID} [patch]
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

// DeleteSession godoc
// @Summary Delete a chat session
// @Description Delete a chat session and all its messages
// @Tags Sessions
// @Security BearerAuth
// @Param sessionID path string true "Session UUID"
// @Success 204 "Session deleted"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /sessions/{sessionID} [delete]
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

// GetMessages godoc
// @Summary Get messages in a session
// @Description Get all messages in a chat session
// @Tags Messages
// @Produce json
// @Security BearerAuth
// @Param sessionID path string true "Session UUID"
// @Param limit query int false "Number of messages (default: 100, max: 500)"
// @Success 200 {array} MessageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /sessions/{sessionID}/messages [get]
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

// SendMessage godoc
// @Summary Send a message (non-streaming)
// @Description Send a message to the chat session and get a response
// @Tags Messages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param sessionID path string true "Session UUID"
// @Param request body SendMessageRequest true "Message content"
// @Success 200 {object} ChatResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /sessions/{sessionID}/messages [post]
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

	// Track message sent event
	if h.analytics != nil {
		messages, _ := h.chatService.GetMessages(r.Context(), sessionID, 100)
		messageCount := len(messages)
		isFirstMessage := messageCount <= 2 // user + assistant message
		h.analytics.TrackMessageSent(userID, sessionID, messageCount, isFirstMessage)
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

// SendMessageStream godoc
// @Summary Send a message (streaming)
// @Description Send a message and stream the response using Vercel AI SDK Data Stream Protocol
// @Tags Messages
// @Accept json
// @Produce text/plain
// @Security BearerAuth
// @Param sessionID path string true "Session UUID"
// @Param request body SendMessageRequest true "Message content"
// @Success 200 {string} string "Stream of parts in format 'type:json\\n'"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /sessions/{sessionID}/messages/stream [post]
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

	// Track message sent event
	if h.analytics != nil {
		messages, _ := h.chatService.GetMessages(r.Context(), sessionID, 100)
		messageCount := len(messages)
		isFirstMessage := messageCount <= 1 // only user message at this point
		h.analytics.TrackMessageSent(userID, sessionID, messageCount, isFirstMessage)
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
