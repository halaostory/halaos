-- +goose Up

-- Review cycles (annual, quarterly, probation)
CREATE TABLE review_cycles (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name VARCHAR(255) NOT NULL,
    cycle_type VARCHAR(30) NOT NULL DEFAULT 'annual', -- annual, quarterly, probation, project
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    review_deadline DATE,
    status VARCHAR(20) NOT NULL DEFAULT 'draft', -- draft, active, closed
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_review_cycles_company ON review_cycles(company_id, status);

-- Goals (employee-level, tied to review cycle or standalone)
CREATE TABLE goals (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    review_cycle_id BIGINT REFERENCES review_cycles(id),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(50) NOT NULL DEFAULT 'individual', -- individual, team, company
    weight NUMERIC(5,2) NOT NULL DEFAULT 0, -- percentage weight for scoring
    target_value VARCHAR(255), -- measurable target
    actual_value VARCHAR(255), -- actual achievement
    status VARCHAR(20) NOT NULL DEFAULT 'draft', -- draft, active, completed, cancelled
    due_date DATE,
    completed_at TIMESTAMPTZ,
    self_rating INT, -- 1-5
    manager_rating INT, -- 1-5
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_goals_employee ON goals(employee_id, review_cycle_id);
CREATE INDEX idx_goals_company ON goals(company_id, status);

-- Performance reviews (one per employee per review cycle)
CREATE TABLE performance_reviews (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    review_cycle_id BIGINT NOT NULL REFERENCES review_cycles(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    reviewer_id BIGINT REFERENCES employees(id),
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, self_review, manager_review, completed
    -- Self assessment
    self_rating INT, -- 1-5 overall
    self_comments TEXT,
    self_submitted_at TIMESTAMPTZ,
    -- Manager assessment
    manager_rating INT, -- 1-5 overall
    manager_comments TEXT,
    manager_submitted_at TIMESTAMPTZ,
    -- Final
    final_rating INT, -- 1-5
    final_comments TEXT,
    strengths TEXT,
    improvements TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(review_cycle_id, employee_id)
);

CREATE INDEX idx_reviews_employee ON performance_reviews(employee_id);
CREATE INDEX idx_reviews_reviewer ON performance_reviews(reviewer_id, status);
CREATE INDEX idx_reviews_cycle ON performance_reviews(review_cycle_id, status);

-- +goose Down
DROP TABLE IF EXISTS performance_reviews;
DROP TABLE IF EXISTS goals;
DROP TABLE IF EXISTS review_cycles;
