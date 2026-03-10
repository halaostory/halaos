-- +goose Up
-- Manager blind spot alerts: weekly automated detection of patterns managers might miss

CREATE TABLE IF NOT EXISTS manager_blind_spots (
    id          BIGSERIAL PRIMARY KEY,
    company_id  BIGINT NOT NULL REFERENCES companies(id),
    manager_id  BIGINT NOT NULL,              -- employee ID of the manager
    spot_type   VARCHAR(50) NOT NULL,          -- category of blind spot
    severity    VARCHAR(10) NOT NULL DEFAULT 'medium',  -- low, medium, high
    title       VARCHAR(200) NOT NULL,
    description TEXT NOT NULL,
    employees   JSONB NOT NULL DEFAULT '[]',   -- [{id, name, detail}]
    is_resolved BOOLEAN NOT NULL DEFAULT false,
    resolved_at TIMESTAMPTZ,
    resolved_by BIGINT,
    week_date   DATE NOT NULL,                 -- Monday of the scoring week
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_blind_spots_company_manager ON manager_blind_spots(company_id, manager_id, week_date DESC);
CREATE INDEX idx_blind_spots_unresolved ON manager_blind_spots(company_id, is_resolved, created_at DESC) WHERE NOT is_resolved;

-- Add blind spot tool to general agent
UPDATE agents SET tools = tools || ARRAY['query_blind_spots'] WHERE slug = 'general';

-- +goose Down
UPDATE agents SET tools = array_remove(tools, 'query_blind_spots') WHERE slug = 'general';
DROP TABLE IF EXISTS manager_blind_spots;
