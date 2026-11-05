-- Workplaces

-- name: ListWorkplaces :many
SELECT w.*, (SELECT COUNT(*) FROM employee_workplaces ew JOIN employees e ON e.id = ew.employee_id
             WHERE ew.workplace_id = w.id AND e.status IN ('active', 'probationary'))::int AS headcount
FROM workplaces w WHERE w.company_id = $1 ORDER BY w.is_active DESC, w.name;

-- name: CreateWorkplace :one
INSERT INTO workplaces (company_id, name, address, city, region, country, is_remote)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING *;

-- name: UpdateWorkplace :one
UPDATE workplaces SET name = $3, address = $4, city = $5, region = $6, country = $7, is_remote = $8, is_active = $9
WHERE id = $1 AND company_id = $2 RETURNING *;

-- name: GetWorkplace :one
SELECT * FROM workplaces WHERE id = $1 AND company_id = $2;

-- name: SetEmployeeWorkplace :exec
INSERT INTO employee_workplaces (employee_id, company_id, workplace_id) VALUES ($1, $2, $3)
ON CONFLICT (employee_id) DO UPDATE SET workplace_id = EXCLUDED.workplace_id, updated_at = now();

-- name: ClearEmployeeWorkplace :exec
DELETE FROM employee_workplaces WHERE employee_id = $1 AND company_id = $2;

-- name: GetEmployeeWorkplace :one
SELECT w.* FROM employee_workplaces ew JOIN workplaces w ON w.id = ew.workplace_id
WHERE ew.employee_id = $1 AND ew.company_id = $2;
