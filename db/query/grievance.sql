-- name: CreateGrievance :one
INSERT INTO grievance_cases (
    company_id, employee_id, case_number, category,
    subject, description, severity, is_anonymous
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListGrievances :many
SELECT gc.*, e.employee_no, e.first_name, e.last_name,
       COALESCE(u.first_name || ' ' || u.last_name, '') as assigned_to_name
FROM grievance_cases gc
JOIN employees e ON e.id = gc.employee_id
LEFT JOIN users u ON u.id = gc.assigned_to
WHERE gc.company_id = @company_id
  AND (@status = '' OR gc.status = @status)
  AND (@category = '' OR gc.category = @category)
  AND (@employee_id = 0 OR gc.employee_id = @employee_id)
ORDER BY gc.created_at DESC
LIMIT @lim OFFSET @off;

-- name: CountGrievances :one
SELECT COUNT(*) FROM grievance_cases
WHERE company_id = @company_id
  AND (@status = '' OR status = @status)
  AND (@category = '' OR category = @category)
  AND (@employee_id = 0 OR employee_id = @employee_id);

-- name: GetGrievance :one
SELECT gc.*, e.employee_no, e.first_name, e.last_name,
       COALESCE(u.first_name || ' ' || u.last_name, '') as assigned_to_name
FROM grievance_cases gc
JOIN employees e ON e.id = gc.employee_id
LEFT JOIN users u ON u.id = gc.assigned_to
WHERE gc.id = $1 AND gc.company_id = $2;

-- name: ListMyGrievances :many
SELECT gc.*, COALESCE(u.first_name || ' ' || u.last_name, '') as assigned_to_name
FROM grievance_cases gc
LEFT JOIN users u ON u.id = gc.assigned_to
WHERE gc.company_id = $1 AND gc.employee_id = $2
ORDER BY gc.created_at DESC;

-- name: UpdateGrievanceStatus :one
UPDATE grievance_cases SET
    status = $3,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: AssignGrievance :one
UPDATE grievance_cases SET
    assigned_to = $3,
    status = 'under_review',
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: ResolveGrievance :one
UPDATE grievance_cases SET
    status = 'resolved',
    resolution = $3,
    resolution_date = CURRENT_DATE,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: WithdrawGrievance :one
UPDATE grievance_cases SET
    status = 'withdrawn',
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND employee_id = $3 AND status = 'open'
RETURNING *;

-- name: AddGrievanceComment :one
INSERT INTO grievance_comments (grievance_id, user_id, comment, is_internal)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListGrievanceComments :many
SELECT gc.*, u.first_name || ' ' || u.last_name as user_name
FROM grievance_comments gc
JOIN users u ON u.id = gc.user_id
WHERE gc.grievance_id = $1
ORDER BY gc.created_at;

-- name: GetGrievanceSummary :one
SELECT
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE status = 'open') as open_count,
    COUNT(*) FILTER (WHERE status = 'under_review') as under_review,
    COUNT(*) FILTER (WHERE status = 'in_mediation') as in_mediation,
    COUNT(*) FILTER (WHERE status = 'resolved') as resolved,
    COUNT(*) FILTER (WHERE severity = 'critical' AND status NOT IN ('resolved', 'closed', 'withdrawn')) as critical_open
FROM grievance_cases
WHERE company_id = $1;

-- name: NextGrievanceCaseNumber :one
SELECT COALESCE(MAX(CAST(SUBSTRING(case_number FROM 'GRV-(\d+)') AS INT)), 0) + 1 as next_num
FROM grievance_cases
WHERE company_id = $1;
