-- name: GetTokenBalance :one
SELECT * FROM token_balances WHERE company_id = $1;

-- name: EnsureTokenBalance :one
INSERT INTO token_balances (company_id, balance) VALUES ($1, 0)
ON CONFLICT (company_id) DO NOTHING
RETURNING *;

-- name: DeductTokenBalance :one
UPDATE token_balances
SET balance = balance - $2,
    total_consumed = total_consumed + $2,
    updated_at = now()
WHERE company_id = $1 AND balance >= $2
RETURNING *;

-- name: CreditTokenBalance :one
UPDATE token_balances
SET balance = balance + $2,
    total_purchased = CASE WHEN $3::boolean THEN total_purchased + $2 ELSE total_purchased END,
    total_granted = CASE WHEN NOT $3::boolean THEN total_granted + $2 ELSE total_granted END,
    updated_at = now()
WHERE company_id = $1
RETURNING *;

-- name: InsertTokenTransaction :one
INSERT INTO token_transactions (company_id, user_id, type, amount, balance_after, agent_slug, description, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListTokenTransactions :many
SELECT * FROM token_transactions
WHERE company_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountTokenTransactions :one
SELECT count(*) FROM token_transactions WHERE company_id = $1;

-- name: GetTokenUsageByAgent :many
SELECT
    agent_slug,
    count(*) as request_count,
    sum(abs(amount)) as total_tokens
FROM token_transactions
WHERE company_id = $1 AND type = 'consumption' AND agent_slug IS NOT NULL
GROUP BY agent_slug
ORDER BY total_tokens DESC;

-- name: GetDailyTokenUsage :many
SELECT
    date_trunc('day', created_at)::date as day,
    sum(abs(amount)) as total_tokens,
    count(*) as request_count
FROM token_transactions
WHERE company_id = $1
    AND type = 'consumption'
    AND created_at >= now() - interval '30 days'
GROUP BY date_trunc('day', created_at)
ORDER BY day;

-- name: ListTokenPackages :many
SELECT * FROM token_packages WHERE is_active = true ORDER BY sort_order;

-- name: GetTokenPackage :one
SELECT * FROM token_packages WHERE id = $1 AND is_active = true;

-- name: ListCompaniesForFreeGrant :many
SELECT tb.company_id
FROM token_balances tb
WHERE tb.free_tier_granted_at IS NULL
   OR tb.free_tier_granted_at < date_trunc('month', now());

-- name: GrantFreeTokens :exec
UPDATE token_balances
SET balance = balance + $2,
    total_granted = total_granted + $2,
    free_tier_granted_at = now(),
    updated_at = now()
WHERE company_id = $1;

-- name: SetFreeGrantedAt :exec
UPDATE token_balances
SET free_tier_granted_at = now(), updated_at = now()
WHERE company_id = $1;

-- name: CreateTokenBalance :one
INSERT INTO token_balances (company_id, balance, total_granted, free_tier_granted_at)
VALUES ($1, $2, $2, now())
RETURNING *;
