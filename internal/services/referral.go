package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/agpt-go/chatbot-api/internal/database"
	"github.com/agpt-go/chatbot-api/internal/logging"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ReferralService handles referral tracking and growth loop functionality
// Implements expert recommendations from Brian Balfour, Andrew Chen, Elena Verna
type ReferralService struct {
	queries   *database.Queries
	analytics *AnalyticsService
	baseURL   string // Base URL for generating share links
}

// NewReferralService creates a new referral service
func NewReferralService(queries *database.Queries, analytics *AnalyticsService, baseURL string) *ReferralService {
	return &ReferralService{
		queries:   queries,
		analytics: analytics,
		baseURL:   strings.TrimSuffix(baseURL, "/"),
	}
}

// ShareChannel represents the channel through which a referral was shared
type ShareChannel string

const (
	ShareChannelCopyLink ShareChannel = "copy_link"
	ShareChannelEmail    ShareChannel = "email"
	ShareChannelTwitter  ShareChannel = "twitter"
	ShareChannelLinkedIn ShareChannel = "linkedin"
)

// ShareSource represents where in the app the share was initiated
type ShareSource string

const (
	ShareSourceBusinessReport ShareSource = "business_report"
	ShareSourceDashboard      ShareSource = "dashboard"
	ShareSourceProfile        ShareSource = "profile"
)

// ReferralCodeResponse is returned when getting or creating a referral code
type ReferralCodeResponse struct {
	Code     string `json:"code"`
	ShareURL string `json:"share_url"`
}

// ReferralStatsResponse contains the user's referral statistics
type ReferralStatsResponse struct {
	Code              string  `json:"code"`
	ShareURL          string  `json:"share_url"`
	TotalShares       int32   `json:"total_shares"`
	TotalClicks       int32   `json:"total_clicks"`
	TotalSignups      int32   `json:"total_signups"`
	TotalActivations  int32   `json:"total_activations"`
	ClickToSignupRate float64 `json:"click_to_signup_rate"`
	ActivationRate    float64 `json:"activation_rate"`
	ViralityK         float64 `json:"virality_k"` // K-factor: invites × conversion
}

// ReferredUserResponse contains info about a referred user
type ReferredUserResponse struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	SignedUpAt  string    `json:"signed_up_at"`
	ActivatedAt *string   `json:"activated_at,omitempty"`
}

// ShareRecordResponse contains info about a share event
type ShareRecordResponse struct {
	Channel   string `json:"channel"`
	Source    string `json:"source"`
	Completed bool   `json:"completed"`
	CreatedAt string `json:"created_at"`
}

// GetOrCreateReferralCode gets the user's referral code or creates one if it doesn't exist
// Andrew Chen: Unique referral codes tied to user ID for attribution
func (s *ReferralService) GetOrCreateReferralCode(ctx context.Context, userID uuid.UUID) (*ReferralCodeResponse, error) {
	// Try to get existing code
	code, err := s.queries.GetReferralCodeByUserID(ctx, userID)
	if err == nil {
		return &ReferralCodeResponse{
			Code:     code.Code,
			ShareURL: s.buildShareURL(code.Code),
		}, nil
	}
	if err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to get referral code: %w", err)
	}

	// Generate new code
	newCode, err := s.generateUniqueCode(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate code: %w", err)
	}

	// Get user's business context for pre-population (Andrew Chen recommendation)
	var industry, businessSize *string
	bu, err := s.queries.GetBusinessUnderstanding(ctx, userID)
	if err == nil {
		industry = bu.Industry
		businessSize = bu.BusinessSize
	}

	// Create the code
	created, err := s.queries.CreateReferralCode(ctx, database.CreateReferralCodeParams{
		UserID:       userID,
		Code:         newCode,
		Industry:     industry,
		BusinessSize: businessSize,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create referral code: %w", err)
	}

	logging.Info("created referral code", "userID", userID.String(), "code", newCode)

	return &ReferralCodeResponse{
		Code:     created.Code,
		ShareURL: s.buildShareURL(created.Code),
	}, nil
}

