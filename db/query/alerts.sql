-- Alert channels

-- name: GetAlertChannel :one
SELECT * FROM alert_channels WHERE company_id = $1;

-- name: UpsertAlertChannel :one
INSERT INTO alert_channels (company_id, slack_webhook_enc, slack_hint, events, enabled, updated_by)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (company_id) DO UPDATE SET
    slack_webhook_enc = COALESCE(EXCLUDED.slack_webhook_enc, alert_channels.slack_webhook_enc),
    slack_hint = CASE WHEN EXCLUDED.slack_webhook_enc IS NULL THEN alert_channels.slack_hint ELSE EXCLUDED.slack_hint END,
    events = EXCLUDED.events, enabled = EXCLUDED.enabled, updated_by = EXCLUDED.updated_by, updated_at = now()
RETURNING *;

-- name: ClearAlertSlack :exec
UPDATE alert_channels SET slack_webhook_enc = NULL, slack_hint = '', updated_at = now() WHERE company_id = $1;

-- name: ListCompaniesDueForDigest :many
SELECT company_id FROM alert_channels
WHERE enabled AND slack_webhook_enc IS NOT NULL AND 'filing_due' = ANY(events)
  AND (last_digest_on IS NULL OR last_digest_on < CURRENT_DATE);

-- name: MarkDigestSent :exec
UPDATE alert_channels SET last_digest_on = CURRENT_DATE WHERE company_id = $1;

-- name: ListFilingsDueSoon :many
SELECT filing_type, period_year, period_month, period_quarter, due_date FROM tax_filings
WHERE company_id = $1 AND status IN ('pending', 'generated') AND due_date BETWEEN CURRENT_DATE AND CURRENT_DATE + 3
ORDER BY due_date;
