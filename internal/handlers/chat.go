package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/agpt-go/chatbot-api/internal/middleware"
	"github.com/agpt-go/chatbot-api/internal/services"
	"github.com/agpt-go/chatbot-api/internal/streaming"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type ChatHandler struct {
	chatService *services.ChatService
	validate    *validator.Validate
}

func NewChatHandler(chatService *services.ChatService) *ChatHandler {
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

	writeJSON(w, http.StatusCreated, SessionResponse{
		ID:           session.ID.String(),
		Title:        session.Title,
		Model:        session.Model,
		SystemPrompt: session.SystemPrompt,
		CreatedAt:    session.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    session.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
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
		response[i] = SessionResponse{
			ID:           s.ID.String(),
			Title:        s.Title,
			Model:        s.Model,
			SystemPrompt: s.SystemPrompt,
			CreatedAt:    s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:    s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
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

	writeJSON(w, http.StatusOK, SessionResponse{
		ID:           session.ID.String(),
		Title:        session.Title,
		Model:        session.Model,
		SystemPrompt: session.SystemPrompt,
		CreatedAt:    session.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    session.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
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

	writeJSON(w, http.StatusOK, SessionResponse{
		ID:           session.ID.String(),
		Title:        session.Title,
		Model:        session.Model,
		SystemPrompt: session.SystemPrompt,
		CreatedAt:    session.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    session.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
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

// GetMessages returns all messages in a session
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

	// Verify ownership
	_, err = h.chatService.GetSession(r.Context(), sessionID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Session not found")
		return
	}

	messages, err := h.chatService.GetMessages(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get messages")
		return
	}

	response := make([]MessageResponse, len(messages))
	for i, m := range messages {
		response[i] = MessageResponse{
			ID:        m.ID.String(),
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
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

	response := ChatResponse{
		UserMessage: &MessageResponse{
			ID:        userMsg.ID.String(),
			Role:      userMsg.Role,
			Content:   userMsg.Content,
			CreatedAt: userMsg.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	}

	if assistantMsg != nil {
		response.AssistantMessage = &MessageResponse{
			ID:        assistantMsg.ID.String(),
			Role:      assistantMsg.Role,
			Content:   assistantMsg.Content,
			CreatedAt: assistantMsg.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	writeJSON(w, http.StatusOK, response)
}

// SendMessageStream sends a message and streams the response
// Implements AI SDK Data Stream Protocol
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

	// Stream response
	var fullContent strings.Builder
	for chunk := range chunks {
		if chunk.Content != "" {
			fullContent.WriteString(chunk.Content)
			if err := sw.WriteText(chunk.Content); err != nil {
				return
			}
		}

		if chunk.Done {
			// Write finish reason
			if err := sw.WriteFinish(chunk.FinishReason, nil); err != nil {
				return
			}
			break
		}
	}

	// Save the complete response to database
	_, _ = h.chatService.SaveStreamedResponse(r.Context(), sessionID, fullContent.String())

	// Include user message ID in annotations
	_ = sw.WriteAnnotation(map[string]string{
		"userMessageId": userMsg.ID.String(),
		"messageId":     messageID,
	})
}
