-- Self-onboarding portal & e-signatures.

-- name: CreateOnboardingInvite :one
INSERT INTO onboarding_invites (company_id, employee_id, token_hash, email, expires_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: RevokeOpenOnboardingInvites :exec
UPDATE onboarding_invites SET status = 'revoked'
WHERE employee_id = $1 AND company_id = $2 AND status IN ('pending', 'opened');

-- name: GetOnboardingInviteByToken :one
SELECT * FROM onboarding_invites WHERE token_hash = $1;

-- name: GetLatestOnboardingInvite :one
SELECT * FROM onboarding_invites WHERE employee_id = $1 AND company_id = $2
ORDER BY created_at DESC LIMIT 1;

-- name: MarkOnboardingInviteOpened :exec
UPDATE onboarding_invites SET status = 'opened', opened_at = COALESCE(opened_at, NOW())
WHERE id = $1 AND status = 'pending';

-- name: MarkOnboardingInviteSubmitted :exec
UPDATE onboarding_invites SET status = 'submitted', submitted_at = NOW()
WHERE id = $1 AND status IN ('pending', 'opened');

-- name: CreateDocumentTemplate :one
INSERT INTO document_templates (company_id, kind, name, body, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateDocumentTemplate :one
UPDATE document_templates SET kind = $3, name = $4, body = $5, is_active = $6, updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: ListDocumentTemplates :many
SELECT * FROM document_templates WHERE company_id = $1 ORDER BY kind, name;

-- name: GetDocumentTemplate :one
SELECT * FROM document_templates WHERE id = $1 AND company_id = $2;

-- name: CreateSignatureRequest :one
INSERT INTO signature_requests (
    company_id, employee_id, template_id, kind, title, body, body_sha256,
    signer_name, signer_email, token_hash, expires_at, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetSignatureRequestByToken :one
SELECT * FROM signature_requests WHERE token_hash = $1;

-- name: GetSignatureRequest :one
SELECT * FROM signature_requests WHERE id = $1 AND company_id = $2;

-- name: ListSignatureRequests :many
SELECT id, company_id, employee_id, template_id, kind, title, signer_name, signer_email, status,
       signed_name, signed_at, declined_reason, viewed_at, expires_at, created_at,
       (signed_pdf_path IS NOT NULL)::boolean AS has_pdf
FROM signature_requests
WHERE company_id = $1 AND (sqlc.arg(employee_id)::bigint = 0 OR employee_id = sqlc.arg(employee_id)::bigint)
ORDER BY created_at DESC
LIMIT 200;

-- name: ListPendingSignaturesForEmployee :many
SELECT id, title, kind, status, expires_at FROM signature_requests
WHERE employee_id = $1 AND company_id = $2 AND status IN ('pending', 'viewed') AND expires_at > NOW()
ORDER BY created_at;

-- name: MarkSignatureViewed :exec
UPDATE signature_requests SET status = 'viewed', viewed_at = COALESCE(viewed_at, NOW())
WHERE id = $1 AND status = 'pending';

-- name: SignSignatureRequest :one
UPDATE signature_requests SET
    status = 'signed', signed_name = $2, signature_image = $3, signed_at = NOW(),
    signed_ip = $4, signed_user_agent = $5
WHERE id = $1 AND status IN ('pending', 'viewed') AND expires_at > NOW()
RETURNING *;

-- name: SetSignaturePDF :exec
UPDATE signature_requests SET signed_pdf_path = $2, signed_pdf_sha256 = $3 WHERE id = $1;

-- name: DeclineSignatureRequest :one
UPDATE signature_requests SET status = 'declined', declined_reason = $2
WHERE id = $1 AND status IN ('pending', 'viewed')
RETURNING *;

-- name: VoidSignatureRequest :one
UPDATE signature_requests SET status = 'voided'
WHERE id = $1 AND company_id = $2 AND status IN ('pending', 'viewed')
RETURNING *;

-- name: GetEmployeeForDocument :one
SELECT e.id, e.employee_no, e.first_name, e.last_name, e.middle_name, e.email, e.hire_date, e.employment_type,
       COALESCE(d.name, '')::text AS department_name, COALESCE(p.title, '')::text AS position_title,
       COALESCE(m.first_name || ' ' || m.last_name, '')::text AS manager_name,
       COALESCE((SELECT es.basic_salary::text FROM employee_salaries es WHERE es.employee_id = e.id ORDER BY es.effective_from DESC LIMIT 1), '')::text AS basic_salary,
       c.name AS company_name, COALESCE(c.address, '')::text AS company_address, c.country AS company_country
FROM employees e
JOIN companies c ON c.id = e.company_id
LEFT JOIN departments d ON d.id = e.department_id
LEFT JOIN positions p ON p.id = e.position_id
LEFT JOIN employees m ON m.id = e.manager_id
WHERE e.id = $1 AND e.company_id = $2;

-- name: UpdateEmployeeSelfService :exec
UPDATE employees SET
    phone = COALESCE(sqlc.narg(phone), phone),
    birth_date = COALESCE(sqlc.narg(birth_date)::date, birth_date),
    gender = COALESCE(sqlc.narg(gender), gender),
    civil_status = COALESCE(sqlc.narg(civil_status), civil_status),
    nationality = COALESCE(sqlc.narg(nationality), nationality),
    updated_at = NOW()
WHERE id = sqlc.arg(id) AND company_id = sqlc.arg(company_id);
