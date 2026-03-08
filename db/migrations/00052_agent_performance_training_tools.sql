-- +goose Up
-- Phase 2: Performance + Training tools for AI agents

-- Performance review agent gets review/goal tools
UPDATE agents SET tools = tools || ARRAY[
  'list_review_cycles','get_my_performance_review',
  'create_goal','submit_self_review'
] WHERE slug = 'performance-review';

-- Manager review tool for general and performance agents
UPDATE agents SET tools = tools || ARRAY['submit_manager_review']
WHERE slug IN ('general', 'performance-review');

-- Training agent gets training/certification tools
UPDATE agents SET tools = tools || ARRAY[
  'list_trainings','list_my_certifications',
  'enroll_in_training','mark_training_complete'
] WHERE slug = 'training';

-- General agent also gets performance and training read tools
UPDATE agents SET tools = tools || ARRAY[
  'list_review_cycles','get_my_performance_review','create_goal',
  'submit_self_review','list_trainings','list_my_certifications',
  'enroll_in_training','mark_training_complete'
] WHERE slug = 'general';

-- +goose Down
UPDATE agents SET tools = array_remove(array_remove(array_remove(array_remove(
  array_remove(array_remove(array_remove(array_remove(array_remove(
    tools,
    'list_review_cycles'),'get_my_performance_review'),
    'create_goal'),'submit_self_review'),'submit_manager_review'),
    'list_trainings'),'list_my_certifications'),
    'enroll_in_training'),'mark_training_complete')
WHERE slug IN ('general', 'performance-review', 'training');
