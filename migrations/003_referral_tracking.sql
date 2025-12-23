-- Migration: Referral Tracking System
-- Purpose: Track referral loops triggered from business report generation
-- Expert guidance: Brian Balfour, Andrew Chen, Elena Verna best practices

-- Referral codes table
-- Each user gets a unique referral code for sharing
CREATE TABLE IF NOT EXISTS referral_codes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Unique referral code (short, shareable)
    code VARCHAR(12) NOT NULL UNIQUE,

    -- Context from when code was generated (for recipient pre-population)
    industry VARCHAR(255),
    business_size VARCHAR(50),

    -- Statistics (denormalized for quick access)
    total_shares INT DEFAULT 0,
    total_clicks INT DEFAULT 0,
    total_signups INT DEFAULT 0,
    total_activations INT DEFAULT 0,

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    -- One code per user
    UNIQUE(user_id)
);

-- Referral shares table
-- Tracks when a user initiates a share (Elena Verna: track intent vs completion)
CREATE TABLE IF NOT EXISTS referral_shares (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    referrer_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    referral_code_id UUID NOT NULL REFERENCES referral_codes(id) ON DELETE CASCADE,

    -- Share context
    share_channel VARCHAR(50) NOT NULL, -- 'copy_link', 'email', 'twitter', 'linkedin'
    share_source VARCHAR(100), -- 'business_report', 'dashboard', 'profile'

    -- Was share completed (if trackable)
    completed BOOLEAN DEFAULT false,

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Referral clicks table
-- Tracks when someone clicks a referral link
CREATE TABLE IF NOT EXISTS referral_clicks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    referral_code_id UUID NOT NULL REFERENCES referral_codes(id) ON DELETE CASCADE,

    -- Visitor identification (before signup)
    visitor_id VARCHAR(255), -- Anonymous identifier from cookie/fingerprint
    ip_hash VARCHAR(64), -- Hashed IP for deduplication
    user_agent TEXT,

    -- Attribution
    landing_page VARCHAR(500),
    utm_source VARCHAR(100),
    utm_medium VARCHAR(100),
    utm_campaign VARCHAR(100),

    -- Conversion tracking
    converted_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    converted_at TIMESTAMPTZ,

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Referral signups table
-- Tracks users who signed up via a referral (Andrew Chen: full attribution)
CREATE TABLE IF NOT EXISTS referral_signups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- The new user who signed up
    referee_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- The user who referred them
    referrer_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- The referral code used
    referral_code_id UUID NOT NULL REFERENCES referral_codes(id) ON DELETE CASCADE,

    -- Attribution details
    click_id UUID REFERENCES referral_clicks(id) ON DELETE SET NULL,

    -- Pre-populated context (from referrer)
    inherited_industry VARCHAR(255),
    inherited_business_size VARCHAR(50),

    -- Activation tracking (Kieran Flanagan: track full journey)
    activated_at TIMESTAMPTZ, -- When referee reached activation milestone
    first_report_at TIMESTAMPTZ, -- When referee generated their first report

    -- Status
    status VARCHAR(50) DEFAULT 'signed_up', -- 'signed_up', 'activated', 'generated_report', 'referred_others'

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    -- One referral record per user
    UNIQUE(referee_id)
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_referral_codes_code ON referral_codes(code);
CREATE INDEX IF NOT EXISTS idx_referral_codes_user ON referral_codes(user_id);
CREATE INDEX IF NOT EXISTS idx_referral_shares_referrer ON referral_shares(referrer_id);
CREATE INDEX IF NOT EXISTS idx_referral_shares_created ON referral_shares(created_at);
CREATE INDEX IF NOT EXISTS idx_referral_clicks_code ON referral_clicks(referral_code_id);
CREATE INDEX IF NOT EXISTS idx_referral_clicks_created ON referral_clicks(created_at);
CREATE INDEX IF NOT EXISTS idx_referral_clicks_visitor ON referral_clicks(visitor_id);
CREATE INDEX IF NOT EXISTS idx_referral_signups_referrer ON referral_signups(referrer_id);
CREATE INDEX IF NOT EXISTS idx_referral_signups_referee ON referral_signups(referee_id);
CREATE INDEX IF NOT EXISTS idx_referral_signups_status ON referral_signups(status);

-- Markus Winand: Index for conversion status lookups
CREATE INDEX IF NOT EXISTS idx_referral_clicks_converted ON referral_clicks(converted_user_id)
    WHERE converted_user_id IS NOT NULL;

-- Markus Winand: Partial index for finding unconverted clicks by visitor
-- Optimizes GetReferralClickByVisitor query
CREATE INDEX IF NOT EXISTS idx_referral_clicks_unconverted ON referral_clicks(visitor_id, created_at DESC)
    WHERE converted_user_id IS NULL;

-- Triggers for updated_at
CREATE TRIGGER update_referral_codes_updated_at
    BEFORE UPDATE ON referral_codes
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_referral_signups_updated_at
    BEFORE UPDATE ON referral_signups
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
