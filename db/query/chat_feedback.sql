-- name: InsertChatFeedback :one
INSERT INTO chat_message_feedback (message_id, company_id, user_id, rating, comment)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (message_id, user_id) DO UPDATE SET rating = $4, comment = $5
RETURNING *;

-- name: GetFeedbackByMessage :one
SELECT * FROM chat_message_feedback WHERE message_id = $1 AND user_id = $2;

-- name: GetFeedbackStats :many
SELECT rating, COUNT(*) as count
FROM chat_message_feedback
WHERE company_id = $1
GROUP BY rating;

-- name: ListRecentFeedback :many
SELECT f.*, m.content as message_content, m.role as message_role
FROM chat_message_feedback f
JOIN chat_messages m ON m.id = f.message_id
WHERE f.company_id = $1
ORDER BY f.created_at DESC
LIMIT $2 OFFSET $3;
