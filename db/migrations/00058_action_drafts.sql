-- +goose Up
CREATE TABLE action_drafts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id BIGINT NOT NULL REFERENCES companies(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    session_id UUID REFERENCES chat_sessions(id) ON DELETE SET NULL,
    tool_name TEXT NOT NULL,
    tool_input JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'rejected', 'expired', 'executed')),
    risk_level TEXT NOT NULL DEFAULT 'medium' CHECK (risk_level IN ('low', 'medium', 'high')),
    description TEXT NOT NULL DEFAULT '',
    result JSONB,
    error_message TEXT,
    expires_at TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '10 minutes',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_action_drafts_company_user ON action_drafts(company_id, user_id);
CREATE INDEX idx_action_drafts_pending ON action_drafts(status, expires_at) WHERE status = 'pending';

-- +goose Down
DROP TABLE IF EXISTS action_drafts;
