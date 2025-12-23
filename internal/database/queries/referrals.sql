-- Referral Codes

-- name: CreateReferralCode :one
INSERT INTO referral_codes (user_id, code, industry, business_size)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetReferralCodeByUserID :one
SELECT * FROM referral_codes WHERE user_id = $1;

-- name: GetReferralCodeByCode :one
SELECT * FROM referral_codes WHERE code = $1;

-- name: UpdateReferralCodeStats :one
UPDATE referral_codes
SET total_shares = total_shares + $2,
    total_clicks = total_clicks + $3,
    total_signups = total_signups + $4,
    total_activations = total_activations + $5,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: IncrementReferralShares :exec
UPDATE referral_codes SET total_shares = total_shares + 1, updated_at = NOW() WHERE id = $1;

-- name: IncrementReferralClicks :exec
UPDATE referral_codes SET total_clicks = total_clicks + 1, updated_at = NOW() WHERE id = $1;

-- name: IncrementReferralSignups :exec
UPDATE referral_codes SET total_signups = total_signups + 1, updated_at = NOW() WHERE id = $1;

-- name: IncrementReferralActivations :exec
UPDATE referral_codes SET total_activations = total_activations + 1, updated_at = NOW() WHERE id = $1;

-- Referral Shares

-- name: CreateReferralShare :one
INSERT INTO referral_shares (referrer_id, referral_code_id, share_channel, share_source, completed)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetReferralSharesByReferrer :many
SELECT * FROM referral_shares WHERE referrer_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: CountReferralSharesByReferrer :one
SELECT COUNT(*) FROM referral_shares WHERE referrer_id = $1;

-- Referral Clicks

-- name: CreateReferralClick :one
INSERT INTO referral_clicks (
    referral_code_id, visitor_id, ip_hash, user_agent,
    landing_page, utm_source, utm_medium, utm_campaign
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetReferralClickByVisitor :one
SELECT * FROM referral_clicks
WHERE visitor_id = $1 AND converted_user_id IS NULL
ORDER BY created_at DESC
LIMIT 1;

-- name: UpdateReferralClickConverted :exec
UPDATE referral_clicks
SET converted_user_id = $2, converted_at = NOW()
WHERE id = $1;

-- name: GetRecentReferralClick :one
SELECT * FROM referral_clicks
WHERE referral_code_id = $1 AND converted_user_id IS NULL
ORDER BY created_at DESC
LIMIT 1;

-- Referral Signups

-- name: CreateReferralSignup :one
INSERT INTO referral_signups (
    referee_id, referrer_id, referral_code_id, click_id,
    inherited_industry, inherited_business_size, status
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetReferralSignupByReferee :one
SELECT * FROM referral_signups WHERE referee_id = $1;

-- name: GetReferralSignupsByReferrer :many
SELECT rs.*, u.email as referee_email, u.name as referee_name
FROM referral_signups rs
JOIN users u ON rs.referee_id = u.id
WHERE rs.referrer_id = $1
ORDER BY rs.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountReferralSignupsByReferrer :one
SELECT COUNT(*) FROM referral_signups WHERE referrer_id = $1;

-- name: UpdateReferralSignupActivated :exec
UPDATE referral_signups
SET activated_at = NOW(), status = 'activated', updated_at = NOW()
WHERE referee_id = $1;

-- name: UpdateReferralSignupFirstReport :exec
UPDATE referral_signups
SET first_report_at = NOW(), status = 'generated_report', updated_at = NOW()
WHERE referee_id = $1;

-- name: UpdateReferralSignupStatus :exec
UPDATE referral_signups
SET status = $2, updated_at = NOW()
WHERE referee_id = $1;

-- Analytics Queries

-- name: GetReferralStats :one
SELECT
    rc.id,
    rc.code,
    rc.total_shares,
    rc.total_clicks,
    rc.total_signups,
    rc.total_activations,
    CASE WHEN rc.total_clicks > 0
         THEN (rc.total_signups::float / rc.total_clicks::float * 100)::numeric(5,2)
         ELSE 0 END as click_to_signup_rate,
    CASE WHEN rc.total_signups > 0
         THEN (rc.total_activations::float / rc.total_signups::float * 100)::numeric(5,2)
         ELSE 0 END as activation_rate
FROM referral_codes rc
WHERE rc.user_id = $1;

-- name: GetReferralLeaderboard :many
SELECT
    u.id as user_id,
    u.name,
    rc.code,
    rc.total_signups,
    rc.total_activations
FROM referral_codes rc
JOIN users u ON rc.user_id = u.id
WHERE rc.total_signups > 0
ORDER BY rc.total_activations DESC, rc.total_signups DESC
LIMIT $1;
