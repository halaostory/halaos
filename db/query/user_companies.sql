-- name: GetUserCompanies :many
SELECT c.id, c.name, c.country, c.timezone, c.currency, c.logo_url
FROM user_companies uc
JOIN companies c ON c.id = uc.company_id
WHERE uc.user_id = $1
ORDER BY c.name;

-- name: GetUserCompanyMembership :one
SELECT user_id, company_id, role, joined_at
FROM user_companies
WHERE user_id = $1 AND company_id = $2;

-- name: UpdateUserActiveCompany :exec
UPDATE users SET company_id = $2, updated_at = NOW()
WHERE id = $1;

-- name: InsertUserCompany :exec
INSERT INTO user_companies (user_id, company_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, company_id) DO NOTHING;
