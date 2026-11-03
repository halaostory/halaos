-- Rule-based time-off policies (Warp-style policy assignment): entitlement
-- per leave type by department, employment type, gender and tenure. The
-- most specific matching rule wins; otherwise the leave type's default.

-- +goose Up
CREATE TABLE leave_policy_rules (
    id                BIGSERIAL PRIMARY KEY,
    company_id        BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    leave_type_id     BIGINT NOT NULL REFERENCES leave_types(id) ON DELETE CASCADE,
    name              VARCHAR(120) NOT NULL,
    priority          INT NOT NULL DEFAULT 0,
    department_ids    BIGINT[] NOT NULL DEFAULT '{}',
    employment_types  TEXT[] NOT NULL DEFAULT '{}',
    gender            VARCHAR(10) NOT NULL DEFAULT '',
    min_tenure_months INT NOT NULL DEFAULT 0,
    max_tenure_months INT,
    days              NUMERIC(5, 1) NOT NULL,
    is_active         BOOLEAN NOT NULL DEFAULT true,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_leave_policy_rules_company ON leave_policy_rules (company_id, leave_type_id);

-- +goose Down
DROP TABLE IF EXISTS leave_policy_rules;
