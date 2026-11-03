-- Agent routines (Warp-style): jobs the AI agent owns, triggered by an event
-- (employee hired, employee terminated, payroll calculated) or a schedule.
-- Each company picks an autonomy level per routine and every run is logged
-- with what the agent did, why, and what it left for a person.

-- +goose Up
CREATE TABLE agent_routines (
    id           BIGSERIAL PRIMARY KEY,
    company_id   BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    routine_key  VARCHAR(50) NOT NULL,
    enabled      BOOLEAN NOT NULL DEFAULT true,
    autonomy     VARCHAR(20) NOT NULL DEFAULT 'approve'
        CHECK (autonomy IN ('recommend', 'approve', 'autonomous')),
    instructions TEXT NOT NULL DEFAULT '',
    updated_by   BIGINT REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, routine_key)
);

CREATE TABLE agent_routine_runs (
    id           BIGSERIAL PRIMARY KEY,
    company_id   BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    routine_key  VARCHAR(50) NOT NULL,
    trigger      VARCHAR(200) NOT NULL,
    subject_type VARCHAR(30) NOT NULL DEFAULT '',
    subject_id   BIGINT NOT NULL DEFAULT 0,
    subject_name VARCHAR(200) NOT NULL DEFAULT '',
    autonomy     VARCHAR(20) NOT NULL,
    status       VARCHAR(20) NOT NULL
        CHECK (status IN ('completed', 'needs_attention', 'failed')),
    summary      TEXT NOT NULL DEFAULT '',
    actions      JSONB NOT NULL DEFAULT '[]',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_agent_routine_runs_company ON agent_routine_runs (company_id, created_at DESC);
CREATE INDEX idx_agent_routine_runs_subject ON agent_routine_runs (company_id, routine_key, subject_type, subject_id);

-- +goose Down
DROP TABLE IF EXISTS agent_routine_runs;
DROP TABLE IF EXISTS agent_routines;
