-- Government filing agent.

-- name: UpsertGovPortalAccount :one
INSERT INTO gov_portal_accounts (company_id, agency, portal, label, username, credentials_enc, otp_channel, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (company_id, agency, portal) DO UPDATE SET
    label = EXCLUDED.label,
    username = EXCLUDED.username,
    credentials_enc = CASE WHEN EXCLUDED.credentials_enc = '' THEN gov_portal_accounts.credentials_enc ELSE EXCLUDED.credentials_enc END,
    otp_channel = EXCLUDED.otp_channel,
    status = 'active',
    last_error = NULL,
    updated_at = NOW()
RETURNING *;

-- name: ListGovPortalAccounts :many
SELECT id, company_id, agency, portal, label, username, otp_channel, status, last_used_at, last_error, created_at, updated_at,
       (credentials_enc <> '')::boolean AS has_credentials
FROM gov_portal_accounts
WHERE company_id = $1
ORDER BY agency, portal;

-- name: GetGovPortalAccount :one
SELECT * FROM gov_portal_accounts WHERE id = $1 AND company_id = $2;

-- name: FindGovPortalAccount :one
SELECT * FROM gov_portal_accounts
WHERE company_id = $1 AND agency = $2 AND portal = $3 AND status <> 'disabled';

-- name: DeleteGovPortalAccount :exec
DELETE FROM gov_portal_accounts WHERE id = $1 AND company_id = $2;

-- name: MarkGovPortalAccountUsed :exec
UPDATE gov_portal_accounts SET
    last_used_at = NOW(),
    last_error = sqlc.narg(last_error),
    status = CASE WHEN sqlc.narg(last_error)::text IS NULL THEN 'active' ELSE 'needs_attention' END,
    updated_at = NOW()
WHERE id = $1;

-- name: CreateGovSubmission :one
INSERT INTO gov_submissions (
    company_id, agency, form_code, period, tax_filing_id, channel, playbook_id,
    portal_account_id, status, file_name, file_path, file_sha256, data, preflight,
    instructions, due_date, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
RETURNING *;

-- name: GetGovSubmission :one
SELECT * FROM gov_submissions WHERE id = $1 AND company_id = $2;

-- name: GetGovSubmissionByID :one
SELECT * FROM gov_submissions WHERE id = $1;

-- name: ListGovSubmissions :many
SELECT * FROM gov_submissions
WHERE company_id = $1
  AND (sqlc.arg(status)::text = '' OR status = sqlc.arg(status)::text)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountGovSubmissionsByStatus :many
SELECT status, COUNT(*)::bigint AS n FROM gov_submissions
WHERE company_id = $1 GROUP BY status;

-- name: ApproveGovSubmission :one
UPDATE gov_submissions SET
    status = 'approved', approved_by = $3, approved_at = NOW(), error_message = NULL, updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status IN ('ready', 'failed')
RETURNING *;

-- name: CancelGovSubmission :one
UPDATE gov_submissions SET status = 'cancelled', updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status IN ('draft', 'ready', 'approved', 'failed', 'needs_human')
RETURNING *;

-- name: ClaimApprovedGovSubmissions :many
-- Worker picks up approved automated submissions (FOR UPDATE SKIP LOCKED avoids double runs).
UPDATE gov_submissions SET status = 'running', attempts = attempts + 1, updated_at = NOW()
WHERE id IN (
    SELECT id FROM gov_submissions
    WHERE status = 'approved' AND channel IN ('api', 'email', 'browser')
    ORDER BY approved_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: SetGovSubmissionStatus :exec
-- Never resurrects a submission the user cancelled meanwhile.
UPDATE gov_submissions SET status = $2, error_message = sqlc.narg(error_message), updated_at = NOW()
WHERE id = $1 AND status <> 'cancelled';

-- name: MarkGovSubmissionSubmitted :one
-- Records what actually happened at the agency, even if the user pressed
-- cancel after the final submit step already went through.
UPDATE gov_submissions SET
    status = 'submitted', submitted_at = NOW(), reference_no = $2, outputs = $3,
    error_message = NULL, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: RecoverStuckGovSubmissions :exec
-- A worker crash mid-run leaves rows in running/needs_human; fail them so a human can retry.
UPDATE gov_submissions SET status = 'failed', error_message = 'interrupted: worker stopped during submission; verify on the portal before retrying', updated_at = NOW()
WHERE status IN ('running', 'needs_human') AND updated_at < NOW() - INTERVAL '45 minutes';

-- name: InsertGovSubmissionEvent :exec
INSERT INTO gov_submission_events (submission_id, kind, message, data, actor_user_id)
VALUES ($1, $2, $3, $4, $5);

-- name: ListGovSubmissionEvents :many
SELECT * FROM gov_submission_events WHERE submission_id = $1 ORDER BY id;

-- name: CreateGovHumanRequest :one
INSERT INTO gov_human_requests (submission_id, company_id, name, prompt, screenshot_path, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetGovHumanRequest :one
SELECT * FROM gov_human_requests WHERE id = $1;

-- name: ListOpenGovHumanRequests :many
SELECT r.id, r.submission_id, r.name, r.prompt, r.screenshot_path, r.expires_at, r.created_at,
       s.agency, s.form_code, s.period
FROM gov_human_requests r JOIN gov_submissions s ON s.id = r.submission_id
WHERE r.company_id = $1 AND r.status = 'open' AND r.expires_at > NOW()
  AND s.status IN ('running', 'needs_human')
ORDER BY r.created_at;

-- name: AnswerGovHumanRequest :one
UPDATE gov_human_requests SET status = 'answered', answer_enc = $3, answered_by = $4, answered_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'open' AND expires_at > NOW()
RETURNING *;

-- name: ConsumeGovHumanRequest :one
-- Reads and wipes the answer so OTPs are not kept at rest.
UPDATE gov_human_requests SET status = 'consumed', answer_enc = NULL
WHERE id = $1 AND status = 'answered'
RETURNING id, name;

-- name: GetGovHumanAnswer :one
SELECT answer_enc FROM gov_human_requests WHERE id = $1 AND status = 'answered';

-- name: ExpireGovHumanRequest :exec
UPDATE gov_human_requests SET status = 'expired', answer_enc = NULL WHERE id = $1 AND status IN ('open', 'answered');

-- name: MarkTaxFilingFiled :exec
UPDATE tax_filings SET
    status = 'filed', filing_date = CURRENT_DATE, reference_no = $3, proof_path = $4, filed_by = $5, updated_at = NOW()
WHERE id = $1 AND company_id = $2;

-- name: ListFilingsToPrepare :many
-- Upcoming returns (all companies) without a live submission, for autopilot.
SELECT tf.* FROM tax_filings tf
WHERE tf.status IN ('pending', 'generated', 'overdue')
  AND tf.due_date <= CURRENT_DATE + sqlc.arg(within_days)::int
  -- older backlog is surfaced by overdue alerts; HR prepares those on demand
  AND tf.due_date >= CURRENT_DATE - 45
  AND NOT EXISTS (
      SELECT 1 FROM gov_submissions gs
      WHERE gs.tax_filing_id = tf.id AND gs.status NOT IN ('failed', 'cancelled')
  )
ORDER BY tf.due_date
LIMIT 200;

-- name: UpsertGovPlaybook :one
INSERT INTO gov_playbooks (company_id, playbook_key, form_code, definition, created_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (COALESCE(company_id, 0), playbook_key) DO UPDATE SET
    form_code = EXCLUDED.form_code,
    definition = EXCLUDED.definition,
    verified = false, verified_by = NULL, verified_at = NULL,
    updated_at = NOW()
RETURNING *;

-- name: ListGovPlaybooks :many
SELECT * FROM gov_playbooks WHERE company_id = $1 ORDER BY playbook_key;

-- name: GetGovPlaybookForForm :one
SELECT * FROM gov_playbooks WHERE company_id = $1 AND form_code = $2
ORDER BY verified DESC, updated_at DESC LIMIT 1;

-- name: GetGovPlaybook :one
SELECT * FROM gov_playbooks WHERE id = $1 AND company_id = $2;

-- name: VerifyGovPlaybook :one
UPDATE gov_playbooks SET verified = $3, verified_by = $4, verified_at = CASE WHEN $3 THEN NOW() ELSE NULL END, updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: DeleteGovPlaybook :exec
DELETE FROM gov_playbooks WHERE id = $1 AND company_id = $2;

-- name: SetGovSubmissionDryRun :exec
UPDATE gov_submissions SET dry_run = $2 WHERE id = $1;

-- name: CancelDraftGovSubmissions :exec
UPDATE gov_submissions SET status = 'cancelled', updated_at = NOW()
WHERE company_id = $1 AND agency = $2 AND form_code = $3 AND period = $4 AND status = 'draft';

-- name: ListCompaniesForAutopilot :many
SELECT id, country FROM companies WHERE country IN ('PHL', 'USA') ORDER BY id;

-- name: ListUnregisteredWorkStates :many
-- US states where active employees live but the company has no state withholding registration yet.
SELECT DISTINCT UPPER(ep.state_of_residence)::text AS state
FROM employee_profiles ep
JOIN employees e ON e.id = ep.employee_id
WHERE e.company_id = $1 AND e.status IN ('active', 'probationary')
  AND ep.state_of_residence IS NOT NULL AND ep.state_of_residence <> ''
  AND NOT EXISTS (
      SELECT 1 FROM company_registration_numbers r
      WHERE r.company_id = e.company_id AND r.registration_type = 'state_withholding_' || UPPER(ep.state_of_residence)
  )
ORDER BY 1;
