-- name: CreateClearanceRequest :one
INSERT INTO clearance_requests (
    company_id, employee_id, resignation_date, last_working_day,
    reason, submitted_by
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetClearanceRequest :one
SELECT cr.*,
       e.first_name || ' ' || e.last_name as employee_name,
       e.employee_no,
       COALESCE(d.name, '') as department_name,
       COALESCE(p.title, '') as position_title
FROM clearance_requests cr
JOIN employees e ON e.id = cr.employee_id
LEFT JOIN departments d ON d.id = e.department_id
LEFT JOIN positions p ON p.id = e.position_id
WHERE cr.id = $1 AND cr.company_id = $2;

-- name: ListClearanceRequests :many
SELECT cr.id, cr.employee_id, cr.resignation_date, cr.last_working_day,
       cr.reason, cr.status, cr.created_at,
       e.first_name || ' ' || e.last_name as employee_name,
       e.employee_no,
       COALESCE(d.name, '') as department_name
FROM clearance_requests cr
JOIN employees e ON e.id = cr.employee_id
LEFT JOIN departments d ON d.id = e.department_id
WHERE cr.company_id = $1
  AND ($2::varchar IS NULL OR cr.status = $2)
ORDER BY cr.created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountClearanceRequests :one
SELECT COUNT(*) FROM clearance_requests
WHERE company_id = $1
  AND ($2::varchar IS NULL OR status = $2);

-- name: UpdateClearanceStatus :one
UPDATE clearance_requests SET
    status = $3,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: CreateClearanceItem :one
INSERT INTO clearance_items (
    clearance_id, department, item_name
) VALUES ($1, $2, $3)
RETURNING *;

-- name: ListClearanceItems :many
SELECT ci.*,
       COALESCE(u.email, '') as cleared_by_email
FROM clearance_items ci
LEFT JOIN users u ON u.id = ci.cleared_by
WHERE ci.clearance_id = $1
ORDER BY ci.department, ci.id;

-- name: UpdateClearanceItem :one
UPDATE clearance_items SET
    status = $2,
    cleared_by = $3,
    cleared_at = NOW(),
    remarks = $4
WHERE id = $1
RETURNING *;

-- name: ListClearanceTemplates :many
SELECT * FROM clearance_templates
WHERE company_id = $1 AND is_active = true
ORDER BY department, sort_order;

-- name: CreateClearanceTemplate :one
INSERT INTO clearance_templates (
    company_id, department, item_name, sort_order
) VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: DeleteClearanceTemplate :exec
DELETE FROM clearance_templates WHERE id = $1 AND company_id = $2;

-- name: CountClearanceItemsByStatus :many
SELECT status, COUNT(*) as count
FROM clearance_items
WHERE clearance_id = $1
GROUP BY status;
