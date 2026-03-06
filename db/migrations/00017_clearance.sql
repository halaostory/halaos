-- +goose Up

CREATE TABLE clearance_requests (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    resignation_date DATE NOT NULL,
    last_working_day DATE NOT NULL,
    reason TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, in_progress, completed, cancelled
    submitted_by BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE clearance_items (
    id BIGSERIAL PRIMARY KEY,
    clearance_id BIGINT NOT NULL REFERENCES clearance_requests(id) ON DELETE CASCADE,
    department VARCHAR(100) NOT NULL, -- e.g., IT, HR, Finance, Admin, Direct Manager
    item_name VARCHAR(200) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, cleared, not_applicable
    cleared_by BIGINT REFERENCES users(id),
    cleared_at TIMESTAMPTZ,
    remarks TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_clearance_requests_employee ON clearance_requests(employee_id);
CREATE INDEX idx_clearance_requests_company ON clearance_requests(company_id, status);
CREATE INDEX idx_clearance_items_clearance ON clearance_items(clearance_id);

-- Seed default clearance items template
CREATE TABLE clearance_templates (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    department VARCHAR(100) NOT NULL,
    item_name VARCHAR(200) NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    UNIQUE(company_id, department, item_name)
);

-- +goose Down

DROP TABLE IF EXISTS clearance_templates;
DROP TABLE IF EXISTS clearance_items;
DROP TABLE IF EXISTS clearance_requests;
