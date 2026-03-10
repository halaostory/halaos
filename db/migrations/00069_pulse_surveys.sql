-- +goose Up
-- Pulse survey system for quick employee sentiment tracking

CREATE TABLE IF NOT EXISTS pulse_surveys (
    id          BIGSERIAL PRIMARY KEY,
    company_id  BIGINT NOT NULL REFERENCES companies(id),
    title       VARCHAR(200) NOT NULL,
    description TEXT,
    frequency   VARCHAR(20) NOT NULL DEFAULT 'weekly',  -- weekly, biweekly, monthly, one_time
    is_anonymous BOOLEAN NOT NULL DEFAULT true,
    is_active   BOOLEAN NOT NULL DEFAULT true,
    created_by  BIGINT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS pulse_questions (
    id          BIGSERIAL PRIMARY KEY,
    survey_id   BIGINT NOT NULL REFERENCES pulse_surveys(id) ON DELETE CASCADE,
    question    TEXT NOT NULL,
    question_type VARCHAR(20) NOT NULL DEFAULT 'rating',  -- rating (1-5), text, yes_no
    sort_order  INT NOT NULL DEFAULT 0,
    is_required BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE IF NOT EXISTS pulse_rounds (
    id          BIGSERIAL PRIMARY KEY,
    survey_id   BIGINT NOT NULL REFERENCES pulse_surveys(id) ON DELETE CASCADE,
    company_id  BIGINT NOT NULL REFERENCES companies(id),
    round_date  DATE NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'open',  -- open, closed
    total_sent  INT NOT NULL DEFAULT 0,
    total_responded INT NOT NULL DEFAULT 0,
    closed_at   TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS pulse_responses (
    id          BIGSERIAL PRIMARY KEY,
    round_id    BIGINT NOT NULL REFERENCES pulse_rounds(id) ON DELETE CASCADE,
    question_id BIGINT NOT NULL REFERENCES pulse_questions(id) ON DELETE CASCADE,
    employee_id BIGINT NOT NULL,
    company_id  BIGINT NOT NULL REFERENCES companies(id),
    rating      INT,                    -- 1-5 for rating type
    answer_text TEXT,                   -- for text/yes_no type
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(round_id, question_id, employee_id)
);

CREATE INDEX idx_pulse_surveys_company ON pulse_surveys(company_id, is_active);
CREATE INDEX idx_pulse_rounds_survey ON pulse_rounds(survey_id, round_date DESC);
CREATE INDEX idx_pulse_rounds_open ON pulse_rounds(company_id, status, round_date) WHERE status = 'open';
CREATE INDEX idx_pulse_responses_round ON pulse_responses(round_id, question_id);
CREATE INDEX idx_pulse_responses_employee ON pulse_responses(company_id, employee_id, submitted_at DESC);

-- Add pulse survey tool to general agent
UPDATE agents SET tools = tools || ARRAY['query_pulse_results'] WHERE slug = 'general';

-- +goose Down
UPDATE agents SET tools = array_remove(tools, 'query_pulse_results') WHERE slug = 'general';
DROP TABLE IF EXISTS pulse_responses;
DROP TABLE IF EXISTS pulse_rounds;
DROP TABLE IF EXISTS pulse_questions;
DROP TABLE IF EXISTS pulse_surveys;
