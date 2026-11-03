-- name: UpsertPushSubscription :exec
INSERT INTO push_subscriptions (company_id, user_id, endpoint, p256dh, auth, user_agent)
VALUES (@company_id, @user_id, @endpoint, @p256dh, @auth, @user_agent)
ON CONFLICT (endpoint) DO UPDATE SET company_id = EXCLUDED.company_id, user_id = EXCLUDED.user_id,
    p256dh = EXCLUDED.p256dh, auth = EXCLUDED.auth, user_agent = EXCLUDED.user_agent;

-- name: DeletePushSubscription :exec
DELETE FROM push_subscriptions WHERE endpoint = @endpoint AND user_id = @user_id;

-- name: DeletePushSubscriptionByEndpoint :exec
DELETE FROM push_subscriptions WHERE endpoint = @endpoint;

-- name: ListPushSubscriptions :many
SELECT * FROM push_subscriptions WHERE user_id = @user_id AND company_id = @company_id;

-- name: CountPushSubscriptions :one
SELECT COUNT(*)::int FROM push_subscriptions WHERE user_id = @user_id;

-- name: TouchPushSubscription :exec
UPDATE push_subscriptions SET last_used_at = now() WHERE id = @id;

-- name: GetVapidKeys :one
SELECT public_key, private_key FROM push_vapid_keys WHERE id = 1;

-- name: InsertVapidKeys :exec
INSERT INTO push_vapid_keys (id, public_key, private_key) VALUES (1, @public_key, @private_key)
ON CONFLICT (id) DO NOTHING;
