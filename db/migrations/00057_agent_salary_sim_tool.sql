-- +goose Up
UPDATE agents SET tools = array_append(tools, 'simulate_salary')
WHERE slug = 'general'
  AND NOT ('simulate_salary' = ANY(tools));

-- +goose Down
UPDATE agents SET tools = array_remove(tools, 'simulate_salary')
WHERE slug = 'general';
