-- +goose Up
-- Add employee creation and search tools to AI agents

UPDATE agents SET tools = tools || ARRAY[
  'search_departments','search_positions','create_employee'
] WHERE slug = 'general';

-- +goose Down
UPDATE agents SET tools = array_remove(array_remove(array_remove(
  tools,
  'search_departments'), 'search_positions'), 'create_employee')
WHERE slug = 'general';
