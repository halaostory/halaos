-- Pay schedules.

-- name: ListPaySchedules :many
SELECT s.*,
       (SELECT COUNT(*) FROM employee_pay_schedules eps JOIN employees e ON e.id = eps.employee_id
         WHERE eps.pay_schedule_id = s.id AND e.status IN ('active', 'probationary'))::bigint AS assigned_employees
FROM pay_schedules s WHERE s.company_id = $1 ORDER BY s.is_default DESC, s.name;

-- name: CreatePaySchedule :one
INSERT INTO pay_schedules (company_id, name, frequency, is_default) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: UpdatePaySchedule :one
UPDATE pay_schedules SET name = $3, frequency = $4 WHERE id = $1 AND company_id = $2 RETURNING *;

-- name: ClearDefaultPaySchedule :exec
UPDATE pay_schedules SET is_default = false WHERE company_id = $1 AND is_default;

-- name: SetDefaultPaySchedule :exec
UPDATE pay_schedules SET is_default = true WHERE id = $1 AND company_id = $2;

-- name: GetPaySchedule :one
SELECT * FROM pay_schedules WHERE id = $1 AND company_id = $2;

-- name: GetDefaultPaySchedule :one
SELECT * FROM pay_schedules WHERE company_id = $1 AND is_default;

-- name: AssignEmployeePaySchedule :exec
INSERT INTO employee_pay_schedules (employee_id, pay_schedule_id) VALUES ($1, $2)
ON CONFLICT (employee_id) DO UPDATE SET pay_schedule_id = EXCLUDED.pay_schedule_id, updated_at = NOW();

-- name: ListEmployeePayScheduleMap :many
SELECT eps.employee_id, eps.pay_schedule_id
FROM employee_pay_schedules eps JOIN employees e ON e.id = eps.employee_id
WHERE e.company_id = $1;

-- name: SetCycleSchedule :exec
INSERT INTO payroll_cycle_schedules (cycle_id, pay_schedule_id) VALUES ($1, $2)
ON CONFLICT (cycle_id) DO UPDATE SET pay_schedule_id = EXCLUDED.pay_schedule_id;

-- name: GetCycleSchedule :one
SELECT s.* FROM payroll_cycle_schedules cs JOIN pay_schedules s ON s.id = cs.pay_schedule_id
WHERE cs.cycle_id = $1;
