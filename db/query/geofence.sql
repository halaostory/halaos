-- name: CreateGeofence :one
INSERT INTO geofence_locations (
    company_id, name, address, latitude, longitude, radius_meters,
    enforce_on_clock_in, enforce_on_clock_out, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateGeofence :one
UPDATE geofence_locations SET
    name = COALESCE($3, name),
    address = COALESCE($4, address),
    latitude = $5,
    longitude = $6,
    radius_meters = $7,
    is_active = $8,
    enforce_on_clock_in = $9,
    enforce_on_clock_out = $10,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: GetGeofence :one
SELECT * FROM geofence_locations WHERE id = $1 AND company_id = $2;

-- name: ListGeofences :many
SELECT * FROM geofence_locations
WHERE company_id = $1
ORDER BY name;

-- name: ListActiveGeofences :many
SELECT * FROM geofence_locations
WHERE company_id = $1 AND is_active = true
ORDER BY name;

-- name: DeleteGeofence :exec
DELETE FROM geofence_locations WHERE id = $1 AND company_id = $2;

-- name: IsGeofenceEnabled :one
SELECT geofence_enabled FROM companies WHERE id = $1;

-- name: SetGeofenceEnabled :exec
UPDATE companies SET geofence_enabled = $2 WHERE id = $1;
