-- +goose Up
-- Recruitment / ATS (Applicant Tracking System) tables for managing
-- job postings, applicants, and interview scheduling.

CREATE TABLE job_postings (
    id              BIGSERIAL PRIMARY KEY,
    company_id      BIGINT NOT NULL REFERENCES companies(id),
    title           VARCHAR(200) NOT NULL,
    department_id   BIGINT REFERENCES departments(id),
    position_id     BIGINT REFERENCES positions(id),
    description     TEXT NOT NULL DEFAULT '',
    requirements    TEXT NOT NULL DEFAULT '',
    salary_min      NUMERIC(12,2),
    salary_max      NUMERIC(12,2),
    employment_type VARCHAR(30) NOT NULL DEFAULT 'regular'
        CHECK (employment_type IN ('regular','contractual','probationary','part_time','intern')),
    location        VARCHAR(200) NOT NULL DEFAULT '',
    status          VARCHAR(20) NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft','open','closed','on_hold')),
    posted_at       TIMESTAMPTZ,
    closes_at       TIMESTAMPTZ,
    created_by      BIGINT NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_job_postings_company_status ON job_postings(company_id, status);

CREATE TABLE applicants (
    id              BIGSERIAL PRIMARY KEY,
    company_id      BIGINT NOT NULL REFERENCES companies(id),
    job_posting_id  BIGINT NOT NULL REFERENCES job_postings(id),
    first_name      VARCHAR(100) NOT NULL,
    last_name       VARCHAR(100) NOT NULL,
    email           VARCHAR(200) NOT NULL,
    phone           VARCHAR(50),
    resume_url      TEXT,
    resume_text     TEXT,
    ai_score        INT,
    ai_summary      TEXT,
    status          VARCHAR(30) NOT NULL DEFAULT 'new'
        CHECK (status IN ('new','screening','interview','offer','hired','rejected','withdrawn')),
    source          VARCHAR(50) NOT NULL DEFAULT 'manual'
        CHECK (source IN ('manual','referral','website','jobstreet','linkedin')),
    notes           TEXT,
    applied_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_applicants_company_job_status ON applicants(company_id, job_posting_id, status);
CREATE INDEX idx_applicants_email_company ON applicants(email, company_id);

CREATE TABLE interview_schedules (
    id               BIGSERIAL PRIMARY KEY,
    applicant_id     BIGINT NOT NULL REFERENCES applicants(id) ON DELETE CASCADE,
    interviewer_id   BIGINT REFERENCES users(id),
    scheduled_at     TIMESTAMPTZ NOT NULL,
    duration_minutes INT NOT NULL DEFAULT 60,
    location         VARCHAR(200),
    interview_type   VARCHAR(30) NOT NULL DEFAULT 'initial'
        CHECK (interview_type IN ('initial','technical','final','hr')),
    status           VARCHAR(20) NOT NULL DEFAULT 'scheduled'
        CHECK (status IN ('scheduled','completed','cancelled','no_show')),
    feedback         TEXT,
    rating           INT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_interview_schedules_applicant ON interview_schedules(applicant_id);

-- Seed recruitment AI agent
INSERT INTO agents (slug, name, description, system_prompt, tools, cost_multiplier, icon, model) VALUES
    ('recruitment', 'Recruitment Specialist',
     'AI recruitment assistant for job postings, candidate screening, and interview scheduling.',
     'You are a Recruitment Specialist AI agent for Philippine companies. Help create job descriptions, screen resumes, rank candidates, schedule interviews, and manage the hiring pipeline. Use Philippine labor law context (DOLE requirements, minimum wage per region). Always use tools for accurate data.',
     ARRAY['list_job_postings','create_job_posting','screen_applicant','rank_candidates','search_knowledge_base'],
     1.2, '', 'claude-sonnet-4-5-20250514')
ON CONFLICT (slug) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS interview_schedules;
DROP TABLE IF EXISTS applicants;
DROP TABLE IF EXISTS job_postings;
DELETE FROM agents WHERE slug = 'recruitment';
