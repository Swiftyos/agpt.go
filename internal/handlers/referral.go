package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/agpt-go/chatbot-api/internal/middleware"
	"github.com/agpt-go/chatbot-api/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ReferralHandler handles referral-related HTTP requests
type ReferralHandler struct {
	referralService *services.ReferralService
}

// NewReferralHandler creates a new referral handler
func NewReferralHandler(referralService *services.ReferralService) *ReferralHandler {
	return &ReferralHandler{
		referralService: referralService,
	}
}

// Request/Response types

// RecordShareRequest is the request body for recording a share
type RecordShareRequest struct {
	Channel   string `json:"channel" validate:"required,oneof=copy_link email twitter linkedin"`
	Source    string `json:"source" validate:"required,oneof=business_report dashboard profile"`
	Completed bool   `json:"completed"`
}

// RecordClickRequest is the request body for recording a click
type RecordClickRequest struct {
	ReferralCode string `json:"referral_code" validate:"required"`
	VisitorID    string `json:"visitor_id"`
	UserAgent    string `json:"user_agent"`
	LandingPage  string `json:"landing_page"`
	UtmSource    string `json:"utm_source"`
	UtmMedium    string `json:"utm_medium"`
	UtmCampaign  string `json:"utm_campaign"`
}

// ReferralCodeResponse is returned when getting the referral code
type ReferralCodeResponse struct {
	Code     string `json:"code"`
	ShareURL string `json:"share_url"`
}

// ReferralStatsResponse contains referral statistics
type ReferralStatsResponse struct {
	Code              string  `json:"code"`
	ShareURL          string  `json:"share_url"`
	TotalShares       int32   `json:"total_shares"`
	TotalClicks       int32   `json:"total_clicks"`
	TotalSignups      int32   `json:"total_signups"`
	TotalActivations  int32   `json:"total_activations"`
	ClickToSignupRate float64 `json:"click_to_signup_rate"`
	ActivationRate    float64 `json:"activation_rate"`
	ViralityK         float64 `json:"virality_k"`
}

// ReferredUsersResponse contains the list of referred users
type ReferredUsersResponse struct {
	Users      []services.ReferredUserResponse `json:"users"`
	TotalCount int64                           `json:"total_count"`
	Limit      int                             `json:"limit"`
	Offset     int                             `json:"offset"`
}

// ValidateCodeResponse indicates if a referral code is valid
type ValidateCodeResponse struct {
	Valid      bool   `json:"valid"`
	ReferrerID string `json:"referrer_id,omitempty"`
}

// GetReferralCode godoc
// @Summary Get or create referral code
// @Description Get the user's unique referral code and share URL. Creates one if it doesn't exist.
// @Tags Referrals
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ReferralCodeResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /referral/code [get]
func (h *ReferralHandler) GetReferralCode(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	code, err := h.referralService.GetOrCreateReferralCode(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get referral code")
		return
	}

	writeJSON(w, http.StatusOK, ReferralCodeResponse{
		Code:     code.Code,
		ShareURL: code.ShareURL,
	})
}

// RecordShare godoc
// @Summary Record a share event
// @Description Record when the user shares their referral link through a specific channel
// @Tags Referrals
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body RecordShareRequest true "Share details"
// @Success 200 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /referral/share [post]
func (h *ReferralHandler) RecordShare(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req RecordShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate channel and source
	validChannels := map[string]bool{"copy_link": true, "email": true, "twitter": true, "linkedin": true}
	validSources := map[string]bool{"business_report": true, "dashboard": true, "profile": true}

	if !validChannels[req.Channel] {
		writeError(w, http.StatusBadRequest, "Invalid share channel")
		return
	}
	if !validSources[req.Source] {
		writeError(w, http.StatusBadRequest, "Invalid share source")
		return
	}

	err := h.referralService.RecordShare(
		r.Context(),
		userID,
		services.ShareChannel(req.Channel),
		services.ShareSource(req.Source),
		req.Completed,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to record share")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "recorded"})
}

// GetReferralStats godoc
// @Summary Get referral statistics
// @Description Get the user's referral statistics including K-factor and conversion rates
// @Tags Referrals
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ReferralStatsResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /referral/stats [get]
func (h *ReferralHandler) GetReferralStats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	stats, err := h.referralService.GetReferralStats(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get referral stats")
		return
	}

	writeJSON(w, http.StatusOK, ReferralStatsResponse{
		Code:              stats.Code,
		ShareURL:          stats.ShareURL,
		TotalShares:       stats.TotalShares,
		TotalClicks:       stats.TotalClicks,
		TotalSignups:      stats.TotalSignups,
		TotalActivations:  stats.TotalActivations,
		ClickToSignupRate: stats.ClickToSignupRate,
		ActivationRate:    stats.ActivationRate,
		ViralityK:         stats.ViralityK,
	})
}

