-- +goose Up

CREATE TABLE schedule_templates (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE schedule_template_days (
    id BIGSERIAL PRIMARY KEY,
    template_id BIGINT NOT NULL REFERENCES schedule_templates(id) ON DELETE CASCADE,
    day_of_week INT NOT NULL CHECK (day_of_week BETWEEN 0 AND 6), -- 0=Sun, 6=Sat
    shift_id BIGINT REFERENCES shifts(id),
    is_rest_day BOOLEAN NOT NULL DEFAULT false,
    UNIQUE(template_id, day_of_week)
);

CREATE TABLE employee_schedule_assignments (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    template_id BIGINT NOT NULL REFERENCES schedule_templates(id),
    effective_from DATE NOT NULL,
    effective_to DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, employee_id, effective_from)
);

-- +goose Down
DROP TABLE IF EXISTS employee_schedule_assignments;
DROP TABLE IF EXISTS schedule_template_days;
DROP TABLE IF EXISTS schedule_templates;
