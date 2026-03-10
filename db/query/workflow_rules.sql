-- name: CreateWorkflowRule :one
INSERT INTO workflow_rules (company_id, name, description, entity_type, rule_type, conditions, priority, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateWorkflowRule :one
UPDATE workflow_rules SET
    name = $3,
    description = $4,
    entity_type = $5,
    rule_type = $6,
    conditions = $7,
    priority = $8,
    is_active = $9,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: DeactivateWorkflowRule :exec
UPDATE workflow_rules SET is_active = false, updated_at = NOW()
WHERE id = $1 AND company_id = $2;

-- name: GetWorkflowRule :one
SELECT * FROM workflow_rules WHERE id = $1 AND company_id = $2;

-- name: ListWorkflowRules :many
SELECT * FROM workflow_rules
WHERE company_id = $1
ORDER BY priority ASC, created_at DESC;

-- name: ListActiveRulesForEntityType :many
SELECT * FROM workflow_rules
WHERE company_id = $1 AND entity_type = $2 AND is_active = true
ORDER BY priority ASC;

-- name: InsertRuleExecution :one
INSERT INTO workflow_rule_executions (company_id, rule_id, entity_type, entity_id, action, reason, evaluated_conditions)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListRuleExecutions :many
SELECT wre.*, wr.name as rule_name
FROM workflow_rule_executions wre
JOIN workflow_rules wr ON wr.id = wre.rule_id
WHERE wre.company_id = $1
  AND ($2::bigint IS NULL OR $2 = 0 OR wre.rule_id = $2)
ORDER BY wre.created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountRuleExecutions :one
SELECT COUNT(*) FROM workflow_rule_executions
WHERE company_id = $1
  AND ($2::bigint IS NULL OR $2 = 0 OR rule_id = $2);

-- name: CountAutoApprovedByCompany :one
SELECT COUNT(*) FROM workflow_rule_executions
WHERE company_id = $1 AND action = 'auto_approved';

-- name: ListPendingLeaveRequestsForAutoApproval :many
SELECT lr.*, lt.code as leave_type_code, lt.name as leave_type_name,
       e.first_name, e.last_name, e.hire_date, e.department_id
FROM leave_requests lr
JOIN leave_types lt ON lt.id = lr.leave_type_id
JOIN employees e ON e.id = lr.employee_id
WHERE lr.company_id = $1
  AND lr.status = 'pending'
  AND lr.created_at < NOW() - INTERVAL '5 minutes'
ORDER BY lr.created_at ASC;

-- name: ListPendingOTRequestsForAutoApproval :many
SELECT otr.*, e.first_name, e.last_name, e.department_id
FROM overtime_requests otr
JOIN employees e ON e.id = otr.employee_id
WHERE otr.company_id = $1
  AND otr.status = 'pending'
  AND otr.created_at < NOW() - INTERVAL '5 minutes'
ORDER BY otr.created_at ASC;
