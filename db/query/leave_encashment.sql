-- name: CreateLeaveEncashment :one
INSERT INTO leave_encashments (
    company_id, employee_id, leave_type_id, year,
    days, daily_rate, total_amount, remarks
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListLeaveEncashments :many
SELECT le.*, lt.name as leave_type_name,
       e.first_name || ' ' || e.last_name as employee_name,
       e.employee_no
FROM leave_encashments le
JOIN leave_types lt ON lt.id = le.leave_type_id
JOIN employees e ON e.id = le.employee_id
WHERE le.company_id = $1
  AND ($2::varchar IS NULL OR $2 = '' OR le.status = $2)
  AND ($3::bigint IS NULL OR $3 = 0 OR le.employee_id = $3)
ORDER BY le.created_at DESC
LIMIT $4 OFFSET $5;

-- name: CountLeaveEncashments :one
SELECT COUNT(*) FROM leave_encashments
WHERE company_id = $1
  AND ($2::varchar IS NULL OR $2 = '' OR status = $2)
  AND ($3::bigint IS NULL OR $3 = 0 OR employee_id = $3);

-- name: ApproveLeaveEncashment :one
UPDATE leave_encashments SET
    status = 'approved',
    approved_by = $3,
    approved_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'pending'
RETURNING *;

-- name: RejectLeaveEncashment :one
UPDATE leave_encashments SET
    status = 'rejected',
    approved_by = $3,
    remarks = COALESCE($4, remarks),
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'pending'
RETURNING *;

-- name: MarkLeaveEncashmentPaid :one
UPDATE leave_encashments SET
    status = 'paid',
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'approved'
RETURNING *;

-- name: GetConvertibleLeaveBalances :many
SELECT lb.id, lb.employee_id, lb.leave_type_id, lb.year,
       lb.earned, lb.used, lb.carried, lb.adjusted,
       (lb.earned + lb.carried + lb.adjusted - lb.used) as remaining,
       lt.code, lt.name as leave_type_name
FROM leave_balances lb
JOIN leave_types lt ON lt.id = lb.leave_type_id
WHERE lb.company_id = $1
  AND lb.employee_id = $2
  AND lb.year = $3
  AND lt.is_convertible = true
  AND (lb.earned + lb.carried + lb.adjusted - lb.used) > 0
ORDER BY lt.name;
