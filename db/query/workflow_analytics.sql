-- name: GetAvgApprovalTimeByType :many
SELECT
    entity_type,
    ROUND(AVG(EXTRACT(EPOCH FROM (decided_at - created_at)) / 3600)::numeric, 1) as avg_hours
FROM approval_workflows
WHERE company_id = $1
  AND status IN ('approved', 'rejected')
  AND decided_at IS NOT NULL
GROUP BY entity_type
ORDER BY entity_type;

-- name: GetAutoApprovalStats :one
SELECT
    COALESCE(SUM(CASE WHEN action = 'auto_approved' THEN 1 ELSE 0 END), 0) as auto_approved,
    COALESCE(SUM(CASE WHEN action = 'auto_rejected' THEN 1 ELSE 0 END), 0) as auto_rejected,
    COUNT(*) as total_executions
FROM workflow_rule_executions
WHERE company_id = $1
  AND created_at >= $2;

-- name: GetApprovalVolumeByDay :many
SELECT
    created_at::date as day,
    COUNT(*) as total,
    COALESCE(SUM(CASE WHEN status = 'approved' THEN 1 ELSE 0 END), 0) as approved,
    COALESCE(SUM(CASE WHEN status = 'rejected' THEN 1 ELSE 0 END), 0) as rejected,
    COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0) as pending
FROM approval_workflows
WHERE company_id = $1
  AND created_at >= $2
GROUP BY created_at::date
ORDER BY day;

-- name: GetPendingAgeDistribution :many
SELECT
    CASE
        WHEN EXTRACT(EPOCH FROM (NOW() - created_at)) / 3600 < 1 THEN '<1h'
        WHEN EXTRACT(EPOCH FROM (NOW() - created_at)) / 3600 < 6 THEN '1-6h'
        WHEN EXTRACT(EPOCH FROM (NOW() - created_at)) / 3600 < 24 THEN '6-24h'
        WHEN EXTRACT(EPOCH FROM (NOW() - created_at)) / 3600 < 48 THEN '24-48h'
        ELSE '48h+'
    END as bucket,
    COUNT(*) as count
FROM approval_workflows
WHERE company_id = $1 AND status = 'pending'
GROUP BY bucket
ORDER BY
    CASE bucket
        WHEN '<1h' THEN 1
        WHEN '1-6h' THEN 2
        WHEN '6-24h' THEN 3
        WHEN '24-48h' THEN 4
        WHEN '48h+' THEN 5
    END;

-- name: GetSLAComplianceRate :one
SELECT
    COUNT(*) as total,
    COALESCE(SUM(CASE WHEN sla_deadline IS NOT NULL AND decided_at <= sla_deadline THEN 1 ELSE 0 END), 0) as within_sla,
    COALESCE(SUM(CASE WHEN sla_deadline IS NOT NULL AND (decided_at > sla_deadline OR (decided_at IS NULL AND NOW() > sla_deadline)) THEN 1 ELSE 0 END), 0) as overdue
FROM approval_workflows
WHERE company_id = $1
  AND created_at >= $2;

-- name: GetWorkflowSummaryStats :one
SELECT
    COUNT(*) as total_approvals,
    COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0) as pending_count,
    COALESCE(SUM(CASE WHEN status = 'approved' THEN 1 ELSE 0 END), 0) as approved_count,
    COALESCE(SUM(CASE WHEN status = 'rejected' THEN 1 ELSE 0 END), 0) as rejected_count
FROM approval_workflows
WHERE company_id = $1
  AND created_at >= $2;
