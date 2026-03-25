-- name: UpsertVirtualOfficeConfig :one
INSERT INTO virtual_office_config (company_id, template)
VALUES ($1, $2)
ON CONFLICT (company_id)
DO UPDATE SET template = $2, updated_at = NOW()
RETURNING *;

-- name: GetVirtualOfficeConfig :one
SELECT * FROM virtual_office_config WHERE company_id = $1;

-- name: AssignSeat :one
INSERT INTO virtual_office_seats (company_id, employee_id, floor, zone, seat_x, seat_y)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: RemoveSeat :exec
DELETE FROM virtual_office_seats WHERE company_id = $1 AND employee_id = $2;

-- name: ListSeats :many
SELECT s.*, e.first_name, e.last_name,
       COALESCE(d.name, '') AS department_name
FROM virtual_office_seats s
JOIN employees e ON e.id = s.employee_id
LEFT JOIN departments d ON d.id = e.department_id
WHERE s.company_id = $1
ORDER BY s.floor, s.zone, s.seat_y, s.seat_x;

-- name: GetSeatByEmployee :one
SELECT * FROM virtual_office_seats WHERE company_id = $1 AND employee_id = $2;

-- name: UpdateSeatStatus :exec
UPDATE virtual_office_seats SET
    custom_status = $3,
    custom_emoji = $4,
    manual_status = $5,
    meeting_room_zone = $6,
    updated_at = NOW()
WHERE company_id = $1 AND employee_id = $2;

-- name: UpdateSeatAvatar :exec
UPDATE virtual_office_seats SET
    avatar_type = $3,
    avatar_color = $4,
    updated_at = NOW()
WHERE company_id = $1 AND employee_id = $2;

-- name: GetSnapshotSeats :many
WITH today_leaves AS (
    SELECT DISTINCT ON (lr.employee_id)
           lr.employee_id, lt.name AS leave_type
    FROM leave_requests lr
    JOIN leave_types lt ON lt.id = lr.leave_type_id
    WHERE lr.company_id = $1
      AND lr.status = 'approved'
      AND CURRENT_DATE BETWEEN lr.start_date AND lr.end_date
    ORDER BY lr.employee_id, lr.created_at DESC
),
today_attendance AS (
    SELECT DISTINCT ON (employee_id)
           employee_id,
           clock_in_at,
           clock_out_at,
           late_minutes
    FROM attendance_logs
    WHERE company_id = $1
      AND clock_in_at >= CURRENT_DATE
      AND clock_in_at < CURRENT_DATE + INTERVAL '1 day'
    ORDER BY employee_id, clock_in_at DESC
)
SELECT
    s.id AS seat_id,
    s.employee_id,
    e.first_name || ' ' || e.last_name AS name,
    COALESCE(p.title, '') AS position,
    COALESCE(d.name, '') AS department,
    s.floor,
    s.zone,
    s.seat_x,
    s.seat_y,
    s.avatar_type,
    s.avatar_color,
    s.custom_status,
    s.custom_emoji,
    s.manual_status,
    s.meeting_room_zone,
    tl.leave_type,
    ta.clock_in_at,
    ta.clock_out_at,
    COALESCE(ta.late_minutes, 0) AS late_minutes
FROM virtual_office_seats s
JOIN employees e ON e.id = s.employee_id AND e.company_id = s.company_id
LEFT JOIN positions p ON p.id = e.position_id
LEFT JOIN departments d ON d.id = e.department_id
LEFT JOIN today_leaves tl ON tl.employee_id = s.employee_id
LEFT JOIN today_attendance ta ON ta.employee_id = s.employee_id
WHERE s.company_id = $1
  AND e.status IN ('active', 'probationary')
ORDER BY s.floor, s.zone, s.seat_y, s.seat_x;

-- name: ListUnassignedActiveEmployees :many
SELECT e.id, e.first_name, e.last_name, e.department_id
FROM employees e
WHERE e.company_id = $1
  AND e.status IN ('active', 'probationary')
  AND NOT EXISTS (
    SELECT 1 FROM virtual_office_seats s
    WHERE s.company_id = e.company_id AND s.employee_id = e.id
  )
ORDER BY e.department_id, e.last_name, e.first_name;

-- name: ListOccupiedPositions :many
SELECT floor, seat_x, seat_y FROM virtual_office_seats WHERE company_id = $1;
