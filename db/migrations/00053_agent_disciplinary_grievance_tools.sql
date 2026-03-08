-- +goose Up
-- Phase 3: Disciplinary + Grievance tools for AI agents

-- Compliance agent gets all disciplinary and grievance tools
UPDATE agents SET tools = tools || ARRAY[
  'query_employee_disciplinary','create_disciplinary_incident',
  'create_disciplinary_action','list_recent_incidents',
  'query_grievance_summary','get_grievance_detail','resolve_grievance'
] WHERE slug = 'compliance';

-- General agent also gets these tools
UPDATE agents SET tools = tools || ARRAY[
  'query_employee_disciplinary','create_disciplinary_incident',
  'create_disciplinary_action','list_recent_incidents',
  'query_grievance_summary','get_grievance_detail','resolve_grievance'
] WHERE slug = 'general';

-- +goose Down
UPDATE agents SET tools = array_remove(array_remove(array_remove(array_remove(
  array_remove(array_remove(array_remove(
    tools,
    'query_employee_disciplinary'),'create_disciplinary_incident'),
    'create_disciplinary_action'),'list_recent_incidents'),
    'query_grievance_summary'),'get_grievance_detail'),'resolve_grievance')
WHERE slug IN ('compliance', 'general');
