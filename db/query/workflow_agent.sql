-- name: CreateWorkflowTrigger :one
INSERT INTO workflow_triggers (company_id, name, description, trigger_type, entity_type, trigger_config, action_type, action_config, priority, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateWorkflowTrigger :one
UPDATE workflow_triggers SET
    name = $3,
    description = $4,
    trigger_type = $5,
    entity_type = $6,
    trigger_config = $7,
    action_type = $8,
    action_config = $9,
    priority = $10,
    is_active = $11,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: DeactivateWorkflowTrigger :exec
UPDATE workflow_triggers SET is_active = false, updated_at = NOW()
WHERE id = $1 AND company_id = $2;

-- name: ListWorkflowTriggers :many
SELECT * FROM workflow_triggers
WHERE company_id = $1
ORDER BY priority ASC, created_at DESC;

-- name: ListActiveTriggersForEvent :many
SELECT * FROM workflow_triggers
WHERE company_id = $1
  AND is_active = true
  AND trigger_type = $2
  AND (entity_type = $3 OR entity_type = '*')
ORDER BY priority ASC;

-- name: InsertWorkflowDecision :one
INSERT INTO workflow_decisions (
    company_id, trigger_id, entity_type, entity_id,
    decision, confidence, reasoning, context_snapshot,
    ai_agent_slug, tokens_used
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: MarkDecisionExecuted :exec
UPDATE workflow_decisions SET
    executed = true,
    executed_at = NOW(),
    execution_result = $2
WHERE id = $1;

-- name: GetWorkflowDecision :one
SELECT * FROM workflow_decisions WHERE id = $1;

-- name: GetDecisionForEntity :one
SELECT * FROM workflow_decisions
WHERE entity_type = $1 AND entity_id = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: HasDecisionForEntity :one
SELECT EXISTS(
    SELECT 1 FROM workflow_decisions
    WHERE entity_type = $1 AND entity_id = $2
) AS has_decision;

-- name: ListWorkflowDecisions :many
SELECT wd.*, wt.name as trigger_name
FROM workflow_decisions wd
LEFT JOIN workflow_triggers wt ON wt.id = wd.trigger_id
WHERE wd.company_id = $1
  AND ($2::varchar = '' OR wd.entity_type = $2)
  AND ($3::bigint IS NULL OR $3 = 0 OR wd.entity_id = $3)
  AND ($4::boolean IS NULL OR $4 = false OR wd.overridden_at IS NOT NULL)
ORDER BY wd.created_at DESC
LIMIT $5 OFFSET $6;

-- name: CountWorkflowDecisions :one
SELECT COUNT(*) FROM workflow_decisions
WHERE company_id = $1
  AND ($2::varchar = '' OR entity_type = $2)
  AND ($3::bigint IS NULL OR $3 = 0 OR entity_id = $3)
  AND ($4::boolean IS NULL OR $4 = false OR overridden_at IS NOT NULL);

-- name: RecordDecisionOverride :exec
UPDATE workflow_decisions SET
    overridden_by = $2,
    override_action = $3,
    override_reason = $4,
    overridden_at = NOW()
WHERE id = $1 AND overridden_at IS NULL;

-- name: GetRecentApprovalPatterns :many
SELECT decision, confidence,
       CASE WHEN overridden_at IS NOT NULL THEN true ELSE false END as was_overridden,
       override_action
FROM workflow_decisions
WHERE company_id = $1 AND entity_type = $2
ORDER BY created_at DESC
LIMIT 50;

-- name: CountDecisionsByOutcome :many
SELECT decision, COUNT(*) as count
FROM workflow_decisions
WHERE company_id = $1
  AND created_at > NOW() - INTERVAL '30 days'
GROUP BY decision;

-- name: GetDecisionAccuracyRate :many
SELECT
    CASE
        WHEN confidence::numeric >= 0.90 THEN 'high'
        WHEN confidence::numeric >= 0.70 THEN 'medium'
        ELSE 'low'
    END as confidence_tier,
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE overridden_at IS NOT NULL) as overridden
FROM workflow_decisions
WHERE company_id = $1
  AND created_at > NOW() - INTERVAL '30 days'
GROUP BY confidence_tier;

-- name: GetDecisionVolumeByDay :many
SELECT
    DATE(created_at) as day,
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE executed = true) as auto_executed,
    COUNT(*) FILTER (WHERE decision LIKE 'recommend_%') as recommended,
    COUNT(*) FILTER (WHERE decision = 'escalate') as escalated
FROM workflow_decisions
WHERE company_id = $1
  AND created_at > NOW() - INTERVAL '30 days'
GROUP BY DATE(created_at)
ORDER BY day DESC;

-- name: GetAgentDecisionStats :one
SELECT
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE executed = true) as auto_executed,
    COUNT(*) FILTER (WHERE decision LIKE 'recommend_%') as recommended,
    COUNT(*) FILTER (WHERE decision = 'escalate') as escalated,
    COUNT(*) FILTER (WHERE overridden_at IS NOT NULL) as overridden,
    COALESCE(AVG(confidence::numeric), 0) as avg_confidence
FROM workflow_decisions
WHERE company_id = $1
  AND created_at > NOW() - INTERVAL '30 days';
