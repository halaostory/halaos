-- +goose Up
-- Add P0 write tools to agents

-- General agent: expense, self-service, reports, approvals
UPDATE agents SET tools = tools || ARRAY[
    'list_expense_categories','create_expense_claim',
    'update_employee_profile',
    'generate_attendance_report',
    'list_pending_approvals','approve_leave_request','approve_overtime_request'
]
WHERE slug = 'general';

-- Leave agent: approval tools
UPDATE agents SET tools = tools || ARRAY['list_pending_approvals','approve_leave_request']
WHERE slug = 'leave';

-- Attendance agent: reports + overtime approval
UPDATE agents SET tools = tools || ARRAY['generate_attendance_report','list_pending_approvals','approve_overtime_request']
WHERE slug = 'attendance';

-- +goose Down
UPDATE agents SET tools = array_remove(array_remove(array_remove(array_remove(array_remove(array_remove(array_remove(tools,
    'list_expense_categories'), 'create_expense_claim'),
    'update_employee_profile'),
    'generate_attendance_report'),
    'list_pending_approvals'), 'approve_leave_request'), 'approve_overtime_request')
WHERE slug IN ('general', 'leave', 'attendance');
