-- name: ListAnnouncements :many
SELECT a.id, a.title, a.content, a.priority,
       a.published_at, a.expires_at, a.created_at,
       COALESCE(u.first_name, '') as author_first_name,
       COALESCE(u.last_name, '') as author_last_name
FROM announcements a
LEFT JOIN users u ON u.id = a.created_by
WHERE a.company_id = $1
  AND (a.published_at IS NULL OR a.published_at <= NOW())
  AND (a.expires_at IS NULL OR a.expires_at > NOW())
ORDER BY a.priority DESC, a.published_at DESC NULLS LAST
LIMIT 50;

-- name: ListAllAnnouncements :many
SELECT a.id, a.title, a.content, a.priority,
       a.target_roles, a.target_departments,
       a.published_at, a.expires_at, a.created_at,
       COALESCE(u.first_name, '') as author_first_name,
       COALESCE(u.last_name, '') as author_last_name
FROM announcements a
LEFT JOIN users u ON u.id = a.created_by
WHERE a.company_id = $1
ORDER BY a.created_at DESC
LIMIT 100;

-- name: CreateAnnouncement :one
INSERT INTO announcements (company_id, title, content, priority, target_roles, target_departments, published_at, expires_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, COALESCE($7, NOW()), $8, $9)
RETURNING *;

-- name: DeleteAnnouncement :exec
DELETE FROM announcements WHERE id = $1 AND company_id = $2;
