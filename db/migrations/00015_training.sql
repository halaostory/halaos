-- +goose Up
CREATE TABLE IF NOT EXISTS trainings (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    trainer VARCHAR(255),
    training_type VARCHAR(50) NOT NULL DEFAULT 'internal', -- internal, external, online
    start_date DATE NOT NULL,
    end_date DATE,
    max_participants INT,
    status VARCHAR(20) NOT NULL DEFAULT 'scheduled', -- scheduled, in_progress, completed, cancelled
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS training_participants (
    id BIGSERIAL PRIMARY KEY,
    training_id BIGINT NOT NULL REFERENCES trainings(id) ON DELETE CASCADE,
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    status VARCHAR(20) NOT NULL DEFAULT 'enrolled', -- enrolled, completed, no_show
    score NUMERIC(5,2),
    feedback TEXT,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(training_id, employee_id)
);

CREATE TABLE IF NOT EXISTS certifications (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    name VARCHAR(255) NOT NULL,
    issuing_body VARCHAR(255),
    credential_id VARCHAR(100),
    issue_date DATE NOT NULL,
    expiry_date DATE,
    status VARCHAR(20) NOT NULL DEFAULT 'active', -- active, expired, revoked
    attachment_path VARCHAR(500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_trainings_company ON trainings(company_id);
CREATE INDEX idx_training_participants_training ON training_participants(training_id);
CREATE INDEX idx_training_participants_employee ON training_participants(employee_id);
CREATE INDEX idx_certifications_company ON certifications(company_id);
CREATE INDEX idx_certifications_employee ON certifications(employee_id);

-- +goose Down
DROP TABLE IF EXISTS certifications;
DROP TABLE IF EXISTS training_participants;
DROP TABLE IF EXISTS trainings;
