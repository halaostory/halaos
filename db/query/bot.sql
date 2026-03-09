-- ========== bot_configs ==========

-- name: GetBotConfig :one
SELECT * FROM bot_configs
WHERE company_id = $1 AND platform = $2;

-- name: ListBotConfigs :many
SELECT * FROM bot_configs
WHERE company_id = $1
ORDER BY platform;

-- name: ListActiveBotConfigs :many
SELECT * FROM bot_configs
WHERE is_active = true AND platform = $1;

-- name: UpsertBotConfig :one
INSERT INTO bot_configs (company_id, platform, bot_token, bot_username, is_active, webhook_url, mode)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (company_id, platform)
DO UPDATE SET
    bot_token = EXCLUDED.bot_token,
    bot_username = EXCLUDED.bot_username,
    is_active = EXCLUDED.is_active,
    webhook_url = EXCLUDED.webhook_url,
    mode = EXCLUDED.mode,
    updated_at = NOW()
RETURNING *;

-- name: DeleteBotConfig :exec
DELETE FROM bot_configs
WHERE company_id = $1 AND platform = $2;

-- ========== bot_user_links ==========

-- name: GetBotUserLinkByPlatformUser :one
SELECT bul.*, u.email AS user_email, e.first_name, e.last_name, e.id AS employee_id
FROM bot_user_links bul
JOIN users u ON u.id = bul.user_id
LEFT JOIN employees e ON e.company_id = bul.company_id AND e.email = u.email
WHERE bul.platform = $1 AND bul.platform_user_id = $2 AND bul.verified_at IS NOT NULL;

-- name: GetBotUserLinkByUserID :one
SELECT * FROM bot_user_links
WHERE user_id = $1 AND platform = $2;

-- name: GetBotUserLinkByCode :one
SELECT * FROM bot_user_links
WHERE link_code = $1 AND link_code_exp > NOW();

-- name: CreateBotUserLink :one
INSERT INTO bot_user_links (platform, user_id, company_id, link_code, link_code_exp)
VALUES ($1, $2, $3, $4, NOW() + INTERVAL '15 minutes')
RETURNING *;

-- name: VerifyBotUserLink :one
UPDATE bot_user_links SET
    platform_user_id = $2,
    verified_at = NOW(),
    link_code = NULL,
    link_code_exp = NULL,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateBotUserLocale :exec
UPDATE bot_user_links SET locale = $2, updated_at = NOW()
WHERE id = $1;

-- name: UpdateBotUserActiveSession :exec
UPDATE bot_user_links SET active_session_id = $2, updated_at = NOW()
WHERE id = $1;

-- name: UnlinkBotUser :exec
DELETE FROM bot_user_links
WHERE user_id = $1 AND platform = $2;

-- name: RegenerateLinkCode :one
UPDATE bot_user_links SET
    link_code = $2,
    link_code_exp = NOW() + INTERVAL '15 minutes',
    updated_at = NOW()
WHERE user_id = $1 AND platform = 'telegram'
RETURNING *;

-- name: ListBotUserLinks :many
SELECT * FROM bot_user_links
WHERE company_id = $1
ORDER BY created_at DESC;
