-- name: CreateLeaveType :one
INSERT INTO leave_types (
    company_id, code, name, is_paid, default_days,
    is_convertible, requires_attachment, min_days_notice,
    accrual_type, gender_specific, is_statutory
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListLeaveTypes :many
SELECT * FROM leave_types WHERE company_id = $1 AND is_active = true ORDER BY name;

-- name: GetLeaveBalance :one
SELECT * FROM leave_balances
WHERE company_id = $1 AND employee_id = $2 AND leave_type_id = $3 AND year = $4;

-- name: ListLeaveBalances :many
SELECT lb.*, lt.code, lt.name as leave_type_name
FROM leave_balances lb
JOIN leave_types lt ON lt.id = lb.leave_type_id
WHERE lb.company_id = $1 AND lb.employee_id = $2 AND lb.year = $3
ORDER BY lt.name;

-- name: UpsertLeaveBalance :one
INSERT INTO leave_balances (company_id, employee_id, leave_type_id, year, earned, used, carried, adjusted)
VALUES ($1, $2, $3, $4, $5, 0, $6, 0)
ON CONFLICT (company_id, employee_id, leave_type_id, year) DO UPDATE SET
    earned = EXCLUDED.earned,
    carried = EXCLUDED.carried,
    updated_at = NOW()
RETURNING *;

-- name: DeductLeaveBalance :exec
UPDATE leave_balances SET
    used = used + $5,
    updated_at = NOW()
WHERE company_id = $1 AND employee_id = $2 AND leave_type_id = $3 AND year = $4;

-- name: CreateLeaveRequest :one
INSERT INTO leave_requests (
    company_id, employee_id, leave_type_id,
    start_date, end_date, days, reason, attachment_path
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetLeaveRequest :one
SELECT * FROM leave_requests WHERE id = $1 AND company_id = $2;

-- name: ListLeaveRequests :many
SELECT lr.*, lt.name as leave_type_name,
       e.first_name || ' ' || e.last_name as employee_name
FROM leave_requests lr
JOIN leave_types lt ON lt.id = lr.leave_type_id
JOIN employees e ON e.id = lr.employee_id
WHERE lr.company_id = $1
  AND ($2::bigint IS NULL OR lr.employee_id = $2)
  AND ($3::varchar IS NULL OR lr.status = $3)
ORDER BY lr.created_at DESC
LIMIT $4 OFFSET $5;

-- name: CountLeaveRequests :one
SELECT COUNT(*) FROM leave_requests
WHERE company_id = $1
  AND ($2::bigint IS NULL OR employee_id = $2)
  AND ($3::varchar IS NULL OR status = $3);

-- name: ApproveLeaveRequest :one
UPDATE leave_requests SET
    status = 'approved',
    approver_id = $3,
    approved_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'pending'
RETURNING *;

-- name: RejectLeaveRequest :one
UPDATE leave_requests SET
    status = 'rejected',
    approver_id = $3,
    rejection_reason = $4,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'pending'
RETURNING *;

-- name: CancelLeaveRequest :one
UPDATE leave_requests SET
    status = 'cancelled',
    updated_at = NOW()
WHERE id = $1 AND employee_id = $2 AND status = 'pending'
RETURNING *;
