-- name: CreateActivityLog :one
INSERT INTO activity_logs (company_id, user_id, action, entity_type, entity_id, description, ip_address, user_agent, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListActivityLogs :many
SELECT al.*, u.email as user_email, u.first_name, u.last_name
FROM activity_logs al
JOIN users u ON u.id = al.user_id
WHERE al.company_id = $1
  AND ($2::varchar = '' OR al.action = $2)
  AND ($3::varchar = '' OR al.entity_type = $3)
ORDER BY al.created_at DESC
LIMIT $4 OFFSET $5;

-- name: CountActivityLogs :one
SELECT COUNT(*) FROM activity_logs
WHERE company_id = $1
  AND ($2::varchar = '' OR action = $2)
  AND ($3::varchar = '' OR entity_type = $3);
