-- +goose Up

-- Geofence Locations
CREATE TABLE geofence_locations (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name VARCHAR(200) NOT NULL,
    address TEXT,
    latitude NUMERIC(10, 7) NOT NULL,
    longitude NUMERIC(10, 7) NOT NULL,
    radius_meters INT NOT NULL DEFAULT 200,
    is_active BOOLEAN NOT NULL DEFAULT true,
    enforce_on_clock_in BOOLEAN NOT NULL DEFAULT true,
    enforce_on_clock_out BOOLEAN NOT NULL DEFAULT false,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_geofence_company ON geofence_locations(company_id, is_active);

-- Company setting to enable/disable geofencing globally
ALTER TABLE companies ADD COLUMN IF NOT EXISTS geofence_enabled BOOLEAN NOT NULL DEFAULT false;

-- Track geofence validation result on attendance logs
ALTER TABLE attendance_logs ADD COLUMN IF NOT EXISTS clock_in_geofence_id BIGINT REFERENCES geofence_locations(id);
ALTER TABLE attendance_logs ADD COLUMN IF NOT EXISTS clock_in_geofence_status VARCHAR(20) DEFAULT 'not_checked'; -- not_checked, inside, outside, bypassed
ALTER TABLE attendance_logs ADD COLUMN IF NOT EXISTS clock_out_geofence_id BIGINT REFERENCES geofence_locations(id);
ALTER TABLE attendance_logs ADD COLUMN IF NOT EXISTS clock_out_geofence_status VARCHAR(20) DEFAULT 'not_checked';

-- +goose Down
ALTER TABLE attendance_logs DROP COLUMN IF EXISTS clock_out_geofence_status;
ALTER TABLE attendance_logs DROP COLUMN IF EXISTS clock_out_geofence_id;
ALTER TABLE attendance_logs DROP COLUMN IF EXISTS clock_in_geofence_status;
ALTER TABLE attendance_logs DROP COLUMN IF EXISTS clock_in_geofence_id;
ALTER TABLE companies DROP COLUMN IF EXISTS geofence_enabled;
DROP TABLE IF EXISTS geofence_locations;
