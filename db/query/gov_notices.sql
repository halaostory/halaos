-- Government notices

-- name: CreateGovNotice :one
INSERT INTO gov_notices (company_id, agency, notice_type, form_code, period, reference_no, amount, respond_by,
    title, summary, recommendation, resolution_path, draft_response, matched_submission_id, status,
    auto_resolvable, triaged_by, source_text, file_name, file_path, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
RETURNING *;

-- name: UpdateGovNoticeTriage :one
UPDATE gov_notices SET agency = $3, notice_type = $4, form_code = $5, period = $6, reference_no = $7,
    amount = $8, respond_by = $9, title = $10, summary = $11, recommendation = $12, resolution_path = $13,
    draft_response = $14, matched_submission_id = $15, status = $16, auto_resolvable = $17, triaged_by = $18,
    updated_at = now()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: GetGovNotice :one
SELECT * FROM gov_notices WHERE id = $1 AND company_id = $2;

-- name: ListGovNotices :many
SELECT * FROM gov_notices
WHERE company_id = $1 AND (sqlc.arg(status)::varchar = '' OR status = sqlc.arg(status)::varchar)
ORDER BY (status = 'resolved'), respond_by NULLS LAST, created_at DESC
LIMIT 200;

-- name: SetGovNoticeStatus :one
UPDATE gov_notices SET status = sqlc.arg(status)::varchar,
    draft_response = COALESCE(sqlc.narg(draft_response)::text, draft_response),
    resolution_note = COALESCE(sqlc.narg(resolution_note)::text, resolution_note),
    resolved_by = CASE WHEN sqlc.arg(status)::varchar = 'resolved' THEN sqlc.narg(user_id)::bigint ELSE resolved_by END,
    resolved_at = CASE WHEN sqlc.arg(status)::varchar = 'resolved' THEN now() ELSE NULL END,
    updated_at = now()
WHERE id = sqlc.arg(id) AND company_id = sqlc.arg(company_id)
RETURNING *;

-- name: CountGovNoticesByStatus :many
SELECT status, COUNT(*)::int AS n FROM gov_notices WHERE company_id = $1 GROUP BY status;

-- name: FindSubmissionForNotice :one
-- The most recent submission of a form for a period (to prove it was filed).
SELECT * FROM gov_submissions
WHERE company_id = $1 AND form_code = $2 AND period = $3 AND dry_run = false AND status <> 'cancelled'
ORDER BY (status = 'submitted') DESC, created_at DESC
LIMIT 1;
