-- Add write tools to agents
UPDATE agents SET tools = tools || ARRAY['list_leave_types','create_leave_request','clock_in','clock_out']
WHERE slug = 'general';

UPDATE agents SET tools = tools || ARRAY['list_leave_types','create_leave_request']
WHERE slug = 'leave';

UPDATE agents SET tools = tools || ARRAY['clock_in','clock_out']
WHERE slug = 'attendance';

UPDATE agents SET tools = tools || ARRAY['create_overtime_request']
WHERE slug IN ('general', 'attendance');

-- Add safety instructions to agent system prompts
UPDATE agents SET system_prompt = system_prompt || E'\n\nWRITE ACTION SAFETY RULES:\n- Before executing any write action (leave request, clock in/out, overtime request), ALWAYS confirm with the user first.\n- Clearly state what you will do and ask "Shall I proceed?"\n- After executing, report the result (success/failure, request ID, status).\n- Never execute multiple write actions without explicit user confirmation for each.'
WHERE slug IN ('general', 'leave', 'attendance');
