-- Chart-of-accounts mapping for payroll journal entries (QuickBooks / Xero / any GL).

-- +goose Up
CREATE TABLE gl_account_mappings (
    company_id BIGINT PRIMARY KEY REFERENCES companies(id) ON DELETE CASCADE,
    accounts   JSONB NOT NULL DEFAULT '{}',
    updated_by BIGINT REFERENCES users(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS gl_account_mappings;
