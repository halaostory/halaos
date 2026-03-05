-- name: CreateOvertimeRequest :one
INSERT INTO overtime_requests (
    company_id, employee_id, ot_date, start_at, end_at,
    hours, ot_type, reason
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetOvertimeRequest :one
SELECT * FROM overtime_requests WHERE id = $1 AND company_id = $2;

-- name: ListOvertimeRequests :many
SELECT ot.*, e.first_name || ' ' || e.last_name as employee_name
FROM overtime_requests ot
JOIN employees e ON e.id = ot.employee_id
WHERE ot.company_id = $1
  AND ($2::bigint IS NULL OR ot.employee_id = $2)
  AND ($3::varchar IS NULL OR ot.status = $3)
ORDER BY ot.created_at DESC
LIMIT $4 OFFSET $5;

-- name: CountOvertimeRequests :one
SELECT COUNT(*) FROM overtime_requests
WHERE company_id = $1
  AND ($2::bigint IS NULL OR employee_id = $2)
  AND ($3::varchar IS NULL OR status = $3);

-- name: ApproveOvertimeRequest :one
UPDATE overtime_requests SET
    status = 'approved',
    approver_id = $3,
    approved_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'pending'
RETURNING *;

-- name: RejectOvertimeRequest :one
UPDATE overtime_requests SET
    status = 'rejected',
    approver_id = $3,
    rejection_reason = $4,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'pending'
RETURNING *;

-- name: GetApprovedOTHours :many
SELECT employee_id, SUM(hours) as total_hours
FROM overtime_requests
WHERE company_id = $1
  AND status = 'approved'
  AND ot_date >= $2
  AND ot_date <= $3
GROUP BY employee_id;
