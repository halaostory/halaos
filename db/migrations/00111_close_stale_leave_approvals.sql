-- Leave approved/rejected/cancelled from its own page left its approval
-- entry pending, so it stayed in approvers' inboxes. Close those entries.

-- +goose Up
UPDATE approval_workflows aw SET status = lr.status, decided_at = COALESCE(lr.approved_at, lr.updated_at, NOW())
FROM leave_requests lr
WHERE aw.entity_type = 'leave_request' AND aw.entity_id = lr.id AND aw.company_id = lr.company_id
  AND aw.status = 'pending' AND lr.status <> 'pending';

-- +goose Down
SELECT 1;