// RecordShare records when a user shares their referral link
// Elena Verna: Track share_intent vs share_completed
func (s *ReferralService) RecordShare(ctx context.Context, userID uuid.UUID, channel ShareChannel, source ShareSource, completed bool) error {
	// Get the user's referral code
	code, err := s.queries.GetReferralCodeByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get referral code: %w", err)
	}

	// Record the share
	_, err = s.queries.CreateReferralShare(ctx, database.CreateReferralShareParams{
		ReferrerID:     userID,
		ReferralCodeID: code.ID,
		ShareChannel:   string(channel),
		ShareSource:    stringPtr(string(source)),
		Completed:      &completed,
	})
	if err != nil {
		return fmt.Errorf("failed to record share: %w", err)
	}

	// Increment share count
	if err := s.queries.IncrementReferralShares(ctx, code.ID); err != nil {
		logging.Error("failed to increment share count", err)
	}

	// Track in analytics
	if s.analytics != nil {
		s.analytics.TrackReferralShared(userID, string(channel), string(source), completed)
	}

	return nil
}

// RecordClick records when someone clicks a referral link
// Kieran Flanagan: Track the full referral journey
func (s *ReferralService) RecordClick(ctx context.Context, referralCode string, visitorID string, ipHash string, userAgent string, landingPage string, utmSource, utmMedium, utmCampaign string) (*database.ReferralClick, error) {
	// Get the referral code
	code, err := s.queries.GetReferralCodeByCode(ctx, referralCode)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("invalid referral code")
		}
		return nil, fmt.Errorf("failed to get referral code: %w", err)
	}

	// Record the click
	click, err := s.queries.CreateReferralClick(ctx, database.CreateReferralClickParams{
		ReferralCodeID: code.ID,
		VisitorID:      stringPtr(visitorID),
		IpHash:         stringPtr(ipHash),
		UserAgent:      stringPtr(userAgent),
		LandingPage:    stringPtr(landingPage),
		UtmSource:      stringPtr(utmSource),
		UtmMedium:      stringPtr(utmMedium),
		UtmCampaign:    stringPtr(utmCampaign),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to record click: %w", err)
	}

	// Increment click count
	if err := s.queries.IncrementReferralClicks(ctx, code.ID); err != nil {
		logging.Error("failed to increment click count", err)
	}

	// Track in analytics
	if s.analytics != nil {
		s.analytics.TrackReferralLinkClicked(code.UserID, referralCode)
	}

	return &click, nil
}

// ProcessReferralSignup handles when a new user signs up via a referral
// Andrew Chen: Full attribution with pre-populated context
func (s *ReferralService) ProcessReferralSignup(ctx context.Context, newUserID uuid.UUID, referralCode string, visitorID *string) error {
	// Get the referral code
	code, err := s.queries.GetReferralCodeByCode(ctx, referralCode)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("invalid referral code")
		}
		return fmt.Errorf("failed to get referral code: %w", err)
	}

	// Check if already referred (prevent duplicate referrals)
	_, err = s.queries.GetReferralSignupByReferee(ctx, newUserID)
	if err == nil {
		logging.Warn("user already has referral record", "userID", newUserID.String())
		return nil
	}
	if err != pgx.ErrNoRows {
		return fmt.Errorf("failed to check existing referral: %w", err)
	}

	// Try to find the click that led to this signup
	var clickID *uuid.UUID
	if visitorID != nil {
		click, err := s.queries.GetReferralClickByVisitor(ctx, visitorID)
		if err == nil {
			clickID = &click.ID
			// Mark click as converted
			if err := s.queries.UpdateReferralClickConverted(ctx, database.UpdateReferralClickConvertedParams{
				ID:              click.ID,
				ConvertedUserID: newUserID,
			}); err != nil {
				logging.Error("failed to update click conversion", err)
			}
		}
	}

	// Create referral signup record with inherited context (Andrew Chen recommendation)
	status := "signed_up"
	_, err = s.queries.CreateReferralSignup(ctx, database.CreateReferralSignupParams{
		RefereeID:             newUserID,
		ReferrerID:            code.UserID,
		ReferralCodeID:        code.ID,
		ClickID:               clickID,
		InheritedIndustry:     code.Industry,
		InheritedBusinessSize: code.BusinessSize,
		Status:                &status,
	})
	if err != nil {
		return fmt.Errorf("failed to create referral signup: %w", err)
	}

	// Increment signup count
	if err := s.queries.IncrementReferralSignups(ctx, code.ID); err != nil {
		logging.Error("failed to increment signup count", err)
	}

	// Track in analytics
	if s.analytics != nil {
		s.analytics.TrackReferralSignup(code.UserID, newUserID, referralCode)
	}

	logging.Info("processed referral signup",
		"referrer", code.UserID.String(),
		"referee", newUserID.String(),
		"code", referralCode)

	return nil
}

