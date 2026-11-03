-- name: ListPerks :many
-- Employees see active, unexpired perks; admins (all_rows) see everything.
SELECT * FROM company_perks
WHERE company_id = @company_id
  AND (@all_rows::bool OR (active AND (expires_on IS NULL OR expires_on >= CURRENT_DATE)))
ORDER BY active DESC, category, title;

-- name: CreatePerk :one
INSERT INTO company_perks (company_id, title, partner, category, description, offer, code, url, expires_on, active, created_by)
VALUES (@company_id, @title, @partner, @category, @description, @offer, @code, @url, @expires_on, @active, @created_by)
RETURNING *;

-- name: UpdatePerk :one
UPDATE company_perks SET title = @title, partner = @partner, category = @category, description = @description,
    offer = @offer, code = @code, url = @url, expires_on = @expires_on, active = @active, updated_at = now()
WHERE id = @id AND company_id = @company_id
RETURNING *;

-- name: DeletePerk :exec
DELETE FROM company_perks WHERE id = @id AND company_id = @company_id;

-- name: ClaimPerk :one
-- Counts a claim and reveals the code/link (only for active perks).
UPDATE company_perks SET claims = claims + 1
WHERE id = @id AND company_id = @company_id AND active AND (expires_on IS NULL OR expires_on >= CURRENT_DATE)
RETURNING code, url;
