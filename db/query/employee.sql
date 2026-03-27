-- name: GetEmployeeByUserID :one
SELECT * FROM employees WHERE user_id = $1 AND company_id = $2 LIMIT 1;

-- name: CreateEmployee :one
INSERT INTO employees (
    company_id, employee_no, first_name, last_name, middle_name, suffix,
    display_name, email, phone, birth_date, gender, civil_status,
    nationality, department_id, position_id, cost_center_id, manager_id,
    hire_date, employment_type, status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
    $13, $14, $15, $16, $17, $18, $19, 'active'
) RETURNING *;

-- name: GetEmployeeByID :one
SELECT * FROM employees WHERE id = $1 AND company_id = $2;

-- name: GetEmployeeByNo :one
SELECT * FROM employees WHERE employee_no = $1 AND company_id = $2;

-- name: LinkUserToEmployee :exec
UPDATE employees SET user_id = $1 WHERE id = $2 AND company_id = $3;

-- name: ListEmployees :many
SELECT * FROM employees
WHERE company_id = $1
  AND ($2::varchar IS NULL OR $2 = '' OR status = $2)
  AND ($3::bigint IS NULL OR $3 = 0 OR department_id = $3)
ORDER BY last_name, first_name
LIMIT $4 OFFSET $5;

-- name: CountEmployees :one
SELECT COUNT(*) FROM employees
WHERE company_id = $1
  AND ($2::varchar IS NULL OR $2 = '' OR status = $2)
  AND ($3::bigint IS NULL OR $3 = 0 OR department_id = $3);

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
    nationality = COALESCE($15, nationality),
    birth_date = COALESCE(sqlc.narg('birth_date'), birth_date),
    hire_date = COALESCE(sqlc.narg('hire_date'), hire_date),
    gender = COALESCE(sqlc.narg('gender'), gender),
    civil_status = COALESCE(sqlc.narg('civil_status'), civil_status),
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
SELECT ep.* FROM employee_profiles ep
JOIN employees e ON e.id = ep.employee_id
WHERE ep.employee_id = $1 AND e.company_id = $2;

