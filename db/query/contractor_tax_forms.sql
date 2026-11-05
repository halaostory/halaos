-- name: CreateContractorTaxForm :one
INSERT INTO contractor_tax_forms (company_id, contractor_id, token_hash, expires_at, created_by)
VALUES (@company_id, @contractor_id, @token_hash, @expires_at, @created_by)
RETURNING *;

-- name: RevokePendingContractorTaxForms :exec
UPDATE contractor_tax_forms SET status = 'revoked'
WHERE contractor_id = @contractor_id AND company_id = @company_id AND status = 'pending';

-- name: LatestContractorTaxForm :one
SELECT * FROM contractor_tax_forms
WHERE contractor_id = @contractor_id AND company_id = @company_id
ORDER BY created_at DESC LIMIT 1;

-- name: GetContractorTaxFormByToken :one
SELECT f.*, c.name AS contractor_name, c.kind AS contractor_kind, c.tin AS current_tin, c.address AS current_address,
       c.is_vat_registered AS current_vat, co.name AS company_name, co.country AS company_country
FROM contractor_tax_forms f
JOIN contractors c ON c.id = f.contractor_id
JOIN companies co ON co.id = f.company_id
WHERE f.token_hash = @token_hash;

-- name: SubmitContractorTaxForm :one
UPDATE contractor_tax_forms SET status = 'submitted', tax_country = @tax_country, entity_type = @entity_type,
    tin = @tin, address = @address, is_vat_registered = @is_vat_registered, declaration = @declaration,
    signer_name = @signer_name, signer_ip = @signer_ip, signer_agent = @signer_agent, submitted_at = now()
WHERE id = @id AND status = 'pending' AND expires_at > now()
RETURNING *;

-- name: ApplyContractorTaxInfo :exec
UPDATE contractors SET tin = @tin, address = @address, is_vat_registered = @is_vat_registered,
    bank_name = COALESCE(NULLIF(sqlc.narg('bank_name')::varchar, ''), bank_name),
    bank_account_no = COALESCE(NULLIF(sqlc.narg('bank_account_no')::varchar, ''), bank_account_no),
    bank_account_name = COALESCE(NULLIF(sqlc.narg('bank_account_name')::varchar, ''), bank_account_name),
    updated_at = now()
WHERE id = @id AND company_id = @company_id;
