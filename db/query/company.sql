-- name: CreateCompany :one
INSERT INTO companies (name) VALUES ($1) RETURNING *;

-- name: GetCompanyByID :one
SELECT * FROM companies WHERE id = $1;

-- name: UpdateCompany :one
UPDATE companies SET
    name = COALESCE($2, name),
    legal_name = COALESCE($3, legal_name),
    tin = COALESCE($4, tin),
    bir_rdo = COALESCE($5, bir_rdo),
    address = COALESCE($6, address),
    city = COALESCE($7, city),
    province = COALESCE($8, province),
    zip_code = COALESCE($9, zip_code),
    timezone = COALESCE($10, timezone),
    pay_frequency = COALESCE($11, pay_frequency),
    logo_url = COALESCE($12, logo_url),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CreateDepartment :one
INSERT INTO departments (company_id, code, name, parent_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetDepartmentByID :one
SELECT * FROM departments WHERE id = $1 AND company_id = $2;

-- name: ListDepartments :many
SELECT * FROM departments WHERE company_id = $1 AND is_active = true ORDER BY name;

-- name: UpdateDepartment :one
UPDATE departments SET
    name = COALESCE($3, name),
    parent_id = $4,
    head_employee_id = $5,
    is_active = COALESCE($6, is_active),
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: CreatePosition :one
INSERT INTO positions (company_id, code, title, department_id, grade)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetPositionByID :one
SELECT * FROM positions WHERE id = $1 AND company_id = $2;

-- name: ListPositions :many
SELECT * FROM positions WHERE company_id = $1 AND is_active = true ORDER BY title;

-- name: UpdatePosition :one
UPDATE positions SET
    title = COALESCE($3, title),
    department_id = $4,
    grade = $5,
    is_active = COALESCE($6, is_active),
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: CreateCostCenter :one
INSERT INTO cost_centers (company_id, code, name)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListCostCenters :many
SELECT * FROM cost_centers WHERE company_id = $1 AND is_active = true ORDER BY name;
