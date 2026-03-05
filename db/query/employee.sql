-- name: GetEmployeeByUserID :one
SELECT * FROM employees WHERE user_id = $1 AND company_id = $2 LIMIT 1;

-- name: CreateEmployee :one
INSERT INTO employees (
    company_id, employee_no, first_name, last_name, middle_name, suffix,
    display_name, email, phone, birth_date, gender, civil_status,
    department_id, position_id, cost_center_id, manager_id,
    hire_date, employment_type, status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
    $13, $14, $15, $16, $17, $18, 'active'
) RETURNING *;

-- name: GetEmployeeByID :one
SELECT * FROM employees WHERE id = $1 AND company_id = $2;

-- name: GetEmployeeByNo :one
SELECT * FROM employees WHERE employee_no = $1 AND company_id = $2;

-- name: ListEmployees :many
SELECT * FROM employees
WHERE company_id = $1
  AND ($2::varchar IS NULL OR status = $2)
  AND ($3::bigint IS NULL OR department_id = $3)
ORDER BY last_name, first_name
LIMIT $4 OFFSET $5;

-- name: CountEmployees :one
SELECT COUNT(*) FROM employees
WHERE company_id = $1
  AND ($2::varchar IS NULL OR status = $2)
  AND ($3::bigint IS NULL OR department_id = $3);

-- name: UpdateEmployee :one
UPDATE employees SET
    first_name = COALESCE($3, first_name),
    last_name = COALESCE($4, last_name),
    middle_name = $5,
    display_name = $6,
    email = $7,
    phone = $8,
    department_id = $9,
    position_id = $10,
    cost_center_id = $11,
    manager_id = $12,
    employment_type = COALESCE($13, employment_type),
    status = COALESCE($14, status),
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: UpsertEmployeeProfile :one
INSERT INTO employee_profiles (
    employee_id, address_line1, address_line2, city, province, zip_code,
    emergency_name, emergency_phone, emergency_relation,
    bank_name, bank_account_no, bank_account_name,
    tin, sss_no, philhealth_no, pagibig_no
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
ON CONFLICT (employee_id) DO UPDATE SET
    address_line1 = EXCLUDED.address_line1,
    address_line2 = EXCLUDED.address_line2,
    city = EXCLUDED.city,
    province = EXCLUDED.province,
    zip_code = EXCLUDED.zip_code,
    emergency_name = EXCLUDED.emergency_name,
    emergency_phone = EXCLUDED.emergency_phone,
    emergency_relation = EXCLUDED.emergency_relation,
    bank_name = EXCLUDED.bank_name,
    bank_account_no = EXCLUDED.bank_account_no,
    bank_account_name = EXCLUDED.bank_account_name,
    tin = EXCLUDED.tin,
    sss_no = EXCLUDED.sss_no,
    philhealth_no = EXCLUDED.philhealth_no,
    pagibig_no = EXCLUDED.pagibig_no,
    updated_at = NOW()
RETURNING *;

-- name: GetEmployeeProfile :one
SELECT * FROM employee_profiles WHERE employee_id = $1;

-- name: CreateEmployeeDocument :one
INSERT INTO employee_documents (
    company_id, employee_id, doc_type, file_name, file_path,
    file_size, mime_type, file_hash, uploaded_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListEmployeeDocuments :many
SELECT * FROM employee_documents WHERE employee_id = $1 ORDER BY created_at DESC;

-- name: CreateEmploymentHistory :one
INSERT INTO employment_history (
    company_id, employee_id, action_type, effective_date,
    from_department_id, to_department_id, from_position_id, to_position_id,
    remarks, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListEmploymentHistory :many
SELECT * FROM employment_history WHERE employee_id = $1 ORDER BY effective_date DESC;
