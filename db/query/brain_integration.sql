-- name: CreateBrainLink :one
INSERT INTO brain_links (
    company_id, brain_tenant_id, api_endpoint, api_key_enc, webhook_secret
) VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetActiveBrainLink :one
SELECT * FROM brain_links
WHERE company_id = $1 AND is_active = true
LIMIT 1;

-- name: GetBrainLinkByID :one
SELECT * FROM brain_links
WHERE id = $1 AND company_id = $2;

-- name: UpdateBrainLinkSyncedAt :exec
UPDATE brain_links
SET last_synced_at = NOW(), updated_at = NOW()
WHERE id = $1;

-- name: UpdateBrainLinkStatus :exec
UPDATE brain_links
SET is_active = $2, updated_at = NOW()
WHERE id = $1;

-- name: DeleteBrainLink :exec
DELETE FROM brain_links WHERE id = $1 AND company_id = $2;

-- name: ListBrainLinks :many
SELECT * FROM brain_links
WHERE company_id = $1
ORDER BY created_at DESC;

-- name: InsertBrainOutbox :one
INSERT INTO brain_outbox (
    company_id, event_type, aggregate_type, aggregate_id,
    payload, idempotency_key
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListPendingBrainOutbox :many
SELECT * FROM brain_outbox
WHERE status IN ('pending', 'failed')
  AND (next_retry_at IS NULL OR next_retry_at <= NOW())
ORDER BY created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkBrainOutboxSent :exec
UPDATE brain_outbox
SET status = 'sent', sent_at = NOW(), error_message = NULL
WHERE id = $1;

-- name: MarkBrainOutboxFailed :exec
UPDATE brain_outbox
SET status = CASE WHEN retry_count + 1 >= max_retries THEN 'dead' ELSE 'failed' END,
    retry_count = retry_count + 1,
    next_retry_at = NOW() + (INTERVAL '1 minute' * POWER(2, retry_count)),
    error_message = $2
WHERE id = $1;
