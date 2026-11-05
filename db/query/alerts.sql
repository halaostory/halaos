-- Alert channels

-- name: GetAlertChannel :one
SELECT * FROM alert_channels WHERE company_id = $1;

-- name: UpsertAlertChannel :one
-- A NULL secret keeps the stored one (and its hint); kind and target are
-- always written (the handler passes the current ones when unchanged).
INSERT INTO alert_channels (company_id, kind, secret_enc, hint, target, events, enabled, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (company_id) DO UPDATE SET
    kind = EXCLUDED.kind,
    secret_enc = COALESCE(EXCLUDED.secret_enc, alert_channels.secret_enc),
    hint = CASE WHEN EXCLUDED.secret_enc IS NULL THEN alert_channels.hint ELSE EXCLUDED.hint END,
    target = EXCLUDED.target,
    events = EXCLUDED.events, enabled = EXCLUDED.enabled, updated_by = EXCLUDED.updated_by, updated_at = now()
RETURNING *;

-- name: ClearAlertChannel :exec
UPDATE alert_channels SET secret_enc = NULL, hint = '', target = '', updated_at = now() WHERE company_id = $1;

-- name: ListCompaniesDueForDigest :many
SELECT company_id FROM alert_channels
WHERE enabled AND secret_enc IS NOT NULL AND 'filing_due' = ANY(events)
  AND (last_digest_on IS NULL OR last_digest_on < CURRENT_DATE);

-- name: MarkDigestSent :exec
UPDATE alert_channels SET last_digest_on = CURRENT_DATE WHERE company_id = $1;

-- name: ListFilingsDueSoon :many
SELECT filing_type, period_year, period_month, period_quarter, due_date FROM tax_filings
WHERE company_id = $1 AND status IN ('pending', 'generated') AND due_date BETWEEN CURRENT_DATE AND CURRENT_DATE + 3
ORDER BY due_date;
