-- +goose Up

CREATE TABLE notifications (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    category VARCHAR(30) NOT NULL DEFAULT 'info', -- info, leave, payroll, performance, onboarding, loan, approval
    entity_type VARCHAR(30), -- leave_request, payroll_cycle, performance_review, loan, etc.
    entity_id BIGINT,
    is_read BOOLEAN NOT NULL DEFAULT false,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user ON notifications(user_id, is_read, created_at DESC);
CREATE INDEX idx_notifications_company ON notifications(company_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS notifications;