// GetReferredUsers godoc
// @Summary Get referred users
// @Description Get the list of users who signed up using this user's referral code
// @Tags Referrals
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Number of users (default: 20, max: 100)"
// @Param offset query int false "Offset for pagination (default: 0)"
// @Success 200 {object} ReferredUsersResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /referral/referred [get]
func (h *ReferralHandler) GetReferredUsers(w http.ResponseWriter, r *http.Request) {
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

	users, totalCount, err := h.referralService.GetReferredUsers(r.Context(), userID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get referred users")
		return
	}

	writeJSON(w, http.StatusOK, ReferredUsersResponse{
		Users:      users,
		TotalCount: totalCount,
		Limit:      int(limit),
		Offset:     int(offset),
	})
}

// GetShareHistory godoc
// @Summary Get share history
// @Description Get the user's history of share events
// @Tags Referrals
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Number of records (default: 50, max: 100)"
// @Success 200 {array} services.ShareRecordResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /referral/shares [get]
func (h *ReferralHandler) GetShareHistory(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	limit := int32(50)
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = int32(parsed)
		}
	}

	shares, err := h.referralService.GetShareHistory(r.Context(), userID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get share history")
		return
	}

	writeJSON(w, http.StatusOK, shares)
}

// RecordClick godoc
// @Summary Record a referral link click
// @Description Record when someone clicks a referral link (public endpoint, no auth required)
// @Tags Referrals
// @Accept json
// @Produce json
// @Param request body RecordClickRequest true "Click details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /referral/click [post]
func (h *ReferralHandler) RecordClick(w http.ResponseWriter, r *http.Request) {
	var req RecordClickRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ReferralCode == "" {
		writeError(w, http.StatusBadRequest, "Referral code is required")
		return
	}

	// Hash the IP for privacy (Troy Hunt: use configurable salt)
	ipHash := ""
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		ipHash = h.referralService.HashIP(ip)
	} else if ip := r.RemoteAddr; ip != "" {
		ipHash = h.referralService.HashIP(ip)
	}

	click, err := h.referralService.RecordClick(
		r.Context(),
		req.ReferralCode,
		req.VisitorID,
		ipHash,
		req.UserAgent,
		req.LandingPage,
		req.UtmSource,
		req.UtmMedium,
		req.UtmCampaign,
	)
	if err != nil {
		if err.Error() == "invalid referral code" {
			writeError(w, http.StatusBadRequest, "Invalid referral code")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to record click")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "recorded",
		"click_id": click.ID.String(),
	})
}

// ValidateCode godoc
// @Summary Validate a referral code
// @Description Check if a referral code is valid (public endpoint, no auth required)
// @Tags Referrals
// @Produce json
// @Param code path string true "Referral code"
// @Success 200 {object} ValidateCodeResponse
// @Failure 400 {object} ErrorResponse
// @Router /referral/validate/{code} [get]
func (h *ReferralHandler) ValidateCode(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "Referral code is required")
		return
	}

	valid, referrerID, err := h.referralService.ValidateReferralCode(r.Context(), code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to validate code")
		return
	}

	resp := ValidateCodeResponse{Valid: valid}
	if valid {
		resp.ReferrerID = referrerID.String()
	}

	writeJSON(w, http.StatusOK, resp)
}
