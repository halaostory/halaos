-- +goose Up

CREATE TABLE company_policies (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    category VARCHAR(50) NOT NULL DEFAULT 'general', -- general, code_of_conduct, safety, benefits, leave, data_privacy, anti_harassment
    version INT NOT NULL DEFAULT 1,
    effective_date DATE NOT NULL DEFAULT CURRENT_DATE,
    requires_acknowledgment BOOLEAN NOT NULL DEFAULT true,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_policies_company ON company_policies(company_id, is_active);

CREATE TABLE policy_acknowledgments (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    policy_id BIGINT NOT NULL REFERENCES company_policies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    acknowledged_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ip_address VARCHAR(45),
    UNIQUE(company_id, policy_id, employee_id)
);

CREATE INDEX idx_policy_ack_policy ON policy_acknowledgments(policy_id);
CREATE INDEX idx_policy_ack_employee ON policy_acknowledgments(employee_id);

-- +goose Down
DROP TABLE IF EXISTS policy_acknowledgments;
DROP TABLE IF EXISTS company_policies;
