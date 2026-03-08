-- name: InsertAIReminder :one
INSERT INTO ai_reminders (company_id, user_id, reminder_type, entity_type, entity_id, scheduled_date, sent_at, notification_id)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), $7)
RETURNING *;

-- name: HasReminderBeenSent :one
SELECT EXISTS(
    SELECT 1 FROM ai_reminders
    WHERE company_id = $1
      AND reminder_type = $2
      AND COALESCE(entity_type, '') = COALESCE($3, '')
      AND COALESCE(entity_id, 0) = COALESCE($4::bigint, 0)
      AND scheduled_date = $5
      AND sent_at IS NOT NULL
) AS sent;

-- name: ListPendingLeaveApprovals :many
SELECT lr.id, lr.company_id, lr.employee_id, lr.leave_type_id, lr.start_date, lr.end_date, lr.days,
       e.first_name || ' ' || e.last_name as employee_name,
       lt.name as leave_type_name
FROM leave_requests lr
JOIN employees e ON e.id = lr.employee_id
JOIN leave_types lt ON lt.id = lr.leave_type_id
WHERE lr.company_id = $1 AND lr.status = 'pending'
ORDER BY lr.created_at ASC;

-- name: ListPendingOvertimeApprovals :many
SELECT ot.id, ot.company_id, ot.employee_id, ot.ot_date, ot.hours, ot.ot_type,
       e.first_name || ' ' || e.last_name as employee_name
FROM overtime_requests ot
JOIN employees e ON e.id = ot.employee_id
WHERE ot.company_id = $1 AND ot.status = 'pending'
ORDER BY ot.created_at ASC;

-- name: ListAdminUsersByCompany :many
SELECT u.id, u.company_id
FROM users u
WHERE u.company_id = $1 AND u.role IN ('admin', 'owner') AND u.status = 'active';

-- name: ListManagerUsersByCompany :many
SELECT DISTINCT u.id, u.company_id
FROM users u
JOIN employees e ON e.user_id = u.id AND e.company_id = u.company_id
WHERE u.company_id = $1 AND u.role IN ('admin', 'owner', 'manager') AND u.status = 'active';
