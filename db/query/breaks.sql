-- name: CreateBreakLog :one
INSERT INTO break_logs (company_id, employee_id, attendance_log_id, break_type, note)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: EndBreakLog :one
UPDATE break_logs SET
    end_at = NOW(),
    duration_minutes = EXTRACT(EPOCH FROM (NOW() - start_at))::int / 60,
    overtime_minutes = CASE
        WHEN $2::int > 0 AND EXTRACT(EPOCH FROM (NOW() - start_at))::int / 60 > $2::int
        THEN EXTRACT(EPOCH FROM (NOW() - start_at))::int / 60 - $2::int
        ELSE 0
    END
WHERE id = $1 AND end_at IS NULL
RETURNING *;

-- name: GetActiveBreak :one
SELECT * FROM break_logs
WHERE employee_id = $1 AND company_id = $2 AND end_at IS NULL
ORDER BY start_at DESC LIMIT 1;

-- name: ListBreaksByAttendance :many
SELECT * FROM break_logs
WHERE attendance_log_id = $1
ORDER BY start_at;

-- name: ListBreaksByDate :many
SELECT * FROM break_logs
WHERE employee_id = $1 AND company_id = $2
    AND start_at >= $3 AND start_at < $4
ORDER BY start_at;

-- name: GetMonthlyBreakSummary :many
SELECT
    bl.employee_id,
    bl.break_type,
    COUNT(*)::int as total_count,
    COALESCE(SUM(bl.duration_minutes), 0)::int as total_minutes,
    COALESCE(SUM(bl.overtime_minutes), 0)::int as total_overtime_minutes,
    COUNT(CASE WHEN bl.overtime_minutes > 0 THEN 1 END)::int as overtime_count
FROM break_logs bl
WHERE bl.company_id = $1
    AND bl.start_at >= $2 AND bl.start_at < $3
    AND bl.end_at IS NOT NULL
GROUP BY bl.employee_id, bl.break_type;

-- name: ListBreakPolicies :many
SELECT * FROM break_policies
WHERE company_id = $1 AND is_active = true;

-- name: UpsertBreakPolicy :one
INSERT INTO break_policies (company_id, break_type, max_minutes)
VALUES ($1, $2, $3)
ON CONFLICT (company_id, break_type)
DO UPDATE SET max_minutes = $3, updated_at = NOW()
RETURNING *;

-- name: GetBreakPolicy :one
SELECT * FROM break_policies
WHERE company_id = $1 AND break_type = $2 AND is_active = true;
