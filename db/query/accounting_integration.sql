-- name: CreateAccountingLink :one
INSERT INTO accounting_links (
    company_id, provider, remote_company_id, api_endpoint,
    api_key_enc, webhook_secret, jurisdiction, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetAccountingLink :one
SELECT * FROM accounting_links
WHERE company_id = $1 AND provider = $2;

-- name: GetAccountingLinkByID :one
SELECT * FROM accounting_links
WHERE id = $1 AND company_id = $2;

-- name: UpdateAccountingLinkStatus :exec
UPDATE accounting_links
SET status = $3, updated_at = NOW()
WHERE id = $1 AND company_id = $2;

-- name: UpdateAccountingLinkSyncedAt :exec
UPDATE accounting_links
SET last_synced_at = NOW(), updated_at = NOW()
WHERE id = $1;

-- name: DeleteAccountingLink :exec
DELETE FROM accounting_links
WHERE id = $1 AND company_id = $2;

-- name: ListAccountingLinks :many
SELECT * FROM accounting_links
WHERE company_id = $1
ORDER BY created_at DESC;

-- name: GetActiveAccountingLink :one
SELECT * FROM accounting_links
WHERE company_id = $1 AND status = 'active'
LIMIT 1;

-- name: InsertAccountingOutbox :one
INSERT INTO accounting_outbox (
    company_id, event_type, aggregate_type, aggregate_id,
    payload, idempotency_key
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListPendingOutboxEvents :many
SELECT * FROM accounting_outbox
WHERE status IN ('pending', 'failed')
  AND (next_retry_at IS NULL OR next_retry_at <= NOW())
ORDER BY created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxSent :exec
UPDATE accounting_outbox
SET status = 'sent', sent_at = NOW(), error_message = NULL
WHERE id = $1;

-- name: MarkOutboxFailed :exec
UPDATE accounting_outbox
SET status = CASE WHEN retry_count + 1 >= max_retries THEN 'dead' ELSE 'failed' END,
    retry_count = retry_count + 1,
    next_retry_at = NOW() + (INTERVAL '1 minute' * POWER(2, retry_count)),
    error_message = $2
WHERE id = $1;

-- name: ListOutboxByCompany :many
SELECT * FROM accounting_outbox
WHERE company_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountOutboxByStatus :many
SELECT status, COUNT(*) as count
FROM accounting_outbox
WHERE company_id = $1
GROUP BY status;
