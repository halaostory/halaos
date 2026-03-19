-- name: InsertDripEmail :exec
INSERT INTO drip_emails (company_id, step) VALUES ($1, $2)
ON CONFLICT (company_id, step) DO NOTHING;

-- name: GetDripEmailsSent :many
SELECT step FROM drip_emails WHERE company_id = $1;

-- name: ListCompaniesForDrip :many
-- Find companies that registered within the last 30 days and haven't received a given drip step.
-- Returns the admin user's email and name for sending.
SELECT c.id AS company_id,
       c.name AS company_name,
       c.created_at AS registered_at,
       u.email AS admin_email,
       u.first_name AS admin_first_name
FROM companies c
JOIN users u ON u.company_id = c.id AND u.role IN ('super_admin', 'admin')
WHERE c.created_at >= now() - INTERVAL '30 days'
  AND c.email_verified = true
  AND NOT EXISTS (
      SELECT 1 FROM drip_emails d WHERE d.company_id = c.id AND d.step = $1
  )
ORDER BY c.created_at ASC
LIMIT 100;
