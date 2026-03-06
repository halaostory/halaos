-- +goose Up

-- Disciplinary incidents
CREATE TABLE disciplinary_incidents (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    reported_by BIGINT REFERENCES users(id),
    incident_date DATE NOT NULL,
    category VARCHAR(50) NOT NULL, -- tardiness, absence, misconduct, insubordination, policy_violation, performance, safety
    severity VARCHAR(20) NOT NULL DEFAULT 'minor', -- minor, moderate, major, grave
    description TEXT NOT NULL,
    witnesses TEXT, -- comma-separated names
    evidence_notes TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'open', -- open, under_review, resolved, dismissed
    resolution_notes TEXT,
    resolved_at TIMESTAMPTZ,
    resolved_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_disciplinary_incidents_company ON disciplinary_incidents(company_id, status);
CREATE INDEX idx_disciplinary_incidents_employee ON disciplinary_incidents(employee_id);

-- Disciplinary actions (progressive discipline)
CREATE TABLE disciplinary_actions (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    incident_id BIGINT REFERENCES disciplinary_incidents(id),
    action_type VARCHAR(30) NOT NULL, -- verbal_warning, written_warning, final_warning, suspension, termination
    action_date DATE NOT NULL,
    issued_by BIGINT NOT NULL REFERENCES users(id),
    description TEXT NOT NULL,
    suspension_days INT, -- for suspension type
    effective_date DATE, -- for suspension/termination
    end_date DATE, -- for suspension
    employee_acknowledged BOOLEAN DEFAULT false,
    acknowledged_at TIMESTAMPTZ,
    appeal_status VARCHAR(20), -- null, appealed, appeal_denied, appeal_granted
    appeal_reason TEXT,
    appeal_date TIMESTAMPTZ,
    appeal_resolved_at TIMESTAMPTZ,
    appeal_resolved_by BIGINT REFERENCES users(id),
    appeal_resolution TEXT,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_disciplinary_actions_company ON disciplinary_actions(company_id);
CREATE INDEX idx_disciplinary_actions_employee ON disciplinary_actions(employee_id);
CREATE INDEX idx_disciplinary_actions_incident ON disciplinary_actions(incident_id);

-- +goose Down
DROP TABLE IF EXISTS disciplinary_actions;
DROP TABLE IF EXISTS disciplinary_incidents;
