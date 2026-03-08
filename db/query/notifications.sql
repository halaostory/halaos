-- name: CreateNotification :one
INSERT INTO notifications (company_id, user_id, title, message, category, entity_type, entity_id, actions)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListNotifications :many
SELECT * FROM notifications
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountUnreadNotifications :one
SELECT COUNT(*) FROM notifications
WHERE user_id = $1 AND is_read = false;

-- name: MarkNotificationRead :exec
UPDATE notifications SET is_read = true, read_at = NOW()
WHERE id = $1 AND user_id = $2;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications SET is_read = true, read_at = NOW()
WHERE user_id = $1 AND is_read = false;

-- name: DeleteNotification :exec
DELETE FROM notifications WHERE id = $1 AND user_id = $2;

-- name: GetNotificationByID :one
SELECT * FROM notifications
WHERE id = $1 AND user_id = $2;

-- name: UpdateNotificationActions :exec
UPDATE notifications SET actions = $3
WHERE id = $1 AND user_id = $2;
