-- +goose Up
CREATE TABLE employee_risk_scores (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    risk_score INT NOT NULL DEFAULT 0,
    factors JSONB NOT NULL DEFAULT '[]',
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, employee_id)
);
CREATE INDEX idx_risk_scores_high ON employee_risk_scores(company_id, risk_score DESC);

-- +goose Down
DROP TABLE IF EXISTS employee_risk_scores;
