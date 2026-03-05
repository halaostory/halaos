-- name: ClockIn :one
INSERT INTO attendance_logs (
    company_id, employee_id, clock_in_at, clock_in_source,
    clock_in_lat, clock_in_lng, clock_in_note
) VALUES ($1, $2, NOW(), $3, $4, $5, $6)
RETURNING *;

-- name: ClockOut :one
UPDATE attendance_logs SET
    clock_out_at = NOW(),
    clock_out_source = $3,
    clock_out_lat = $4,
    clock_out_lng = $5,
    clock_out_note = $6,
    updated_at = NOW()
WHERE id = $1 AND employee_id = $2 AND clock_out_at IS NULL
RETURNING *;

-- name: GetOpenAttendance :one
SELECT * FROM attendance_logs
WHERE employee_id = $1 AND company_id = $2 AND clock_out_at IS NULL
ORDER BY clock_in_at DESC LIMIT 1;

-- name: ListAttendanceLogs :many
SELECT * FROM attendance_logs
WHERE company_id = $1
  AND ($2::bigint IS NULL OR employee_id = $2)
  AND clock_in_at >= $3
  AND clock_in_at < $4
ORDER BY clock_in_at DESC
LIMIT $5 OFFSET $6;

-- name: CountAttendanceLogs :one
SELECT COUNT(*) FROM attendance_logs
WHERE company_id = $1
  AND ($2::bigint IS NULL OR employee_id = $2)
  AND clock_in_at >= $3
  AND clock_in_at < $4;

-- name: UpdateAttendanceCalc :exec
UPDATE attendance_logs SET
    work_hours = $2,
    overtime_hours = $3,
    late_minutes = $4,
    undertime_minutes = $5,
    status = $6,
    updated_at = NOW()
WHERE id = $1;

-- name: GetTodayAttendanceSummary :many
SELECT
    status,
    COUNT(*) as count
FROM attendance_logs
WHERE company_id = $1 AND clock_in_at::date = CURRENT_DATE
GROUP BY status;

-- name: CreateShift :one
INSERT INTO shifts (company_id, name, start_time, end_time, break_minutes, grace_minutes, is_overnight)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListShifts :many
SELECT * FROM shifts WHERE company_id = $1 AND is_active = true ORDER BY name;

-- name: UpdateShift :one
UPDATE shifts SET
    name = COALESCE($3, name),
    start_time = $4,
    end_time = $5,
    break_minutes = COALESCE($6, break_minutes),
    grace_minutes = COALESCE($7, grace_minutes),
    is_overnight = COALESCE($8, is_overnight),
    is_active = COALESCE($9, is_active),
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: AssignSchedule :one
INSERT INTO work_schedules (company_id, employee_id, shift_id, work_date, is_rest_day)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (company_id, employee_id, work_date) DO UPDATE SET
    shift_id = EXCLUDED.shift_id,
    is_rest_day = EXCLUDED.is_rest_day
RETURNING *;

-- name: GetSchedule :one
SELECT * FROM work_schedules
WHERE employee_id = $1 AND company_id = $2 AND work_date = $3;

-- name: ListSchedules :many
SELECT ws.*, s.name as shift_name, s.start_time, s.end_time
FROM work_schedules ws
JOIN shifts s ON s.id = ws.shift_id
WHERE ws.company_id = $1 AND ws.employee_id = $2
  AND ws.work_date >= $3 AND ws.work_date <= $4
ORDER BY ws.work_date;
