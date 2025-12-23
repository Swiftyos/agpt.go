package services

import (
	"fmt"
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
	EventSessionCreated          = "session_created"
	EventMessageSent             = "message_sent"
	EventBusinessContextAdded    = "business_context_added"
	EventBusinessReportRequested = "business_report_requested"

	// Retention tracking (properties on events)
	PropertyIsReturningUser   = "is_returning_user"
	PropertyDaysSinceSignup   = "days_since_signup"
	PropertyDaysSinceLastSeen = "days_since_last_seen"
	PropertySessionCount      = "session_count"
	PropertyMessageCount      = "message_count"

	// Referral events - Growth loop tracking (Brian Balfour, Andrew Chen, Kieran Flanagan)
	EventReferralShared    = "referral_shared"      // User shared their referral link
	EventReferralClicked   = "referral_link_clicked" // Someone clicked a referral link
	EventReferralSignup    = "referral_signup"       // New user signed up via referral
	EventReferralActivated = "referral_activated"    // Referred user reached activation

	// Error tracking
	EventError = "error_occurred"
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

// IdentifyOnce sets user properties that should only be set once (immutable)
// Uses PostHog's $set_once to prevent overwriting existing values
func (s *AnalyticsService) IdentifyOnce(userID uuid.UUID, properties map[string]interface{}) {
	if !s.enabled {
		return
	}

	props := posthog.NewProperties()
	props.Set("$set_once", properties)

	err := s.client.Enqueue(posthog.Identify{
		DistinctId: userID.String(),
		Properties: props,
	})
	if err != nil {
		logging.Error("failed to identify user (set_once)", err, "userID", userID.String())
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

// getSignupCohort returns the cohort identifier (YYYY-WW format for week-based cohorts)
func getSignupCohort(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}

// TrackUserSignedUp tracks a new user registration
func (s *AnalyticsService) TrackUserSignedUp(userID uuid.UUID, email, name, signupMethod string) {
	now := time.Now()
	cohort := getSignupCohort(now)

	// Set mutable properties (can be updated)
	s.Identify(userID, map[string]interface{}{
		"email": email,
		"name":  name,
	})

	// Set immutable properties with $set_once (PostHog recommendation)
	// These won't be overwritten if user signs up again via different method
	s.IdentifyOnce(userID, map[string]interface{}{
		"signup_method":    signupMethod,
		"signup_date":      now.Format(time.RFC3339),
		"signup_timestamp": now.Unix(), // Unix timestamp for time calculations
		"signup_cohort":    cohort,     // Brian Balfour: essential for retention analysis
	})

	// Track the signup event
	s.Track(userID, EventUserSignedUp, map[string]interface{}{
		"signup_method": signupMethod,
		"signup_cohort": cohort,
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
	now := time.Now()

	s.Track(userID, EventSessionCreated, map[string]interface{}{
		"session_id":            sessionID.String(),
		PropertyIsReturningUser: isReturningUser,
		PropertySessionCount:    sessionCount,
	})

	// Track first session with $set_once
	if sessionCount == 1 {
		s.IdentifyOnce(userID, map[string]interface{}{
			"first_session_date":      now.Format(time.RFC3339),
			"first_session_timestamp": now.Unix(),
		})
	}

	// Update last session for retention tracking
	s.Identify(userID, map[string]interface{}{
		"last_session_date":  now.Format(time.RFC3339),
		"total_session_count": sessionCount,
	})
}

// TrackMessageSent tracks when a user sends a message to the AI
func (s *AnalyticsService) TrackMessageSent(userID uuid.UUID, sessionID uuid.UUID, messageNumber int, isFirstMessage bool) {
	now := time.Now()

	s.Track(userID, EventMessageSent, map[string]interface{}{
		"session_id":       sessionID.String(),
		"message_number":   messageNumber,
		"is_first_message": isFirstMessage,
	})

	// Track first message milestone with $set_once (Dave McClure: time-to-activation)
	if isFirstMessage {
		s.IdentifyOnce(userID, map[string]interface{}{
			"first_message_date":      now.Format(time.RFC3339),
			"first_message_timestamp": now.Unix(),
		})

		// Also set that user has sent a message (mutable for counting)
		s.Identify(userID, map[string]interface{}{
			"has_sent_message": true,
		})
	}

	// Update total message count for feature depth tracking
	s.Identify(userID, map[string]interface{}{
		"last_message_date": now.Format(time.RFC3339),
	})
}

// TrackBusinessContextAdded tracks when user provides business context
func (s *AnalyticsService) TrackBusinessContextAdded(userID uuid.UUID, fieldsUpdated []string, completenessPercentage float64) {
	now := time.Now()

	s.Track(userID, EventBusinessContextAdded, map[string]interface{}{
		"fields_updated":          fieldsUpdated,
		"fields_count":            len(fieldsUpdated),
		"completeness_percentage": completenessPercentage,
	})

	// Update user property for activation tracking (mutable)
	s.Identify(userID, map[string]interface{}{
		"has_added_business_context":    true,
		"business_context_completeness": completenessPercentage,
		"last_context_update":           now.Format(time.RFC3339),
	})

	// Track first context addition with $set_once
	s.IdentifyOnce(userID, map[string]interface{}{
		"first_context_date":      now.Format(time.RFC3339),
		"first_context_timestamp": now.Unix(),
	})
}

// TrackBusinessReportRequested tracks when user requests a business report
func (s *AnalyticsService) TrackBusinessReportRequested(userID uuid.UUID, reportType, status string, dataCompleteness float64) {
	now := time.Now()

	s.Track(userID, EventBusinessReportRequested, map[string]interface{}{
		"report_type":       reportType,
		"status":            status,
		"data_completeness": dataCompleteness,
	})

	// Update user property - this is the key activation metric
	if status == "ready_for_report" {
		// Mutable property (can track multiple reports)
		s.Identify(userID, map[string]interface{}{
			"has_generated_report": true,
			"last_report_date":     now.Format(time.RFC3339),
			"last_report_type":     reportType,
		})

		// Immutable properties with $set_once (Dave McClure: time-to-report)
		s.IdentifyOnce(userID, map[string]interface{}{
			"first_report_date":      now.Format(time.RFC3339),
			"first_report_timestamp": now.Unix(),
			"first_report_type":      reportType,
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

// TrackError tracks error events for debugging and monitoring
func (s *AnalyticsService) TrackError(userID uuid.UUID, errorType, errorMessage, context string) {
	s.Track(userID, EventError, map[string]interface{}{
		"error_type":    errorType,
		"error_message": errorMessage,
		"context":       context,
	})
}

// IdentifyCompany identifies a company/organization for B2B group analytics
// This enables tracking metrics at the company level, not just user level
func (s *AnalyticsService) IdentifyCompany(userID uuid.UUID, companyName, industry, size string) {
	if !s.enabled || companyName == "" {
		return
	}

	// Associate user with company group
	err := s.client.Enqueue(posthog.GroupIdentify{
		Type: "company",
		Key:  companyName,
		Properties: posthog.NewProperties().
			Set("name", companyName).
			Set("industry", industry).
			Set("size", size),
	})
	if err != nil {
		logging.Error("failed to identify company", err, "company", companyName)
	}

	// Link user to this company
	s.Identify(userID, map[string]interface{}{
		"$groups": map[string]string{
			"company": companyName,
		},
	})
}

// Referral tracking methods - Growth loop analytics
// Expert guidance: Brian Balfour (Reforge), Andrew Chen (a16z), Kieran Flanagan (Zapier)

// TrackReferralShared tracks when a user shares their referral link
// Elena Verna: Track share_intent vs share_completed
func (s *AnalyticsService) TrackReferralShared(userID uuid.UUID, channel, source string, completed bool) {
	now := time.Now()

	s.Track(userID, EventReferralShared, map[string]interface{}{
		"share_channel":   channel,
		"share_source":    source,
		"share_completed": completed,
	})

	// Track first share milestone with $set_once
	s.IdentifyOnce(userID, map[string]interface{}{
		"first_share_date":      now.Format(time.RFC3339),
		"first_share_timestamp": now.Unix(),
		"first_share_channel":   channel,
	})

	// Update mutable share stats
	s.Identify(userID, map[string]interface{}{
		"has_shared_referral": true,
		"last_share_date":     now.Format(time.RFC3339),
		"last_share_channel":  channel,
	})
}

// TrackReferralLinkClicked tracks when someone clicks a referral link
// Kieran Flanagan: Track full referral journey
func (s *AnalyticsService) TrackReferralLinkClicked(referrerID uuid.UUID, referralCode string) {
	s.Track(referrerID, EventReferralClicked, map[string]interface{}{
		"referral_code": referralCode,
	})
}

// TrackReferralSignup tracks when a new user signs up via referral
// Andrew Chen: Full attribution for referral signups
func (s *AnalyticsService) TrackReferralSignup(referrerID, refereeID uuid.UUID, referralCode string) {
	now := time.Now()

	// Track event for the referrer
	s.Track(referrerID, EventReferralSignup, map[string]interface{}{
		"referee_id":    refereeID.String(),
		"referral_code": referralCode,
	})

	// Update referrer properties
	s.Identify(referrerID, map[string]interface{}{
		"has_successful_referral": true,
		"last_referral_date":      now.Format(time.RFC3339),
	})

	// Track first successful referral with $set_once
	s.IdentifyOnce(referrerID, map[string]interface{}{
		"first_referral_date":      now.Format(time.RFC3339),
		"first_referral_timestamp": now.Unix(),
	})

	// Set properties on the new user (referee)
	s.IdentifyOnce(refereeID, map[string]interface{}{
		"referred_by":            referrerID.String(),
		"referral_code_used":     referralCode,
		"referral_signup_date":   now.Format(time.RFC3339),
		"acquisition_channel":    "referral",
	})
}

// TrackReferralActivated tracks when a referred user reaches activation milestone
// Dave McClure: Activation is the key metric for referral success
func (s *AnalyticsService) TrackReferralActivated(referrerID, refereeID uuid.UUID) {
	now := time.Now()

	// Track event for the referrer
	s.Track(referrerID, EventReferralActivated, map[string]interface{}{
		"referee_id": refereeID.String(),
	})

	// Update referrer properties
	s.Identify(referrerID, map[string]interface{}{
		"has_activated_referral":    true,
		"last_referral_activation":  now.Format(time.RFC3339),
	})

	// Track first activated referral with $set_once
	s.IdentifyOnce(referrerID, map[string]interface{}{
		"first_referral_activation_date":      now.Format(time.RFC3339),
		"first_referral_activation_timestamp": now.Unix(),
	})

	// Update referee properties
	s.Identify(refereeID, map[string]interface{}{
		"referral_activated":    true,
		"referral_activated_at": now.Format(time.RFC3339),
	})
}
