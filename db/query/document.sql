-- name: EnsureDefaultCategories :exec
INSERT INTO document_categories (company_id, name, slug, description, sort_order, is_system)
VALUES
    ($1, 'Hiring & Pre-Employment', 'hiring', 'Application, resume, NBI clearance, medical results, SSS/TIN/PhilHealth/Pag-IBIG', 1, true),
    ($1, 'Personal Information', 'personal', 'Birth certificate, marriage certificate, IDs, emergency contacts', 2, true),
    ($1, 'Employment Contracts', 'contracts', 'Job offer, employment contract, amendments, regularization', 3, true),
    ($1, 'Compensation & Benefits', 'compensation', 'Salary records, benefits enrollment, loan documents', 4, true),
    ($1, 'Performance', 'performance', 'Performance reviews, KPIs, commendations', 5, true),
    ($1, 'Training & Development', 'training', 'Training certificates, seminars, continuing education', 6, true),
    ($1, 'Disciplinary', 'disciplinary', 'Incident reports, NTEs, disciplinary actions', 7, true),
    ($1, 'Medical', 'medical', 'Medical certificates, annual physical exam, drug test results', 8, true),
    ($1, 'Government Compliance', 'compliance', 'SSS, PhilHealth, Pag-IBIG, BIR forms', 9, true),
    ($1, 'Separation', 'separation', 'Resignation letter, clearance, certificate of employment, final pay', 10, true)
ON CONFLICT (company_id, slug) DO NOTHING;

-- name: ListDocumentCategories :many
SELECT * FROM document_categories
WHERE company_id = $1
ORDER BY sort_order, name;

-- name: CreateDocumentCategory :one
INSERT INTO document_categories (company_id, name, slug, description, sort_order)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: Upload201Document :one
INSERT INTO employee_documents (
    company_id, employee_id, category_id, title, doc_type,
    file_name, file_path, file_size, mime_type, version,
    expiry_date, is_required, uploaded_by, notes
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;

-- name: List201Documents :many
SELECT ed.*, dc.name as category_name, dc.slug as category_slug
FROM employee_documents ed
LEFT JOIN document_categories dc ON dc.id = ed.category_id
WHERE ed.company_id = @company_id
  AND ed.employee_id = @employee_id
  AND (@category_id = 0 OR ed.category_id = @category_id)
  AND (@status = '' OR ed.status = @status)
ORDER BY dc.sort_order, ed.created_at DESC;

-- name: Get201Document :one
SELECT ed.*, dc.name as category_name, dc.slug as category_slug
FROM employee_documents ed
LEFT JOIN document_categories dc ON dc.id = ed.category_id
WHERE ed.id = $1 AND ed.company_id = $2;

-- name: Update201Document :one
UPDATE employee_documents SET
    title = COALESCE($3, title),
    category_id = $4,
    expiry_date = $5,
    notes = $6,
    status = COALESCE($7, status),
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: Delete201Document :exec
DELETE FROM employee_documents WHERE id = $1 AND company_id = $2;

-- name: List201ExpiringDocuments :many
SELECT ed.*, dc.name as category_name,
       e.employee_no, e.first_name, e.last_name
FROM employee_documents ed
LEFT JOIN document_categories dc ON dc.id = ed.category_id
JOIN employees e ON e.id = ed.employee_id
WHERE ed.company_id = $1
  AND ed.status = 'active'
  AND ed.expiry_date IS NOT NULL
  AND ed.expiry_date <= CURRENT_DATE + INTERVAL '30 days'
ORDER BY ed.expiry_date;

-- name: MarkExpiredDocuments :exec
UPDATE employee_documents SET
    status = 'expired',
    updated_at = NOW()
WHERE status = 'active'
  AND expiry_date IS NOT NULL
  AND expiry_date < CURRENT_DATE;

-- name: GetEmployee201Stats :one
SELECT
    COUNT(*) as total_documents,
    COUNT(*) FILTER (WHERE status = 'active') as active_documents,
    COUNT(*) FILTER (WHERE status = 'expired') as expired_documents,
    COUNT(*) FILTER (WHERE expiry_date IS NOT NULL AND expiry_date <= CURRENT_DATE + INTERVAL '30 days' AND status = 'active') as expiring_soon
FROM employee_documents
WHERE company_id = $1 AND employee_id = $2;

-- name: ListDocumentRequirements :many
SELECT dr.*, dc.name as category_name
FROM document_requirements dr
JOIN document_categories dc ON dc.id = dr.category_id
WHERE dr.company_id = $1
ORDER BY dc.sort_order, dr.document_name;

-- name: CreateDocumentRequirement :one
INSERT INTO document_requirements (company_id, category_id, document_name, is_required, applies_to, expiry_months)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: DeleteDocumentRequirement :exec
DELETE FROM document_requirements WHERE id = $1 AND company_id = $2;

-- name: GetComplianceChecklist :many
SELECT dr.id as requirement_id, dr.document_name, dr.is_required, dc.name as category_name,
       CASE WHEN ed.id IS NOT NULL THEN true ELSE false END as is_fulfilled,
       ed.id as document_id, ed.expiry_date, ed.status as document_status
FROM document_requirements dr
JOIN document_categories dc ON dc.id = dr.category_id
LEFT JOIN employee_documents ed ON ed.company_id = dr.company_id
    AND ed.employee_id = $2
    AND ed.category_id = dr.category_id
    AND COALESCE(ed.title, ed.doc_type) = dr.document_name
    AND ed.status = 'active'
WHERE dr.company_id = $1
ORDER BY dc.sort_order, dr.document_name;
