-- +goose Up

-- Add actions column to notifications table.
-- JSON structure: [{"label": "Approve", "action": "approve_leave", "params": {"id": 123}}, {"label": "View", "route": "/leaves/123"}]
ALTER TABLE notifications ADD COLUMN actions JSONB DEFAULT NULL;

-- +goose Down

ALTER TABLE notifications DROP COLUMN IF EXISTS actions;
