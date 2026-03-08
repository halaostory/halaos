-- +goose Up
-- Add onboard_employee tool to general and onboarding agents

UPDATE agents SET tools = tools || ARRAY['onboard_employee']
WHERE slug = 'general';

UPDATE agents SET tools = tools || ARRAY['onboard_employee']
WHERE slug = 'onboarding';

-- +goose Down
UPDATE agents SET tools = array_remove(tools, 'onboard_employee')
WHERE slug IN ('general', 'onboarding');
