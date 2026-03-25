-- +goose Up
CREATE TABLE onboarding_progress (
    id           BIGSERIAL PRIMARY KEY,
    company_id   BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    persona      VARCHAR(20) NOT NULL DEFAULT 'employee',
    steps        JSONB NOT NULL DEFAULT '{}',
    dismissed    BOOLEAN NOT NULL DEFAULT FALSE,
    completed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, user_id, persona)
);

CREATE INDEX idx_onboarding_progress_company ON onboarding_progress(company_id);

-- +goose Down
DROP TABLE IF EXISTS onboarding_progress;
