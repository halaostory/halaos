-- +goose Up

-- Government filing agent: prepares a return, gets it reviewed (rules + AI),
-- waits for HR approval, then submits it through the agency's channel
-- (official API, e-mail submission, supervised browser automation, or a
-- guided manual hand-off) and records the government reference number.

-- Encrypted credentials for government portals (eFPS, My.SSS, EPRS, eSRS, CorpPass…)
CREATE TABLE IF NOT EXISTS gov_portal_accounts (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    agency VARCHAR(30) NOT NULL,
    portal VARCHAR(50) NOT NULL,
    label VARCHAR(120) NOT NULL DEFAULT '',
    username VARCHAR(255) NOT NULL DEFAULT '',
    credentials_enc TEXT NOT NULL DEFAULT '',
    otp_channel VARCHAR(20) NOT NULL DEFAULT 'none'
        CHECK (otp_channel IN ('none', 'sms', 'email', 'authenticator')),
    status VARCHAR(20) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'needs_attention', 'disabled')),
    last_used_at TIMESTAMPTZ,
    last_error TEXT,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (company_id, agency, portal)
);

CREATE TABLE IF NOT EXISTS gov_submissions (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    agency VARCHAR(30) NOT NULL,
    form_code VARCHAR(50) NOT NULL,
    period VARCHAR(20) NOT NULL,
    tax_filing_id BIGINT REFERENCES tax_filings(id),
    channel VARCHAR(20) NOT NULL
        CHECK (channel IN ('api', 'email', 'browser', 'manual')),
    playbook_id VARCHAR(80),
    portal_account_id BIGINT REFERENCES gov_portal_accounts(id),
    status VARCHAR(20) NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'ready', 'approved', 'running', 'needs_human',
                          'submitted', 'failed', 'cancelled')),
    file_name VARCHAR(255),
    file_path TEXT,
    file_sha256 VARCHAR(64),
    data JSONB NOT NULL DEFAULT '{}',
    preflight JSONB NOT NULL DEFAULT '{}',
    instructions TEXT,
    due_date DATE,
    approved_by BIGINT REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    submitted_at TIMESTAMPTZ,
    reference_no VARCHAR(120),
    outputs JSONB NOT NULL DEFAULT '{}',
    error_message TEXT,
    attempts INT NOT NULL DEFAULT 0,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- At most one live submission per return/period (failed/cancelled ones may be retried).
CREATE UNIQUE INDEX IF NOT EXISTS idx_gov_submissions_live
    ON gov_submissions(company_id, agency, form_code, period)
    WHERE status NOT IN ('failed', 'cancelled');
CREATE INDEX IF NOT EXISTS idx_gov_submissions_queue ON gov_submissions(status, updated_at);
CREATE INDEX IF NOT EXISTS idx_gov_submissions_company ON gov_submissions(company_id, created_at DESC);

-- Audit trail: every step, screenshot and human hand-off.
CREATE TABLE IF NOT EXISTS gov_submission_events (
    id BIGSERIAL PRIMARY KEY,
    submission_id BIGINT NOT NULL REFERENCES gov_submissions(id) ON DELETE CASCADE,
    kind VARCHAR(20) NOT NULL
        CHECK (kind IN ('status', 'log', 'evidence', 'human_request', 'human_answer', 'ai')),
    message TEXT NOT NULL DEFAULT '',
    data JSONB NOT NULL DEFAULT '{}',
    actor_user_id BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_gov_submission_events ON gov_submission_events(submission_id, id);

-- CAPTCHA / OTP / confirmation requests raised by a running browser session.
CREATE TABLE IF NOT EXISTS gov_human_requests (
    id BIGSERIAL PRIMARY KEY,
    submission_id BIGINT NOT NULL REFERENCES gov_submissions(id) ON DELETE CASCADE,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name VARCHAR(50) NOT NULL,
    prompt TEXT NOT NULL,
    screenshot_path TEXT,
    answer_enc TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'answered', 'consumed', 'expired')),
    answered_by BIGINT REFERENCES users(id),
    answered_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_gov_human_requests_open ON gov_human_requests(company_id, status) WHERE status = 'open';

-- Browser playbooks. company_id NULL = built-in. A playbook must be marked
-- verified (after a supervised dry run against the live portal) before the
-- agent will run it unattended up to the final submit step.
CREATE TABLE IF NOT EXISTS gov_playbooks (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT REFERENCES companies(id),
    playbook_key VARCHAR(80) NOT NULL,
    form_code VARCHAR(50) NOT NULL,
    definition JSONB NOT NULL,
    verified BOOLEAN NOT NULL DEFAULT false,
    verified_by BIGINT REFERENCES users(id),
    verified_at TIMESTAMPTZ,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_gov_playbooks_key ON gov_playbooks(COALESCE(company_id, 0), playbook_key);

ALTER TABLE gov_submissions ADD COLUMN IF NOT EXISTS dry_run BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
DROP TABLE IF EXISTS gov_playbooks;
DROP TABLE IF EXISTS gov_human_requests;
DROP TABLE IF EXISTS gov_submission_events;
DROP TABLE IF EXISTS gov_submissions;
DROP TABLE IF EXISTS gov_portal_accounts;
