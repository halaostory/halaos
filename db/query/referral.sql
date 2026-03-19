-- name: GetCompanyReferralCode :one
SELECT referral_code FROM companies WHERE id = $1;

-- name: SetCompanyReferralCode :exec
UPDATE companies SET referral_code = $2 WHERE id = $1;

-- name: GetCompanyByReferralCode :one
SELECT id, name, referral_code FROM companies WHERE referral_code = $1;

-- name: SetReferredByCode :exec
UPDATE companies SET referred_by_code = $2 WHERE id = $1;

-- name: CreateReferralEvent :one
INSERT INTO referral_events (referrer_company_id, referred_company_id, referral_code, status)
VALUES ($1, $2, $3, 'pending')
RETURNING *;

-- name: ActivateReferralEvent :exec
UPDATE referral_events
SET status = 'activated', activated_at = NOW(), reward_type = $2
WHERE referred_company_id = $1 AND status = 'pending';

-- name: ListReferralsByCompany :many
SELECT re.*, c.name as referred_company_name
FROM referral_events re
JOIN companies c ON c.id = re.referred_company_id
WHERE re.referrer_company_id = $1
ORDER BY re.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountReferralsByCompany :one
SELECT COUNT(*) FROM referral_events
WHERE referrer_company_id = $1;

-- name: CountActiveReferralsByCompany :one
SELECT COUNT(*) FROM referral_events
WHERE referrer_company_id = $1 AND status = 'activated';
