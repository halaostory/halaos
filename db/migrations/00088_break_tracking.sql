-- +goose Up

-- Break policies: company-level per-break-type overtime thresholds
CREATE TABLE break_policies (
    id              BIGSERIAL PRIMARY KEY,
    company_id      BIGINT NOT NULL REFERENCES companies(id),
    break_type      VARCHAR(20) NOT NULL,
    max_minutes     INT NOT NULL,
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, break_type)
);

-- Break logs: one row per break session
CREATE TABLE break_logs (
    id                BIGSERIAL PRIMARY KEY,
    company_id        BIGINT NOT NULL REFERENCES companies(id),
    employee_id       BIGINT NOT NULL REFERENCES employees(id),
    attendance_log_id BIGINT NOT NULL REFERENCES attendance_logs(id),
    break_type        VARCHAR(20) NOT NULL CHECK (break_type IN ('meal','bathroom','rest','leave_post')),
    start_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    end_at            TIMESTAMPTZ,
    duration_minutes  INT,
    overtime_minutes  INT DEFAULT 0,
    note              TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_break_logs_attendance ON break_logs(attendance_log_id);
CREATE INDEX idx_break_logs_employee_date ON break_logs(employee_id, start_at DESC);
CREATE INDEX idx_break_logs_company_period ON break_logs(company_id, start_at);

-- Seed default break policies for all existing companies
INSERT INTO break_policies (company_id, break_type, max_minutes)
SELECT c.id, bt.break_type, bt.max_minutes
FROM companies c
CROSS JOIN (VALUES
    ('meal', 30),
    ('bathroom', 5),
    ('rest', 0),
    ('leave_post', 0)
) AS bt(break_type, max_minutes)
ON CONFLICT (company_id, break_type) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS break_logs;
DROP TABLE IF EXISTS break_policies;
