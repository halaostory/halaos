-- Government notices (Warp-style notice handling): letters and e-mails from
-- BIR, SSS, PhilHealth, Pag-IBIG, IRS or a state agency. The agent triages
-- each notice, matches it against the company's own filings and drafts a
-- response (or prepares the missing return). Nothing is sent without a
-- person's approval; audits are escalated.

-- +goose Up
CREATE TABLE gov_notices (
    id                 BIGSERIAL PRIMARY KEY,
    company_id         BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    agency             VARCHAR(30) NOT NULL DEFAULT '',
    notice_type        VARCHAR(30) NOT NULL DEFAULT 'other'
        CHECK (notice_type IN ('missing_filing', 'late_payment', 'assessment', 'audit', 'registration', 'informational', 'other')),
    form_code          VARCHAR(50) NOT NULL DEFAULT '',
    period             VARCHAR(20) NOT NULL DEFAULT '',
    reference_no       VARCHAR(100) NOT NULL DEFAULT '',
    amount             NUMERIC(14, 2),
    respond_by         DATE,
    title              VARCHAR(300) NOT NULL DEFAULT '',
    summary            TEXT NOT NULL DEFAULT '',
    recommendation     TEXT NOT NULL DEFAULT '',
    resolution_path    VARCHAR(30) NOT NULL DEFAULT 'review'
        CHECK (resolution_path IN ('proof_of_filing', 'prepare_filing', 'pay', 'update_records', 'no_action', 'escalate', 'review')),
    draft_response     TEXT NOT NULL DEFAULT '',
    matched_submission_id BIGINT REFERENCES gov_submissions(id) ON DELETE SET NULL,
    status             VARCHAR(20) NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'awaiting_approval', 'escalated', 'resolved')),
    auto_resolvable    BOOLEAN NOT NULL DEFAULT false,
    triaged_by         VARCHAR(10) NOT NULL DEFAULT 'rules',
    source_text        TEXT NOT NULL DEFAULT '',
    file_name          VARCHAR(255),
    file_path          TEXT,
    resolution_note    TEXT NOT NULL DEFAULT '',
    resolved_by        BIGINT REFERENCES users(id),
    resolved_at        TIMESTAMPTZ,
    created_by         BIGINT REFERENCES users(id),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_gov_notices_company ON gov_notices (company_id, status, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS gov_notices;