// MarkRefereeActivated marks a referred user as activated
// Dave McClure: Track activation milestone
func (s *ReferralService) MarkRefereeActivated(ctx context.Context, userID uuid.UUID) error {
	signup, err := s.queries.GetReferralSignupByReferee(ctx, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil // Not a referred user
		}
		return fmt.Errorf("failed to get referral signup: %w", err)
	}

	// Update activation status
	if err := s.queries.UpdateReferralSignupActivated(ctx, userID); err != nil {
		return fmt.Errorf("failed to update activation: %w", err)
	}

	// Increment activation count for the referrer
	if err := s.queries.IncrementReferralActivations(ctx, signup.ReferralCodeID); err != nil {
		logging.Error("failed to increment activation count", err)
	}

	// Track in analytics
	if s.analytics != nil {
		s.analytics.TrackReferralActivated(signup.ReferrerID, userID)
	}

	return nil
}

// MarkRefereeGeneratedReport marks when a referred user generates their first report
// Brian Balfour: Business report is the key value delivery moment
func (s *ReferralService) MarkRefereeGeneratedReport(ctx context.Context, userID uuid.UUID) error {
	_, err := s.queries.GetReferralSignupByReferee(ctx, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil // Not a referred user
		}
		return fmt.Errorf("failed to get referral signup: %w", err)
	}

	// Update first report status
	if err := s.queries.UpdateReferralSignupFirstReport(ctx, userID); err != nil {
		return fmt.Errorf("failed to update first report: %w", err)
	}

	return nil
}

// GetReferralStats returns the user's referral statistics
// Kieran Flanagan: Make referrals measurable
func (s *ReferralService) GetReferralStats(ctx context.Context, userID uuid.UUID) (*ReferralStatsResponse, error) {
	// Ensure user has a referral code
	codeResp, err := s.GetOrCreateReferralCode(ctx, userID)
	if err != nil {
		return nil, err
	}

	stats, err := s.queries.GetReferralStats(ctx, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return &ReferralStatsResponse{
				Code:     codeResp.Code,
				ShareURL: codeResp.ShareURL,
			}, nil
		}
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	// Calculate K-factor (virality coefficient)
	// K = invites per user × conversion rate
	var kFactor float64
	if stats.TotalShares > 0 && stats.TotalClicks > 0 {
		conversionRate := float64(stats.TotalSignups) / float64(stats.TotalClicks)
		kFactor = float64(stats.TotalShares) * conversionRate
	}

	return &ReferralStatsResponse{
		Code:              stats.Code,
		ShareURL:          s.buildShareURL(stats.Code),
		TotalShares:       stats.TotalShares,
		TotalClicks:       stats.TotalClicks,
		TotalSignups:      stats.TotalSignups,
		TotalActivations:  stats.TotalActivations,
		ClickToSignupRate: stats.ClickToSignupRate,
		ActivationRate:    stats.ActivationRate,
		ViralityK:         kFactor,
	}, nil
}

