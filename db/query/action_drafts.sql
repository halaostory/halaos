-- name: CreateActionDraft :one
INSERT INTO action_drafts (company_id, user_id, session_id, tool_name, tool_input, risk_level, description, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() + INTERVAL '10 minutes')
RETURNING *;

-- name: GetActionDraft :one
SELECT * FROM action_drafts
WHERE id = $1 AND company_id = $2 AND user_id = $3;

-- name: ConfirmActionDraft :one
UPDATE action_drafts SET status = 'confirmed', updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND user_id = $3 AND status = 'pending' AND expires_at > NOW()
RETURNING *;

-- name: ExecuteActionDraft :exec
UPDATE action_drafts SET status = 'executed', result = $2, updated_at = NOW()
WHERE id = $1;

-- name: FailActionDraft :exec
UPDATE action_drafts SET status = 'executed', error_message = $2, updated_at = NOW()
WHERE id = $1;

-- name: RejectActionDraft :exec
UPDATE action_drafts SET status = 'rejected', updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND user_id = $3 AND status = 'pending';

-- name: ExpirePendingDrafts :exec
UPDATE action_drafts SET status = 'expired', updated_at = NOW()
WHERE status = 'pending' AND expires_at < NOW();

-- name: ListPendingDrafts :many
SELECT * FROM action_drafts
WHERE company_id = $1 AND user_id = $2 AND status = 'pending' AND expires_at > NOW()
ORDER BY created_at DESC;
