-- +goose Up
-- Grant the Warp-style lifecycle / government filing / device tools to the
-- relevant system agents. Agents with an empty tool list already get every
-- tool, so they are left alone.

UPDATE agents SET tools = ARRAY(SELECT DISTINCT unnest(tools || ARRAY['get_hr_inbox', 'get_employee_lifecycle']))
WHERE slug = 'general' AND company_id IS NULL AND cardinality(tools) > 0;

UPDATE agents SET tools = ARRAY(SELECT DISTINCT unnest(tools || ARRAY[
    'get_hr_inbox', 'get_employee_lifecycle', 'hire_employee', 'terminate_employee',
    'send_onboarding_invite', 'send_document_for_signature', 'lock_device']))
WHERE slug = 'onboarding' AND company_id IS NULL AND cardinality(tools) > 0;

UPDATE agents SET tools = ARRAY(SELECT DISTINCT unnest(tools || ARRAY['get_hr_inbox', 'list_gov_filings', 'prepare_gov_filing']))
WHERE slug = 'compliance' AND company_id IS NULL AND cardinality(tools) > 0;

UPDATE agents SET tools = ARRAY(SELECT DISTINCT unnest(tools || ARRAY['list_gov_filings']))
WHERE slug = 'payroll' AND company_id IS NULL AND cardinality(tools) > 0;

-- +goose Down
UPDATE agents SET tools = ARRAY(
    SELECT t FROM unnest(tools) t WHERE t NOT IN (
        'get_hr_inbox', 'get_employee_lifecycle', 'hire_employee', 'terminate_employee',
        'send_onboarding_invite', 'send_document_for_signature', 'lock_device',
        'list_gov_filings', 'prepare_gov_filing'))
WHERE company_id IS NULL AND slug IN ('general', 'onboarding', 'compliance', 'payroll');