-- name: CreateEmployeeDocument :one
INSERT INTO employee_documents (
    company_id, employee_id, doc_type, file_name, file_path,
    file_size, mime_type, file_hash, uploaded_by, expiry_date
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListEmployeeDocuments :many
SELECT * FROM employee_documents WHERE employee_id = $1 AND company_id = $2 ORDER BY created_at DESC;

-- name: GetEmployeeDocument :one
SELECT * FROM employee_documents WHERE id = $1 AND company_id = $2;

-- name: DeleteEmployeeDocument :exec
DELETE FROM employee_documents WHERE id = $1 AND company_id = $2;

-- name: ListExpiringDocuments :many
SELECT ed.id, ed.company_id, ed.employee_id, ed.doc_type, ed.file_name,
       ed.expiry_date, e.first_name, e.last_name, e.employee_no
FROM employee_documents ed
JOIN employees e ON e.id = ed.employee_id
WHERE ed.company_id = $1
  AND ed.expiry_date IS NOT NULL
  AND ed.expiry_date BETWEEN NOW()::date AND (NOW() + INTERVAL '60 days')::date
ORDER BY ed.expiry_date ASC
LIMIT 50;

-- name: CreateEmploymentHistory :one
INSERT INTO employment_history (
    company_id, employee_id, action_type, effective_date,
    from_department_id, to_department_id, from_position_id, to_position_id,
    remarks, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListEmploymentHistory :many
SELECT * FROM employment_history WHERE employee_id = $1 AND company_id = $2 ORDER BY effective_date DESC;

-- name: ListEmployeeTimeline :many
SELECT eh.id, eh.action_type, eh.effective_date, eh.remarks, eh.created_at,
       COALESCE(fd.name, '') as from_department,
       COALESCE(td.name, '') as to_department,
       COALESCE(fp.title, '') as from_position,
       COALESCE(tp.title, '') as to_position,
       COALESCE(u.email, '') as created_by_email
FROM employment_history eh
LEFT JOIN departments fd ON fd.id = eh.from_department_id
LEFT JOIN departments td ON td.id = eh.to_department_id
LEFT JOIN positions fp ON fp.id = eh.from_position_id
LEFT JOIN positions tp ON tp.id = eh.to_position_id
LEFT JOIN users u ON u.id = eh.created_by
WHERE eh.employee_id = $1 AND eh.company_id = $2
ORDER BY eh.effective_date DESC, eh.created_at DESC
LIMIT 50;

-- name: ListActiveEmployees :many
SELECT * FROM employees
WHERE company_id = $1 AND status = 'active'
ORDER BY last_name, first_name;

-- name: ListEmployeeDirectory :many
SELECT e.id, e.employee_no, e.first_name, e.last_name, e.display_name,
       e.email, e.phone, e.status, e.employment_type, e.manager_id,
       COALESCE(d.name, '') as department_name,
       COALESCE(p.title, '') as position_title,
       u.avatar_url
FROM employees e
LEFT JOIN departments d ON d.id = e.department_id
LEFT JOIN positions p ON p.id = e.position_id
LEFT JOIN users u ON u.id = e.user_id
WHERE e.company_id = $1
  AND e.status = 'active'
  AND ($2::varchar IS NULL OR $2 = '' OR
    e.first_name ILIKE $2 OR e.last_name ILIKE $2 OR
    e.employee_no ILIKE $2 OR e.email ILIKE $2)
  AND ($3::bigint IS NULL OR $3 = 0 OR e.department_id = $3)
ORDER BY e.last_name, e.first_name;

-- name: GetEmployeeForCOE :one
SELECT e.id, e.employee_no, e.first_name, e.last_name, e.middle_name,
       e.hire_date, e.employment_type, e.status,
       COALESCE(d.name, '') as department_name,
       COALESCE(p.title, '') as position_title
FROM employees e
LEFT JOIN departments d ON d.id = e.department_id
LEFT JOIN positions p ON p.id = e.position_id
WHERE e.id = $1 AND e.company_id = $2;

-- name: ListEmployeesForDOLERegister :many
SELECT e.id, e.employee_no, e.first_name, e.last_name, e.middle_name,
       e.gender, e.birth_date, e.civil_status, e.nationality,
       e.hire_date, e.employment_type, e.status,
       COALESCE(d.name, '') as department_name,
       COALESCE(p.title, '') as position_title,
       COALESCE(ep.tin, '') as tin,
       COALESCE(ep.sss_no, '') as sss_no,
       COALESCE(ep.philhealth_no, '') as philhealth_no,
       COALESCE(ep.pagibig_no, '') as pagibig_no
FROM employees e
LEFT JOIN departments d ON d.id = e.department_id
LEFT JOIN positions p ON p.id = e.position_id
LEFT JOIN employee_profiles ep ON ep.employee_id = e.id
WHERE e.company_id = $1 AND e.status IN ('active', 'probationary')
ORDER BY e.last_name, e.first_name;

-- name: RegularizeEmployee :one
UPDATE employees SET
    employment_type = 'regular',
    regularization_date = CURRENT_DATE,
    status = 'active',
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
  AND employment_type = 'probationary'
  AND status IN ('active', 'probationary')
RETURNING id, employee_no, first_name, last_name;

-- name: GetOrgChartData :many
SELECT e.id, e.first_name, e.last_name, e.display_name,
       e.manager_id,
       COALESCE(d.name, '') as department_name,
       COALESCE(p.title, '') as position_title,
       u.avatar_url
FROM employees e
LEFT JOIN departments d ON d.id = e.department_id
LEFT JOIN positions p ON p.id = e.position_id
LEFT JOIN users u ON u.id = e.user_id
WHERE e.company_id = $1 AND e.status = 'active'
ORDER BY e.last_name, e.first_name;
