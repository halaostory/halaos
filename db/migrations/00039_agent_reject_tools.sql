-- +goose Up
-- Add reject tools to agents (complement to approve tools)

UPDATE agents SET tools = tools || ARRAY['reject_leave_request','reject_overtime_request']
WHERE slug = 'general';

UPDATE agents SET tools = tools || ARRAY['reject_leave_request']
WHERE slug = 'leave';

UPDATE agents SET tools = tools || ARRAY['reject_overtime_request']
WHERE slug = 'attendance';

-- +goose Down
UPDATE agents SET tools = array_remove(array_remove(tools, 'reject_leave_request'), 'reject_overtime_request')
WHERE slug IN ('general', 'leave', 'attendance');
