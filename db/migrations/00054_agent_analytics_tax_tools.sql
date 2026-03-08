-- +goose Up
-- Phase 4: Analytics + Tax filing tools for AI agents

-- Compliance agent gets tax tools
UPDATE agents SET tools = tools || ARRAY[
  'query_tax_filing_status','create_tax_filing_record'
] WHERE slug = 'compliance';

-- General agent gets analytics + tax tools
UPDATE agents SET tools = tools || ARRAY[
  'query_company_analytics','query_headcount_trend',
  'query_leave_utilization','query_tax_filing_status',
  'create_tax_filing_record'
] WHERE slug = 'general';

-- +goose Down
UPDATE agents SET tools = array_remove(array_remove(array_remove(array_remove(
  array_remove(tools,
    'query_company_analytics'),'query_headcount_trend'),
    'query_leave_utilization'),'query_tax_filing_status'),
    'create_tax_filing_record')
WHERE slug IN ('compliance', 'general');
