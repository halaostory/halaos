-- +goose Up
CREATE TABLE compliance_alerts (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    alert_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'medium',
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    entity_type VARCHAR(50),
    entity_id BIGINT,
    due_date DATE,
    days_remaining INT,
    is_resolved BOOLEAN NOT NULL DEFAULT false,
    resolved_at TIMESTAMPTZ,
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, alert_type, entity_type, COALESCE(entity_id, 0), due_date)
);
CREATE INDEX idx_compliance_alerts_active ON compliance_alerts(company_id, is_resolved, severity);
CREATE INDEX idx_compliance_alerts_due ON compliance_alerts(company_id, due_date) WHERE NOT is_resolved;

-- +goose Down
DROP TABLE IF EXISTS compliance_alerts;
