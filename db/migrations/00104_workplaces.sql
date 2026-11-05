-- Workplaces (Warp: company work locations; policies and registrations
-- follow where people work).

-- +goose Up
CREATE TABLE workplaces (
    id         BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    name       VARCHAR(120) NOT NULL,
    address    TEXT NOT NULL DEFAULT '',
    city       VARCHAR(100) NOT NULL DEFAULT '',
    region     VARCHAR(100) NOT NULL DEFAULT '',
    country    VARCHAR(3) NOT NULL DEFAULT 'PHL',
    is_remote  BOOLEAN NOT NULL DEFAULT false,
    is_active  BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, name)
);

CREATE TABLE employee_workplaces (
    employee_id  BIGINT PRIMARY KEY REFERENCES employees(id) ON DELETE CASCADE,
    company_id   BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    workplace_id BIGINT NOT NULL REFERENCES workplaces(id) ON DELETE CASCADE,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_employee_workplaces_wp ON employee_workplaces (workplace_id);

ALTER TABLE leave_policy_rules ADD COLUMN workplace_ids BIGINT[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE leave_policy_rules DROP COLUMN IF EXISTS workplace_ids;
DROP TABLE IF EXISTS employee_workplaces;
DROP TABLE IF EXISTS workplaces;
