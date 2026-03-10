-- name: UpsertSLAConfig :one
INSERT INTO approval_sla_configs (
    company_id, entity_type, reminder_after_hours, second_reminder_hours,
    escalate_after_hours, auto_action_hours, auto_action, escalation_role
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (company_id, entity_type) DO UPDATE SET
    reminder_after_hours = EXCLUDED.reminder_after_hours,
    second_reminder_hours = EXCLUDED.second_reminder_hours,
    escalate_after_hours = EXCLUDED.escalate_after_hours,
    auto_action_hours = EXCLUDED.auto_action_hours,
    auto_action = EXCLUDED.auto_action,
    escalation_role = EXCLUDED.escalation_role,
    is_active = EXCLUDED.is_active,
    updated_at = NOW()
RETURNING *;

-- name: GetSLAConfig :one
SELECT * FROM approval_sla_configs
WHERE company_id = $1 AND entity_type = $2;

-- name: ListSLAConfigs :many
SELECT * FROM approval_sla_configs
WHERE company_id = $1
ORDER BY entity_type;

-- name: InsertSLAEvent :one
INSERT INTO approval_sla_events (company_id, entity_type, entity_id, event_type, target_user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: HasSLAEventBeenSent :one
SELECT EXISTS(
    SELECT 1 FROM approval_sla_events
    WHERE company_id = $1 AND entity_type = $2 AND entity_id = $3 AND event_type = $4
) as sent;

-- name: GetSLAEventsForEntity :many
SELECT * FROM approval_sla_events
WHERE entity_type = $1 AND entity_id = $2
ORDER BY created_at DESC;

-- name: ListPendingLeaveRequestsWithAge :many
SELECT lr.id, lr.company_id, lr.employee_id, lr.status, lr.created_at,
       e.first_name || ' ' || e.last_name as employee_name,
       lt.name as leave_type_name,
       EXTRACT(EPOCH FROM (NOW() - lr.created_at)) / 3600 as age_hours
FROM leave_requests lr
JOIN employees e ON e.id = lr.employee_id
JOIN leave_types lt ON lt.id = lr.leave_type_id
WHERE lr.company_id = $1 AND lr.status = 'pending'
ORDER BY lr.created_at ASC;

-- name: ListPendingOTRequestsWithAge :many
SELECT otr.id, otr.company_id, otr.employee_id, otr.status, otr.created_at,
       e.first_name || ' ' || e.last_name as employee_name,
       otr.ot_type,
       EXTRACT(EPOCH FROM (NOW() - otr.created_at)) / 3600 as age_hours
FROM overtime_requests otr
JOIN employees e ON e.id = otr.employee_id
WHERE otr.company_id = $1 AND otr.status = 'pending'
ORDER BY otr.created_at ASC;
