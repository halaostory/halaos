-- name: CreateDisciplinaryIncident :one
INSERT INTO disciplinary_incidents (
    company_id, employee_id, reported_by, incident_date,
    category, severity, description, witnesses, evidence_notes
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListDisciplinaryIncidents :many
SELECT di.*, e.employee_no, e.first_name, e.last_name,
       COALESCE(d.name, '') as department_name
FROM disciplinary_incidents di
JOIN employees e ON e.id = di.employee_id
LEFT JOIN departments d ON d.id = e.department_id
WHERE di.company_id = @company_id
  AND (@status = '' OR di.status = @status)
  AND (@employee_id = 0 OR di.employee_id = @employee_id)
ORDER BY di.incident_date DESC
LIMIT @lim OFFSET @off;

-- name: CountDisciplinaryIncidents :one
SELECT COUNT(*) FROM disciplinary_incidents
WHERE company_id = @company_id
  AND (@status = '' OR status = @status)
  AND (@employee_id = 0 OR employee_id = @employee_id);

-- name: GetDisciplinaryIncident :one
SELECT di.*, e.employee_no, e.first_name, e.last_name,
       COALESCE(d.name, '') as department_name
FROM disciplinary_incidents di
JOIN employees e ON e.id = di.employee_id
LEFT JOIN departments d ON d.id = e.department_id
WHERE di.id = $1 AND di.company_id = $2;

-- name: UpdateIncidentStatus :one
UPDATE disciplinary_incidents SET
    status = $3,
    resolution_notes = $4,
    resolved_at = CASE WHEN $3 IN ('resolved', 'dismissed') THEN NOW() ELSE NULL END,
    resolved_by = $5,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: CreateDisciplinaryAction :one
INSERT INTO disciplinary_actions (
    company_id, employee_id, incident_id, action_type,
    action_date, issued_by, description, suspension_days,
    effective_date, end_date, notes
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListDisciplinaryActions :many
SELECT da.*, e.employee_no, e.first_name, e.last_name,
       COALESCE(d.name, '') as department_name
FROM disciplinary_actions da
JOIN employees e ON e.id = da.employee_id
LEFT JOIN departments d ON d.id = e.department_id
WHERE da.company_id = @company_id
  AND (@employee_id = 0 OR da.employee_id = @employee_id)
ORDER BY da.action_date DESC
LIMIT @lim OFFSET @off;

-- name: CountDisciplinaryActions :one
SELECT COUNT(*) FROM disciplinary_actions
WHERE company_id = @company_id
  AND (@employee_id = 0 OR employee_id = @employee_id);

-- name: ListActionsByIncident :many
SELECT da.*, e.employee_no, e.first_name, e.last_name
FROM disciplinary_actions da
JOIN employees e ON e.id = da.employee_id
WHERE da.incident_id = $1 AND da.company_id = $2
ORDER BY da.action_date;

-- name: AcknowledgeDisciplinaryAction :one
UPDATE disciplinary_actions SET
    employee_acknowledged = true,
    acknowledged_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: AppealDisciplinaryAction :one
UPDATE disciplinary_actions SET
    appeal_status = 'appealed',
    appeal_reason = $3,
    appeal_date = NOW(),
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND (appeal_status IS NULL)
RETURNING *;

-- name: ResolveAppeal :one
UPDATE disciplinary_actions SET
    appeal_status = $3,
    appeal_resolution = $4,
    appeal_resolved_at = NOW(),
    appeal_resolved_by = $5,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND appeal_status = 'appealed'
RETURNING *;

-- name: GetEmployeeDisciplinarySummary :one
SELECT
    COUNT(*) as total_incidents,
    COUNT(CASE WHEN di.severity = 'grave' THEN 1 END) as grave_count,
    COUNT(CASE WHEN di.status = 'open' THEN 1 END) as open_count
FROM disciplinary_incidents di
WHERE di.company_id = $1 AND di.employee_id = $2;

-- name: GetEmployeeActionCounts :one
SELECT
    COUNT(*) as total_actions,
    COUNT(CASE WHEN da.action_type = 'verbal_warning' THEN 1 END) as verbal_warnings,
    COUNT(CASE WHEN da.action_type = 'written_warning' THEN 1 END) as written_warnings,
    COUNT(CASE WHEN da.action_type = 'final_warning' THEN 1 END) as final_warnings,
    COUNT(CASE WHEN da.action_type = 'suspension' THEN 1 END) as suspensions
FROM disciplinary_actions da
WHERE da.company_id = $1 AND da.employee_id = $2;
