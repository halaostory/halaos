-- +goose Up
CREATE TABLE IF NOT EXISTS team_health_scores (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    department_id BIGINT NOT NULL REFERENCES departments(id),
    department_name TEXT NOT NULL,
    health_score INT NOT NULL DEFAULT 0,
    factors JSONB NOT NULL DEFAULT '{}',
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, department_id)
);
CREATE INDEX idx_team_health_company_score ON team_health_scores (company_id, health_score DESC);

-- +goose Down
DROP TABLE IF EXISTS team_health_scores;
