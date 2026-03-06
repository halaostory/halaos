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

-- name: GetAttendanceByID :one
SELECT * FROM attendance_logs
WHERE id = $1 AND company_id = $2;

-- name: ListAttendanceLogs :many
SELECT * FROM attendance_logs
WHERE company_id = $1
  AND ($2::bigint IS NULL OR $2 = 0 OR employee_id = $2)
  AND clock_in_at >= $3
  AND clock_in_at < $4
ORDER BY clock_in_at DESC
LIMIT $5 OFFSET $6;

-- name: CountAttendanceLogs :one
SELECT COUNT(*) FROM attendance_logs
WHERE company_id = $1
  AND ($2::bigint IS NULL OR $2 = 0 OR employee_id = $2)
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

-- name: GetAttendanceSummaryForPeriod :many
SELECT
    employee_id,
    COUNT(*) as days_worked,
    COALESCE(SUM(work_hours), 0) as total_work_hours,
    COALESCE(SUM(overtime_hours), 0) as total_overtime_hours,
    COALESCE(SUM(late_minutes), 0) as total_late_minutes,
    COALESCE(SUM(undertime_minutes), 0) as total_undertime_minutes
FROM attendance_logs
WHERE company_id = $1
  AND clock_in_at >= $2
  AND clock_in_at < $3
  AND status IN ('present', 'late', 'undertime')
GROUP BY employee_id;

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

-- name: ListAllSchedules :many
SELECT ws.id, ws.company_id, ws.employee_id, ws.shift_id, ws.work_date, ws.is_rest_day,
       s.name as shift_name, s.start_time, s.end_time,
       e.first_name, e.last_name, e.employee_no
FROM work_schedules ws
JOIN shifts s ON s.id = ws.shift_id
JOIN employees e ON e.id = ws.employee_id
WHERE ws.company_id = $1
  AND ws.work_date >= $2 AND ws.work_date <= $3
ORDER BY ws.work_date, e.last_name;

-- name: BulkAssignSchedule :exec
INSERT INTO work_schedules (company_id, employee_id, shift_id, work_date, is_rest_day)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (company_id, employee_id, work_date) DO UPDATE SET
    shift_id = EXCLUDED.shift_id,
    is_rest_day = EXCLUDED.is_rest_day;

-- name: GetAttendanceReport :many
SELECT
    e.id as employee_id, e.employee_no, e.first_name, e.last_name,
    COALESCE(d.name, '') as department_name,
    COUNT(al.id) as days_worked,
    COALESCE(SUM(al.work_hours), 0) as total_work_hours,
    COALESCE(SUM(al.overtime_hours), 0) as total_overtime_hours,
    COALESCE(SUM(al.late_minutes), 0) as total_late_minutes,
    COALESCE(SUM(al.undertime_minutes), 0) as total_undertime_minutes,
    COUNT(CASE WHEN al.status = 'late' THEN 1 END) as late_count,
    COUNT(CASE WHEN al.status = 'undertime' THEN 1 END) as undertime_count
FROM employees e
LEFT JOIN departments d ON d.id = e.department_id
LEFT JOIN attendance_logs al ON al.employee_id = e.id
    AND al.clock_in_at >= $2
    AND al.clock_in_at < $3
    AND al.status IN ('present', 'late', 'undertime')
WHERE e.company_id = $1 AND e.status = 'active'
GROUP BY e.id, e.employee_no, e.first_name, e.last_name, d.name
ORDER BY e.last_name, e.first_name;

-- name: GetAttendanceRecordsForPeriod :many
SELECT id, employee_id, clock_in_at, clock_out_at
FROM attendance_logs
WHERE company_id = $1
  AND clock_in_at >= $2
  AND clock_in_at < $3
  AND clock_out_at IS NOT NULL
  AND status IN ('present', 'late', 'undertime')
ORDER BY employee_id, clock_in_at;

-- name: GetHolidayAttendanceForPeriod :many
SELECT
    al.employee_id,
    h.holiday_type,
    COUNT(*) as days_worked
FROM attendance_logs al
JOIN holidays h ON h.company_id = al.company_id
    AND h.holiday_date = (al.clock_in_at AT TIME ZONE 'Asia/Manila')::date
WHERE al.company_id = $1
  AND al.clock_in_at >= $2
  AND al.clock_in_at < $3
  AND al.status IN ('present', 'late', 'undertime')
GROUP BY al.employee_id, h.holiday_type;

-- name: DeleteSchedule :exec
DELETE FROM work_schedules WHERE id = $1 AND company_id = $2;

-- name: ListOpenAttendanceRecords :many
SELECT al.* FROM attendance_logs al
WHERE al.clock_out_at IS NULL
  AND al.clock_in_at < NOW() - INTERVAL '16 hours'
ORDER BY al.clock_in_at;

-- name: AutoCloseAttendance :exec
UPDATE attendance_logs SET
    clock_out_at = clock_in_at + INTERVAL '8 hours',
    clock_out_source = 'system_auto_close',
    clock_out_note = 'Auto-closed by system (no clock-out recorded)',
    work_hours = 8,
    overtime_hours = 0,
    status = 'present'
WHERE id = $1;

-- name: GetDTR :many
SELECT al.id, al.employee_id, al.clock_in_at, al.clock_out_at,
       al.work_hours, al.overtime_hours, al.late_minutes, al.undertime_minutes,
       al.status, al.clock_in_source, al.clock_out_source,
       e.employee_no, e.first_name, e.last_name,
       COALESCE(d.name, '') as department_name,
       COALESCE(p.title, '') as position_name
FROM attendance_logs al
JOIN employees e ON e.id = al.employee_id
LEFT JOIN departments d ON d.id = e.department_id
LEFT JOIN positions p ON p.id = e.position_id
WHERE al.company_id = $1
  AND al.employee_id = $2
  AND al.clock_in_at >= $3
  AND al.clock_in_at < $4
ORDER BY al.clock_in_at;

-- name: GetDTRAllEmployees :many
SELECT al.id, al.employee_id, al.clock_in_at, al.clock_out_at,
       al.work_hours, al.overtime_hours, al.late_minutes, al.undertime_minutes,
       al.status, al.clock_in_source, al.clock_out_source,
       e.employee_no, e.first_name, e.last_name,
       COALESCE(d.name, '') as department_name,
       COALESCE(p.title, '') as position_name
FROM attendance_logs al
JOIN employees e ON e.id = al.employee_id
LEFT JOIN departments d ON d.id = e.department_id
LEFT JOIN positions p ON p.id = e.position_id
WHERE al.company_id = $1
  AND al.clock_in_at >= $2
  AND al.clock_in_at < $3
ORDER BY e.last_name, e.first_name, al.clock_in_at;

-- name: ExportAttendanceLogs :many
SELECT al.clock_in_at, al.clock_out_at, al.work_hours, al.overtime_hours,
       al.late_minutes, al.undertime_minutes, al.status,
       e.employee_no, e.first_name, e.last_name
FROM attendance_logs al
JOIN employees e ON e.id = al.employee_id
WHERE al.company_id = $1
  AND al.clock_in_at >= $2
  AND al.clock_in_at < $3
ORDER BY al.clock_in_at DESC;
