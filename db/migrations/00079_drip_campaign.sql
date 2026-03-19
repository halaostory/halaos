-- +goose Up

-- Track drip emails sent to companies
CREATE TABLE IF NOT EXISTS drip_emails (
    id          BIGSERIAL PRIMARY KEY,
    company_id  BIGINT NOT NULL REFERENCES companies(id),
    step        INT NOT NULL,          -- 1=getting_started, 2=first_employee, 3=first_payroll, 4=explore_features
    sent_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(company_id, step)
);

CREATE INDEX idx_drip_emails_company ON drip_emails(company_id);

-- +goose Down
DROP TABLE IF EXISTS drip_emails;
