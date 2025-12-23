package services

import (
	"time"

	"github.com/agpt-go/chatbot-api/internal/config"
	"github.com/agpt-go/chatbot-api/internal/logging"
	"github.com/google/uuid"
	"github.com/posthog/posthog-go"
)

// AnalyticsService handles all PostHog analytics tracking
type AnalyticsService struct {
	client  posthog.Client
	enabled bool
}

// Event names for AARRR pirate metrics
const (
	// Acquisition events
	EventUserSignedUp = "user_signed_up"
	EventUserLoggedIn = "user_logged_in"

	// Activation events
	EventSessionCreated         = "session_created"
	EventMessageSent            = "message_sent"
	EventBusinessContextAdded   = "business_context_added"
	EventBusinessReportRequested = "business_report_requested"

	// Retention tracking (properties on events)
	PropertyIsReturningUser   = "is_returning_user"
	PropertyDaysSinceSignup   = "days_since_signup"
	PropertyDaysSinceLastSeen = "days_since_last_seen"
	PropertySessionCount      = "session_count"
	PropertyMessageCount      = "message_count"
)

// NewAnalyticsService creates a new analytics service with PostHog client
func NewAnalyticsService(cfg *config.AnalyticsConfig) *AnalyticsService {
	if !cfg.Enabled || cfg.PostHogAPIKey == "" {
		logging.Info("analytics disabled or no API key provided")
		return &AnalyticsService{enabled: false}
	}

	client, err := posthog.NewWithConfig(
		cfg.PostHogAPIKey,
		posthog.Config{
			Endpoint: cfg.PostHogHost,
		},
	)
	if err != nil {
		logging.Error("failed to create PostHog client", err)
		return &AnalyticsService{enabled: false}
	}

	logging.Info("analytics service initialized", "host", cfg.PostHogHost)
	return &AnalyticsService{
		client:  client,
		enabled: true,
	}
}

// Close flushes and closes the PostHog client
func (s *AnalyticsService) Close() {
	if s.client != nil {
		if err := s.client.Close(); err != nil {
			logging.Error("failed to close PostHog client", err)
		}
	}
}

// Identify sets user properties in PostHog
func (s *AnalyticsService) Identify(userID uuid.UUID, properties map[string]interface{}) {
	if !s.enabled {
		return
	}

	props := posthog.NewProperties()
	for k, v := range properties {
		props.Set(k, v)
	}

	err := s.client.Enqueue(posthog.Identify{
		DistinctId: userID.String(),
		Properties: props,
	})
	if err != nil {
		logging.Error("failed to identify user", err, "userID", userID.String())
	}
}

// Track sends an event to PostHog
func (s *AnalyticsService) Track(userID uuid.UUID, event string, properties map[string]interface{}) {
	if !s.enabled {
		return
	}

	props := posthog.NewProperties()
	for k, v := range properties {
		props.Set(k, v)
	}

	err := s.client.Enqueue(posthog.Capture{
		DistinctId: userID.String(),
		Event:      event,
		Properties: props,
		Timestamp:  time.Now(),
	})
	if err != nil {
		logging.Error("failed to track event", err, "event", event, "userID", userID.String())
	}
}

// TrackUserSignedUp tracks a new user registration
func (s *AnalyticsService) TrackUserSignedUp(userID uuid.UUID, email, name, signupMethod string) {
	// Identify the user with initial properties
	s.Identify(userID, map[string]interface{}{
		"email":         email,
		"name":          name,
		"signup_method": signupMethod,
		"signup_date":   time.Now().Format(time.RFC3339),
	})

	// Track the signup event
	s.Track(userID, EventUserSignedUp, map[string]interface{}{
		"signup_method": signupMethod,
	})
}

// TrackUserLoggedIn tracks a user login
func (s *AnalyticsService) TrackUserLoggedIn(userID uuid.UUID, loginMethod string) {
	s.Track(userID, EventUserLoggedIn, map[string]interface{}{
		"login_method": loginMethod,
	})
}

// TrackSessionCreated tracks when a user creates a new chat session
func (s *AnalyticsService) TrackSessionCreated(userID uuid.UUID, sessionID uuid.UUID, isReturningUser bool, sessionCount int) {
	s.Track(userID, EventSessionCreated, map[string]interface{}{
		"session_id":        sessionID.String(),
		PropertyIsReturningUser: isReturningUser,
		PropertySessionCount:    sessionCount,
	})
}

// TrackMessageSent tracks when a user sends a message to the AI
func (s *AnalyticsService) TrackMessageSent(userID uuid.UUID, sessionID uuid.UUID, messageNumber int, isFirstMessage bool) {
	s.Track(userID, EventMessageSent, map[string]interface{}{
		"session_id":       sessionID.String(),
		"message_number":   messageNumber,
		"is_first_message": isFirstMessage,
	})
}

// TrackBusinessContextAdded tracks when user provides business context
func (s *AnalyticsService) TrackBusinessContextAdded(userID uuid.UUID, fieldsUpdated []string, completenessPercentage float64) {
	s.Track(userID, EventBusinessContextAdded, map[string]interface{}{
		"fields_updated":          fieldsUpdated,
		"fields_count":            len(fieldsUpdated),
		"completeness_percentage": completenessPercentage,
	})

	// Update user property for activation tracking
	s.Identify(userID, map[string]interface{}{
		"has_added_business_context": true,
		"business_context_completeness": completenessPercentage,
	})
}

// TrackBusinessReportRequested tracks when user requests a business report
func (s *AnalyticsService) TrackBusinessReportRequested(userID uuid.UUID, reportType, status string, dataCompleteness float64) {
	s.Track(userID, EventBusinessReportRequested, map[string]interface{}{
		"report_type":       reportType,
		"status":            status,
		"data_completeness": dataCompleteness,
	})

	// Update user property - this is the key activation metric
	if status == "ready_for_report" {
		s.Identify(userID, map[string]interface{}{
			"has_generated_report": true,
			"first_report_date":    time.Now().Format(time.RFC3339),
		})
	}
}

// CalculateCompletenessPercentage calculates how complete the business understanding is
func CalculateCompletenessPercentage(status UnderstandingStatus) float64 {
	total := 15 // Total number of trackable fields
	filled := 0

	if status.UserName {
		filled++
	}
	if status.JobTitle {
		filled++
	}
	if status.BusinessName {
		filled++
	}
	if status.Industry {
		filled++
	}
	if status.BusinessSize {
		filled++
	}
	if status.UserRole {
		filled++
	}
	if status.KeyWorkflowsCount > 0 {
		filled++
	}
	if status.DailyActivitiesCount > 0 {
		filled++
	}
	if status.PainPointsCount > 0 {
		filled++
	}
	if status.BottlenecksCount > 0 {
		filled++
	}
	if status.ManualTasksCount > 0 {
		filled++
	}
	if status.AutomationGoalsCount > 0 {
		filled++
	}
	if status.CurrentSoftwareCount > 0 {
		filled++
	}
	if status.ExistingAutomationCount > 0 {
		filled++
	}
	if status.AdditionalNotes {
		filled++
	}

	return float64(filled) / float64(total) * 100
}
