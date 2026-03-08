-- +goose Up
-- Phase 6: Schedule tools for AI agents

-- Attendance agent gets schedule tools
UPDATE agents SET tools = tools || ARRAY[
  'list_schedule_templates','get_employee_schedule','assign_schedule'
] WHERE slug = 'attendance';

-- General agent also gets schedule tools
UPDATE agents SET tools = tools || ARRAY[
  'list_schedule_templates','get_employee_schedule','assign_schedule'
] WHERE slug = 'general';

-- +goose Down
UPDATE agents SET tools = array_remove(array_remove(array_remove(
  tools,
  'list_schedule_templates'),'get_employee_schedule'),'assign_schedule')
WHERE slug IN ('attendance', 'general');
