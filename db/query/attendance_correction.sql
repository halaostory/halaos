-- name: CreateAttendanceCorrection :one
INSERT INTO attendance_corrections (
    company_id, employee_id, attendance_id, correction_date,
    original_clock_in, original_clock_out,
    requested_clock_in, requested_clock_out, reason
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListAttendanceCorrections :many
SELECT ac.*, e.first_name, e.last_name, e.employee_no
FROM attendance_corrections ac
JOIN employees e ON e.id = ac.employee_id
WHERE ac.company_id = $1
ORDER BY ac.created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListPendingCorrections :many
SELECT ac.*, e.first_name, e.last_name, e.employee_no
FROM attendance_corrections ac
JOIN employees e ON e.id = ac.employee_id
WHERE ac.company_id = $1 AND ac.status = 'pending'
ORDER BY ac.created_at DESC;

-- name: ListMyCorrections :many
SELECT ac.* FROM attendance_corrections ac
WHERE ac.company_id = $1 AND ac.employee_id = $2
ORDER BY ac.created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetAttendanceCorrection :one
SELECT ac.*, e.first_name, e.last_name, e.employee_no
FROM attendance_corrections ac
JOIN employees e ON e.id = ac.employee_id
WHERE ac.id = $1 AND ac.company_id = $2;

-- name: ApproveAttendanceCorrection :one
UPDATE attendance_corrections SET
    status = 'approved',
    reviewed_by = $3,
    reviewed_at = NOW(),
    review_note = $4,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'pending'
RETURNING *;

-- name: RejectAttendanceCorrection :one
UPDATE attendance_corrections SET
    status = 'rejected',
    reviewed_by = $3,
    reviewed_at = NOW(),
    review_note = $4,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'pending'
RETURNING *;

-- name: CountPendingCorrections :one
SELECT COUNT(*) FROM attendance_corrections
WHERE company_id = $1 AND status = 'pending';
