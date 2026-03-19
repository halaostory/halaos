-- name: CreateUser :one
INSERT INTO users (company_id, email, password_hash, first_name, last_name, role, status)
VALUES ($1, $2, $3, $4, $5, $6, 'active')
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 AND status = 'active' LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByCompanyAndEmail :one
SELECT * FROM users WHERE company_id = $1 AND email = $2 LIMIT 1;

-- name: UpdateLastLogin :exec
UPDATE users SET last_login_at = NOW() WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2 AND company_id = $3;

-- name: UpdateUserProfile :one
UPDATE users SET
    first_name = COALESCE($2, first_name),
    last_name = COALESCE($3, last_name),
    avatar_url = COALESCE($4, avatar_url),
    locale = COALESCE($5, locale),
    updated_at = NOW()
WHERE id = $1 AND company_id = $6
RETURNING *;

-- name: UpdateUserRole :exec
UPDATE users SET role = $2, updated_at = NOW() WHERE id = $1 AND company_id = $3;

-- name: ListUsersByCompany :many
SELECT * FROM users WHERE company_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: CountUsersByCompany :one
SELECT COUNT(*) FROM users WHERE company_id = $1;

-- name: UpdateUserStatus :exec
UPDATE users SET status = $2, updated_at = NOW() WHERE id = $1 AND company_id = $3;

-- name: AdminResetPassword :exec
UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1 AND company_id = $3;

-- name: SetVerificationToken :exec
UPDATE users SET verification_token = $2, verification_token_expires_at = $3, updated_at = NOW()
WHERE id = $1;

-- name: GetUserByVerificationToken :one
SELECT * FROM users WHERE verification_token = $1 AND verification_token_expires_at > NOW();

-- name: MarkEmailVerified :exec
UPDATE users SET email_verified = true, verification_token = NULL, verification_token_expires_at = NULL, updated_at = NOW()
WHERE id = $1;
