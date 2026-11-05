-- User MFA (TOTP)

-- name: GetUserMFA :one
SELECT * FROM user_mfa WHERE user_id = $1;

-- name: StartUserMFASetup :exec
INSERT INTO user_mfa (user_id, secret_enc) VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE SET secret_enc = EXCLUDED.secret_enc, enabled_at = NULL, last_step = 0, recovery_hashes = '{}'
WHERE user_mfa.enabled_at IS NULL;

-- name: EnableUserMFA :exec
UPDATE user_mfa SET enabled_at = now(), last_step = $2, recovery_hashes = $3 WHERE user_id = $1 AND enabled_at IS NULL;

-- name: SetUserMFAStep :execrows
-- Records the last accepted time step; refuses replays of the same or older step.
UPDATE user_mfa SET last_step = $2 WHERE user_id = $1 AND last_step < $2;

-- name: SetUserMFARecovery :exec
UPDATE user_mfa SET recovery_hashes = $2 WHERE user_id = $1;

-- name: DeleteUserMFA :exec
DELETE FROM user_mfa WHERE user_id = $1;
