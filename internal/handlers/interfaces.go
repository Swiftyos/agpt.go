package handlers

import (
	"context"

	"github.com/agpt-go/chatbot-api/internal/database"
	"github.com/agpt-go/chatbot-api/internal/services"
	"github.com/google/uuid"
)

// ChatServicer defines the interface for chat service operations
// This interface is defined at the consumer site for testability
type ChatServicer interface {
	CreateSession(ctx context.Context, userID uuid.UUID, input services.CreateSessionInput) (*database.ChatSession, error)
	GetSession(ctx context.Context, sessionID, userID uuid.UUID) (*database.ChatSession, error)
	ListSessions(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]database.ChatSession, error)
	UpdateSession(ctx context.Context, sessionID uuid.UUID, title, systemPrompt *string) (*database.ChatSession, error)
	DeleteSession(ctx context.Context, sessionID, userID uuid.UUID) error
	GetMessages(ctx context.Context, sessionID uuid.UUID, limit int) ([]database.ChatMessage, error)
	SendMessage(ctx context.Context, sessionID, userID uuid.UUID, content string) (*database.ChatMessage, *database.ChatMessage, error)
	SendMessageStream(ctx context.Context, sessionID, userID uuid.UUID, content string) (*database.ChatMessage, <-chan services.StreamChunk, error)
	SaveStreamedResponse(ctx context.Context, sessionID uuid.UUID, content string) (*database.ChatMessage, error)
	GetToolExecutor() *services.ToolExecutor
	GetAvailableTools() []services.ToolDefinition
}

// AuthServicer defines the interface for auth service operations
// This interface is defined at the consumer site for testability
type AuthServicer interface {
	Register(ctx context.Context, email, password, name string) (*database.User, *services.TokenPair, error)
	Login(ctx context.Context, email, password string) (*database.User, *services.TokenPair, error)
	RefreshTokens(ctx context.Context, refreshToken string) (*services.TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
	ValidateAccessToken(tokenString string) (*services.Claims, error)
	GenerateOAuthState(ctx context.Context) (string, error)
	ValidateOAuthState(ctx context.Context, state string) error
	GetGoogleAuthURL(state string) string
	HandleGoogleCallback(ctx context.Context, code string) (*database.User, *services.TokenPair, error)
}

// Validator defines the interface for request validation
type Validator interface {
	Struct(s interface{}) error
}

// AnalyticsServicer defines the interface for analytics operations
type AnalyticsServicer interface {
	TrackUserSignedUp(userID uuid.UUID, email, name, signupMethod string)
	TrackUserLoggedIn(userID uuid.UUID, loginMethod string)
	TrackSessionCreated(userID uuid.UUID, sessionID uuid.UUID, isReturningUser bool, sessionCount int)
	TrackMessageSent(userID uuid.UUID, sessionID uuid.UUID, messageNumber int, isFirstMessage bool)
	// Charity Majors: Track errors for observability
	TrackError(userID uuid.UUID, errorType, errorMessage, context string)
}

// ReferralServicer defines the interface for referral service operations
type ReferralServicer interface {
	ProcessReferralSignup(ctx context.Context, newUserID uuid.UUID, referralCode string, visitorID *string) error
	HashIP(ip string) string
	HashVisitorID(visitorID string) string
}
