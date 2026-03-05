-- name: GetSSSContribution :one
SELECT * FROM sss_contribution_table
WHERE msc_min <= $1 AND msc_max >= $1
  AND effective_from <= $2
  AND (effective_to IS NULL OR effective_to >= $2)
LIMIT 1;

-- name: ListSSSTable :many
SELECT * FROM sss_contribution_table
WHERE effective_from <= $1 AND (effective_to IS NULL OR effective_to >= $1)
ORDER BY msc_min;

-- name: GetPhilHealthContribution :one
SELECT * FROM philhealth_contribution_table
WHERE salary_min <= $1 AND salary_max >= $1
  AND effective_from <= $2
  AND (effective_to IS NULL OR effective_to >= $2)
LIMIT 1;

-- name: ListPhilHealthTable :many
SELECT * FROM philhealth_contribution_table
WHERE effective_from <= $1 AND (effective_to IS NULL OR effective_to >= $1)
ORDER BY salary_min;

-- name: GetPagIBIGContribution :one
SELECT * FROM pagibig_contribution_table
WHERE salary_min <= $1 AND salary_max >= $1
  AND effective_from <= $2
  AND (effective_to IS NULL OR effective_to >= $2)
LIMIT 1;

-- name: ListPagIBIGTable :many
SELECT * FROM pagibig_contribution_table
WHERE effective_from <= $1 AND (effective_to IS NULL OR effective_to >= $1)
ORDER BY salary_min;

-- name: GetBIRTaxBracket :one
SELECT * FROM bir_tax_table
WHERE frequency = $1
  AND bracket_min <= $2
  AND (bracket_max IS NULL OR bracket_max > $2)
  AND effective_from <= $3
  AND (effective_to IS NULL OR effective_to >= $3)
LIMIT 1;

-- name: ListBIRTaxTable :many
SELECT * FROM bir_tax_table
WHERE frequency = $1
  AND effective_from <= $2 AND (effective_to IS NULL OR effective_to >= $2)
ORDER BY bracket_min;

-- name: CreateGovernmentForm :one
INSERT INTO government_forms (company_id, form_type, tax_year, period, payload)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListGovernmentForms :many
SELECT * FROM government_forms
WHERE company_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
