-- name: CreateRecognition :one
INSERT INTO recognitions (company_id, from_employee_id, to_employee_id, category, message, is_public, points)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListRecognitions :many
SELECT r.*,
    fe.first_name as from_first_name, fe.last_name as from_last_name, fe.employee_no as from_employee_no,
    te.first_name as to_first_name, te.last_name as to_last_name, te.employee_no as to_employee_no,
    td.name as to_department
FROM recognitions r
JOIN employees fe ON fe.id = r.from_employee_id
JOIN employees te ON te.id = r.to_employee_id
LEFT JOIN departments td ON td.id = te.department_id
WHERE r.company_id = $1 AND r.is_public = true
ORDER BY r.created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListMyRecognitions :many
SELECT r.*,
    fe.first_name as from_first_name, fe.last_name as from_last_name,
    te.first_name as to_first_name, te.last_name as to_last_name
FROM recognitions r
JOIN employees fe ON fe.id = r.from_employee_id
JOIN employees te ON te.id = r.to_employee_id
WHERE r.company_id = $1 AND (r.to_employee_id = $2 OR r.from_employee_id = $2)
ORDER BY r.created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountRecognitionsReceived :one
SELECT COUNT(*) as count FROM recognitions
WHERE to_employee_id = $1 AND company_id = $2;

-- name: GetTopRecognized :many
SELECT te.id as employee_id, te.first_name, te.last_name, te.employee_no,
    d.name as department,
    COUNT(*) as recognition_count,
    SUM(r.points) as total_points
FROM recognitions r
JOIN employees te ON te.id = r.to_employee_id
LEFT JOIN departments d ON d.id = te.department_id
WHERE r.company_id = $1 AND r.created_at >= $2
GROUP BY te.id, te.first_name, te.last_name, te.employee_no, d.name
ORDER BY total_points DESC
LIMIT $3;

-- name: GetRecognitionStats :one
SELECT
    COUNT(*) as total_recognitions,
    COUNT(DISTINCT from_employee_id) as unique_givers,
    COUNT(DISTINCT to_employee_id) as unique_receivers
FROM recognitions
WHERE company_id = $1 AND created_at >= $2;

-- name: GetCategoryBreakdown :many
SELECT category, COUNT(*) as count
FROM recognitions
WHERE company_id = $1 AND created_at >= $2
GROUP BY category
ORDER BY count DESC;
