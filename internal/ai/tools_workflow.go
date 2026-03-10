package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/store"
)

func (r *ToolRegistry) toolQueryWorkflowRules(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if user.CompanyID != companyID {
		return "", fmt.Errorf("access denied")
	}

	rules, err := r.queries.ListWorkflowRules(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("list workflow rules: %w", err)
	}

	autoApproved, _ := r.queries.CountAutoApprovedByCompany(ctx, companyID)

	type ruleItem struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		EntityType string `json:"entity_type"`
		RuleType   string `json:"rule_type"`
		Priority   int32  `json:"priority"`
		IsActive   bool   `json:"is_active"`
	}

	items := make([]ruleItem, 0, len(rules))
	activeCount := 0
	for _, r := range rules {
		items = append(items, ruleItem{
			ID:         r.ID,
			Name:       r.Name,
			EntityType: r.EntityType,
			RuleType:   r.RuleType,
			Priority:   r.Priority,
			IsActive:   r.IsActive,
		})
		if r.IsActive {
			activeCount++
		}
	}

	recentExecs, err := r.queries.ListRuleExecutions(ctx, store.ListRuleExecutionsParams{
		CompanyID: companyID,
		Column2:   0,
		Limit:     10,
		Offset:    0,
	})
	if err != nil {
		recentExecs = nil
	}

	type execItem struct {
		EntityType string `json:"entity_type"`
		EntityID   int64  `json:"entity_id"`
		Action     string `json:"action"`
		RuleName   string `json:"rule_name"`
		Reason     string `json:"reason"`
	}

	execItems := make([]execItem, 0, len(recentExecs))
	for _, e := range recentExecs {
		reason := ""
		if e.Reason != nil {
			reason = *e.Reason
		}
		execItems = append(execItems, execItem{
			EntityType: e.EntityType,
			EntityID:   e.EntityID,
			Action:     e.Action,
			RuleName:   e.RuleName,
			Reason:     reason,
		})
	}

	return toJSON(map[string]any{
		"total_rules":         len(items),
		"active_rules":        activeCount,
		"total_auto_approved": autoApproved,
		"rules":               items,
		"recent_executions":   execItems,
	})
}

func (r *ToolRegistry) toolQueryWorkflowAnalytics(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	since := time.Now().AddDate(0, -1, 0) // last 30 days

	summary, err := r.queries.GetWorkflowSummaryStats(ctx, store.GetWorkflowSummaryStatsParams{
		CompanyID: companyID,
		CreatedAt: since,
	})
	if err != nil {
		summary = store.GetWorkflowSummaryStatsRow{}
	}

	avgTimes, err := r.queries.GetAvgApprovalTimeByType(ctx, companyID)
	if err != nil {
		avgTimes = nil
	}

	autoStats, err := r.queries.GetAutoApprovalStats(ctx, store.GetAutoApprovalStatsParams{
		CompanyID: companyID,
		CreatedAt: since,
	})
	if err != nil {
		autoStats = store.GetAutoApprovalStatsRow{}
	}

	sla, err := r.queries.GetSLAComplianceRate(ctx, store.GetSLAComplianceRateParams{
		CompanyID: companyID,
		CreatedAt: since,
	})
	if err != nil {
		sla = store.GetSLAComplianceRateRow{}
	}

	pendingAge, err := r.queries.GetPendingAgeDistribution(ctx, companyID)
	if err != nil {
		pendingAge = nil
	}

	type avgTimeItem struct {
		EntityType string `json:"entity_type"`
		AvgHours   string `json:"avg_hours"`
	}
	times := make([]avgTimeItem, 0, len(avgTimes))
	for _, t := range avgTimes {
		times = append(times, avgTimeItem{
			EntityType: t.EntityType,
			AvgHours:   fmt.Sprintf("%v", t.AvgHours),
		})
	}

	type ageItem struct {
		Bucket string `json:"bucket"`
		Count  int64  `json:"count"`
	}
	ages := make([]ageItem, 0, len(pendingAge))
	for _, a := range pendingAge {
		ages = append(ages, ageItem{
			Bucket: fmt.Sprintf("%v", a.Bucket),
			Count:  toInt64(a.Count),
		})
	}

	return toJSON(map[string]any{
		"period": "last_30_days",
		"summary": map[string]any{
			"total":    summary.TotalApprovals,
			"pending":  toInt64(summary.PendingCount),
			"approved": toInt64(summary.ApprovedCount),
			"rejected": toInt64(summary.RejectedCount),
		},
		"avg_approval_time_by_type": times,
		"auto_approval": map[string]any{
			"auto_approved": toInt64(autoStats.AutoApproved),
			"auto_rejected": toInt64(autoStats.AutoRejected),
			"total":         autoStats.TotalExecutions,
		},
		"sla_compliance": map[string]any{
			"total":      sla.Total,
			"within_sla": toInt64(sla.WithinSla),
			"overdue":    toInt64(sla.Overdue),
		},
		"pending_age_distribution": ages,
	})
}

// toInt64 converts interface{} from COALESCE to int64.
func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int32:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		var n int64
		fmt.Sscanf(val, "%d", &n)
		return n
	default:
		return 0
	}
}

func (r *ToolRegistry) toolQueryWorkflowDecisions(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	stats, err := r.queries.GetAgentDecisionStats(ctx, companyID)
	if err != nil {
		return toJSON(map[string]any{"error": "no agent decision data available"})
	}

	accuracy, _ := r.queries.GetDecisionAccuracyRate(ctx, companyID)

	type accuracyItem struct {
		Tier       string `json:"tier"`
		Total      int64  `json:"total"`
		Overridden int64  `json:"overridden"`
	}
	items := make([]accuracyItem, 0, len(accuracy))
	for _, a := range accuracy {
		items = append(items, accuracyItem{
			Tier:       a.ConfidenceTier,
			Total:      a.Total,
			Overridden: a.Overridden,
		})
	}

	overrideRate := "0%"
	if stats.Total > 0 {
		overrideRate = fmt.Sprintf("%.0f%%", float64(stats.Overridden)/float64(stats.Total)*100)
	}

	return toJSON(map[string]any{
		"period": "last_30_days",
		"agent_decisions": map[string]any{
			"total":          stats.Total,
			"auto_executed":  stats.AutoExecuted,
			"recommended":    stats.Recommended,
			"escalated":      stats.Escalated,
			"overridden":     stats.Overridden,
			"override_rate":  overrideRate,
			"avg_confidence": fmt.Sprintf("%v", stats.AvgConfidence),
		},
		"accuracy_by_confidence": items,
	})
}

func workflowDefs() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "query_workflow_rules",
			Description: "Query workflow automation rules and recent execution history. Returns active rules, auto-approval counts, and recent auto-approved/rejected items.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "query_workflow_analytics",
			Description: "Query workflow analytics metrics for the last 30 days. Returns approval summary (total/pending/approved/rejected), average approval time by entity type, auto-approval stats, SLA compliance rate, and pending age distribution.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "query_workflow_decisions",
			Description: "Query AI agent decision statistics. Returns total decisions, auto-executed count, recommended count, escalated count, override rate, and accuracy by confidence tier.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
	}
}

func (r *ToolRegistry) registerWorkflowTools() {
	r.tools["query_workflow_rules"] = r.toolQueryWorkflowRules
	r.tools["query_workflow_analytics"] = r.toolQueryWorkflowAnalytics
	r.tools["query_workflow_decisions"] = r.toolQueryWorkflowDecisions
}
