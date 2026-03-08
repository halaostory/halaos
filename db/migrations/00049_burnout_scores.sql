-- +goose Up
CREATE TABLE employee_burnout_scores (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    burnout_score INT NOT NULL DEFAULT 0,
    factors JSONB NOT NULL DEFAULT '[]',
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, employee_id)
);
CREATE INDEX idx_burnout_scores_high ON employee_burnout_scores(company_id, burnout_score DESC);

-- +goose Down
DROP TABLE IF EXISTS employee_burnout_scores;
