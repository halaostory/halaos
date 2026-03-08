-- +goose Up
-- Phase 1: Loan + Benefit + Leave Encashment tools for AI agents

-- General agent gets all loan/benefit/encashment tools
UPDATE agents SET tools = tools || ARRAY[
  'query_my_loans','list_pending_loans','approve_loan','reject_loan',
  'query_loan_eligibility','query_my_benefits',
  'list_pending_benefit_claims','approve_benefit_claim','reject_benefit_claim',
  'query_encashment_eligibility','approve_leave_encashment'
] WHERE slug = 'general';

-- Leave agent gets encashment tools
UPDATE agents SET tools = tools || ARRAY[
  'query_encashment_eligibility','approve_leave_encashment'
] WHERE slug = 'leave';

-- +goose Down
UPDATE agents SET tools = array_remove(array_remove(array_remove(array_remove(array_remove(
  array_remove(array_remove(array_remove(array_remove(array_remove(array_remove(
    tools,
    'query_my_loans'),'list_pending_loans'),'approve_loan'),'reject_loan'),
    'query_loan_eligibility'),'query_my_benefits'),
    'list_pending_benefit_claims'),'approve_benefit_claim'),'reject_benefit_claim'),
    'query_encashment_eligibility'),'approve_leave_encashment')
WHERE slug IN ('general', 'leave');
