-- name: CreateScheduleTemplate :one
INSERT INTO schedule_templates (company_id, name, description)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListScheduleTemplates :many
SELECT * FROM schedule_templates
WHERE company_id = $1 AND is_active = true
ORDER BY name;

-- name: GetScheduleTemplate :one
SELECT * FROM schedule_templates
WHERE id = $1 AND company_id = $2;

-- name: UpdateScheduleTemplate :one
UPDATE schedule_templates
SET name = $3, description = $4, updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: DeleteScheduleTemplate :exec
UPDATE schedule_templates SET is_active = false, updated_at = NOW()
WHERE id = $1 AND company_id = $2;

-- name: UpsertScheduleTemplateDay :one
INSERT INTO schedule_template_days (template_id, day_of_week, shift_id, is_rest_day)
VALUES ($1, $2, $3, $4)
ON CONFLICT (template_id, day_of_week) DO UPDATE
SET shift_id = EXCLUDED.shift_id, is_rest_day = EXCLUDED.is_rest_day
RETURNING *;

-- name: ListScheduleTemplateDays :many
SELECT std.*, s.name as shift_name, s.start_time, s.end_time
FROM schedule_template_days std
LEFT JOIN shifts s ON s.id = std.shift_id
WHERE std.template_id = $1
ORDER BY std.day_of_week;

-- name: DeleteScheduleTemplateDays :exec
DELETE FROM schedule_template_days WHERE template_id = $1;

-- name: AssignScheduleTemplate :one
INSERT INTO employee_schedule_assignments (company_id, employee_id, template_id, effective_from, effective_to)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (company_id, employee_id, effective_from) DO UPDATE
SET template_id = EXCLUDED.template_id, effective_to = EXCLUDED.effective_to
RETURNING *;

-- name: ListEmployeeScheduleAssignments :many
SELECT esa.*, st.name as template_name, e.first_name, e.last_name, e.employee_no
FROM employee_schedule_assignments esa
JOIN schedule_templates st ON st.id = esa.template_id
JOIN employees e ON e.id = esa.employee_id
WHERE esa.company_id = $1
ORDER BY esa.effective_from DESC
LIMIT $2 OFFSET $3;

-- name: GetEmployeeCurrentTemplate :one
SELECT esa.*, st.name as template_name
FROM employee_schedule_assignments esa
JOIN schedule_templates st ON st.id = esa.template_id
WHERE esa.company_id = $1 AND esa.employee_id = $2
AND esa.effective_from <= $3
AND (esa.effective_to IS NULL OR esa.effective_to >= $3)
ORDER BY esa.effective_from DESC
LIMIT 1;

-- name: DeleteScheduleAssignment :exec
DELETE FROM employee_schedule_assignments WHERE id = $1 AND company_id = $2;
