-- +goose Up

-- Employee lifecycle (Warp-style "one action" hire / terminate) + custom fields.

-- Structured separation record. One employee may have several over time (rehires).
CREATE TABLE IF NOT EXISTS employee_separations (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    separation_type VARCHAR(30) NOT NULL
        CHECK (separation_type IN ('resignation', 'termination', 'end_of_contract', 'retirement', 'layoff', 'death', 'other')),
    reason TEXT,
    notice_date DATE NOT NULL,
    last_working_day DATE NOT NULL,
    rehire_eligible BOOLEAN NOT NULL DEFAULT true,
    clearance_id BIGINT REFERENCES clearance_requests(id),
    revoke_access BOOLEAN NOT NULL DEFAULT true,
    -- scheduled: waiting for last_working_day; completed: employee separated + access revoked
    status VARCHAR(20) NOT NULL DEFAULT 'scheduled'
        CHECK (status IN ('scheduled', 'completed', 'cancelled')),
    completed_at TIMESTAMPTZ,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_employee_separations_employee ON employee_separations(employee_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_employee_separations_due ON employee_separations(status, last_working_day) WHERE status = 'scheduled';

-- Company-defined employee profile fields.
CREATE TABLE IF NOT EXISTS employee_custom_fields (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    field_key VARCHAR(64) NOT NULL,
    label VARCHAR(120) NOT NULL,
    field_type VARCHAR(20) NOT NULL DEFAULT 'text'
        CHECK (field_type IN ('text', 'number', 'date', 'select', 'boolean')),
    options TEXT[] NOT NULL DEFAULT '{}',
    is_required BOOLEAN NOT NULL DEFAULT false,
    is_sensitive BOOLEAN NOT NULL DEFAULT false,
    sort_order INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (company_id, field_key)
);

CREATE TABLE IF NOT EXISTS employee_custom_field_values (
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    field_id BIGINT NOT NULL REFERENCES employee_custom_fields(id) ON DELETE CASCADE,
    value TEXT NOT NULL,
    updated_by BIGINT REFERENCES users(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (employee_id, field_id)
);

CREATE INDEX IF NOT EXISTS idx_custom_field_values_company ON employee_custom_field_values(company_id);

-- +goose Down

DROP TABLE IF EXISTS employee_custom_field_values;
DROP TABLE IF EXISTS employee_custom_fields;
DROP TABLE IF EXISTS employee_separations;
