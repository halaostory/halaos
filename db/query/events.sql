-- name: InsertHREvent :one
INSERT INTO hr_events (
    company_id, aggregate_type, aggregate_id, event_type,
    event_version, payload, actor_user_id, idempotency_key
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetPendingEvents :many
SELECT * FROM hr_events
WHERE status IN ('pending', 'failed')
  AND retries < 5
ORDER BY created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkEventProcessed :exec
UPDATE hr_events SET
    status = 'processed',
    processed_at = NOW()
WHERE id = $1;

-- name: MarkEventFailed :exec
UPDATE hr_events SET
    status = CASE WHEN retries + 1 >= 5 THEN 'dead' ELSE 'failed' END,
    retries = retries + 1,
    error_message = $2
WHERE id = $1;

-- name: ListEventsByAggregate :many
SELECT * FROM hr_events
WHERE aggregate_type = $1 AND aggregate_id = $2
ORDER BY occurred_at DESC
LIMIT $3;

-- name: InsertAIAuditLog :one
INSERT INTO ai_audit_log (
    company_id, user_id, request_id, session_id, intent, model,
    prompt_hash, tool_calls, decision, risk_level,
    input_tokens, output_tokens, latency_ms,
    redacted_input, redacted_output
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING *;

-- name: ListAIAuditLogs :many
SELECT * FROM ai_audit_log
WHERE company_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: InsertAuditLog :exec
INSERT INTO audit_logs (
    company_id, user_id, action, entity_type, entity_id,
    old_values, new_values, ip_address, user_agent
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: ListPendingApprovals :many
SELECT * FROM approval_workflows
WHERE approver_id = $1 AND status = 'pending'
ORDER BY created_at ASC;

-- name: ApproveWorkflow :exec
UPDATE approval_workflows SET
    status = 'approved',
    comments = $3,
    decided_at = NOW()
WHERE id = $1 AND approver_id = $2 AND status = 'pending';

-- name: RejectWorkflow :exec
UPDATE approval_workflows SET
    status = 'rejected',
    comments = $3,
    decided_at = NOW()
WHERE id = $1 AND approver_id = $2 AND status = 'pending';

-- name: GetApprovalWorkflowEntity :one
SELECT entity_type, entity_id FROM approval_workflows WHERE id = $1;

-- name: InsertApprovalWorkflow :one
INSERT INTO approval_workflows (company_id, entity_type, entity_id, step, approver_id, status)
VALUES ($1, $2, $3, 1, $4, 'pending')
RETURNING *;

-- name: GetFirstAdminEmployeeID :one
SELECT e.id FROM employees e
JOIN users u ON u.id = e.user_id
WHERE e.company_id = $1 AND u.role IN ('super_admin', 'admin') AND e.status = 'active'
ORDER BY u.role ASC, e.id ASC
LIMIT 1;
