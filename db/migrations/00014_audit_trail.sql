-- +goose Up

CREATE TABLE activity_logs (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    action VARCHAR(50) NOT NULL,  -- create, update, delete, approve, reject, login, logout
    entity_type VARCHAR(50) NOT NULL,  -- employee, leave_request, payroll, etc.
    entity_id VARCHAR(50),  -- ID of the affected entity
    description TEXT NOT NULL,
    ip_address VARCHAR(45),
    user_agent TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_activity_logs_company ON activity_logs(company_id, created_at DESC);
CREATE INDEX idx_activity_logs_user ON activity_logs(user_id, created_at DESC);
CREATE INDEX idx_activity_logs_entity ON activity_logs(entity_type, entity_id);
CREATE INDEX idx_activity_logs_action ON activity_logs(action, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS activity_logs;
