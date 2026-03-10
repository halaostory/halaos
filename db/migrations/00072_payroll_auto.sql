-- +goose Up

-- Payroll automation configuration per company
CREATE TABLE IF NOT EXISTS payroll_auto_config (
    id              BIGSERIAL PRIMARY KEY,
    company_id      BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    auto_run_enabled   BOOLEAN NOT NULL DEFAULT false,
    days_before_pay    INT NOT NULL DEFAULT 2,
    auto_approve_enabled BOOLEAN NOT NULL DEFAULT false,
    max_auto_approve_amount NUMERIC(15,2) DEFAULT 0,
    notify_on_auto     BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id)
);

-- Log of automatic payroll actions
CREATE TABLE IF NOT EXISTS payroll_auto_log (
    id              BIGSERIAL PRIMARY KEY,
    company_id      BIGINT NOT NULL REFERENCES companies(id),
    cycle_id        BIGINT NOT NULL,
    run_id          BIGINT,
    action          VARCHAR(30) NOT NULL, -- auto_run, auto_approve, auto_skipped
    detail          TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payroll_auto_log_company ON payroll_auto_log(company_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS payroll_auto_log;
DROP TABLE IF EXISTS payroll_auto_config;
