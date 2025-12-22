-- Migration: Business Understanding for AI Agent Tool Calling
-- Purpose: Store user business context for personalized AI recommendations

-- Business understanding table
-- Stores incremental understanding of user's business context
CREATE TABLE IF NOT EXISTS business_understanding (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Personal & Role Context
    user_name VARCHAR(255),
    job_title VARCHAR(255),
    business_name VARCHAR(500),
    industry VARCHAR(255),
    business_size VARCHAR(50), -- '1-10', '11-50', '51-200', '201-1000', '1000+'
    user_role VARCHAR(255), -- 'decision maker', 'implementer', 'end user'

    -- Workflows & Activities (stored as JSONB arrays for flexibility)
    key_workflows JSONB DEFAULT '[]'::jsonb,
    daily_activities JSONB DEFAULT '[]'::jsonb,

    -- Pain Points & Challenges
    pain_points JSONB DEFAULT '[]'::jsonb,
    bottlenecks JSONB DEFAULT '[]'::jsonb,
    manual_tasks JSONB DEFAULT '[]'::jsonb,

    -- Goals & Aspirations
    automation_goals JSONB DEFAULT '[]'::jsonb,

    -- Current Tech Stack
    current_software JSONB DEFAULT '[]'::jsonb,
    existing_automation JSONB DEFAULT '[]'::jsonb,

    -- Additional Context
    additional_notes TEXT,

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    -- Ensure one understanding record per user
    UNIQUE(user_id)
);

-- Index for quick user lookups
CREATE INDEX IF NOT EXISTS idx_business_understanding_user ON business_understanding(user_id);

-- Trigger for updated_at
CREATE TRIGGER update_business_understanding_updated_at
    BEFORE UPDATE ON business_understanding
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
