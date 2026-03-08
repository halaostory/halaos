-- +goose Up
-- Phase 5: Clearance + Final Pay tools for AI agents

-- Compliance agent gets clearance/final pay tools
UPDATE agents SET tools = tools || ARRAY[
  'get_clearance_status','update_clearance_item',
  'query_final_pay_components','create_final_pay','complete_clearance'
] WHERE slug = 'compliance';

-- General agent also gets these tools
UPDATE agents SET tools = tools || ARRAY[
  'get_clearance_status','update_clearance_item',
  'query_final_pay_components','create_final_pay','complete_clearance'
] WHERE slug = 'general';

-- +goose Down
UPDATE agents SET tools = array_remove(array_remove(array_remove(array_remove(
  array_remove(tools,
    'get_clearance_status'),'update_clearance_item'),
    'query_final_pay_components'),'create_final_pay'),'complete_clearance')
WHERE slug IN ('compliance', 'general');