// GetReferredUsers returns list of users referred by this user
// Elena Verna: Referrer dashboard showing invite status
func (s *ReferralService) GetReferredUsers(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]ReferredUserResponse, int64, error) {
	signups, err := s.queries.GetReferralSignupsByReferrer(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get signups: %w", err)
	}

	count, err := s.queries.CountReferralSignupsByReferrer(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count signups: %w", err)
	}

	result := make([]ReferredUserResponse, len(signups))
	for i, s := range signups {
		resp := ReferredUserResponse{
			ID:         s.RefereeID,
			Email:      s.RefereeEmail,
			Name:       s.RefereeName,
			Status:     derefString(s.Status),
			SignedUpAt: formatTimestamptz(s.CreatedAt),
		}
		if s.ActivatedAt.Valid {
			activatedStr := formatTimestamptz(s.ActivatedAt)
			resp.ActivatedAt = &activatedStr
		}
		result[i] = resp
	}

	return result, count, nil
}

// GetShareHistory returns the user's share history
func (s *ReferralService) GetShareHistory(ctx context.Context, userID uuid.UUID, limit int32) ([]ShareRecordResponse, error) {
	shares, err := s.queries.GetReferralSharesByReferrer(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get shares: %w", err)
	}

	result := make([]ShareRecordResponse, len(shares))
	for i, share := range shares {
		result[i] = ShareRecordResponse{
			Channel:   share.ShareChannel,
			Source:    derefString(share.ShareSource),
			Completed: share.Completed != nil && *share.Completed,
			CreatedAt: formatTimestamptz(share.CreatedAt),
		}
	}

	return result, nil
}

// GetInheritedContext returns any context inherited from the referrer
// Andrew Chen: Pre-populate context from referrer to reduce activation friction
func (s *ReferralService) GetInheritedContext(ctx context.Context, userID uuid.UUID) (industry, businessSize *string, err error) {
	signup, err := s.queries.GetReferralSignupByReferee(ctx, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	return signup.InheritedIndustry, signup.InheritedBusinessSize, nil
}

// ValidateReferralCode checks if a referral code is valid
func (s *ReferralService) ValidateReferralCode(ctx context.Context, code string) (bool, uuid.UUID, error) {
	referralCode, err := s.queries.GetReferralCodeByCode(ctx, code)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, uuid.Nil, nil
		}
		return false, uuid.Nil, err
	}
	return true, referralCode.UserID, nil
}

// Helper functions

func (s *ReferralService) buildShareURL(code string) string {
	return fmt.Sprintf("%s/r/%s", s.baseURL, code)
}

func (s *ReferralService) generateUniqueCode(ctx context.Context) (string, error) {
	const maxAttempts = 10
	for i := 0; i < maxAttempts; i++ {
		code := generateCode(8)
		_, err := s.queries.GetReferralCodeByCode(ctx, code)
		if err == pgx.ErrNoRows {
			return code, nil
		}
		if err != nil {
			return "", err
		}
		// Code exists, try again
	}
	return "", fmt.Errorf("failed to generate unique code after %d attempts", maxAttempts)
}

// generateCode generates a random alphanumeric code
func generateCode(length int) string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // Removed confusing chars like 0/O, 1/I/L
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		// Fallback to less secure but working method
		return fmt.Sprintf("%X", sha256.Sum256([]byte(uuid.New().String())))[:length]
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// hashIP creates a privacy-preserving hash of an IP address
func HashIP(ip string) string {
	hash := sha256.Sum256([]byte(ip + "referral-salt"))
	return hex.EncodeToString(hash[:8])
}

func formatTimestamptz(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.Format(time.RFC3339)
}
