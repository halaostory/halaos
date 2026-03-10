-- name: CreateHRRequest :one
INSERT INTO hr_requests (company_id, employee_id, request_type, subject, description, priority)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetHRRequest :one
SELECT r.*,
    e.first_name, e.last_name, e.employee_no
FROM hr_requests r
JOIN employees e ON e.id = r.employee_id
WHERE r.id = $1 AND r.company_id = $2;

-- name: ListHRRequestsByEmployee :many
SELECT * FROM hr_requests
WHERE company_id = $1 AND employee_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListHRRequests :many
SELECT r.*,
    e.first_name, e.last_name, e.employee_no,
    d.name as department
FROM hr_requests r
JOIN employees e ON e.id = r.employee_id
LEFT JOIN departments d ON d.id = e.department_id
WHERE r.company_id = $1
    AND ($2 = '' OR r.status = $2)
    AND ($3 = '' OR r.request_type = $3)
ORDER BY
    CASE r.priority
        WHEN 'urgent' THEN 1
        WHEN 'high' THEN 2
        WHEN 'normal' THEN 3
        WHEN 'low' THEN 4
    END,
    r.created_at DESC
LIMIT $4 OFFSET $5;

-- name: UpdateHRRequestStatus :one
UPDATE hr_requests SET
    status = $3,
    assigned_to = $4,
    resolution_note = $5,
    updated_at = NOW(),
    resolved_at = CASE WHEN $3 IN ('resolved', 'closed') THEN NOW() ELSE resolved_at END
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: CountHRRequestsByStatus :many
SELECT status, COUNT(*) as count
FROM hr_requests
WHERE company_id = $1
GROUP BY status;

-- name: CountOpenHRRequests :one
SELECT COUNT(*) as count FROM hr_requests
WHERE company_id = $1 AND status IN ('open', 'in_progress');
