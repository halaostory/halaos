package workflow

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// GetWorkflowAnalytics returns a summary of workflow metrics for the dashboard.
func (h *Handler) GetWorkflowAnalytics(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	since := time.Now().AddDate(0, -1, 0) // last 30 days

	// Summary stats
	summary, err := h.queries.GetWorkflowSummaryStats(c.Request.Context(), store.GetWorkflowSummaryStatsParams{
		CompanyID: companyID,
		CreatedAt: since,
	})
	if err != nil {
		h.logger.Error("analytics: failed to get summary", "error", err)
		summary = store.GetWorkflowSummaryStatsRow{}
	}

	// Avg approval time by type
	avgTimes, err := h.queries.GetAvgApprovalTimeByType(c.Request.Context(), companyID)
	if err != nil {
		h.logger.Error("analytics: failed to get avg times", "error", err)
		avgTimes = []store.GetAvgApprovalTimeByTypeRow{}
	}

	// Auto-approval stats
	autoStats, err := h.queries.GetAutoApprovalStats(c.Request.Context(), store.GetAutoApprovalStatsParams{
		CompanyID: companyID,
		CreatedAt: since,
	})
	if err != nil {
		h.logger.Error("analytics: failed to get auto stats", "error", err)
		autoStats = store.GetAutoApprovalStatsRow{}
	}

	// Volume by day
	volume, err := h.queries.GetApprovalVolumeByDay(c.Request.Context(), store.GetApprovalVolumeByDayParams{
		CompanyID: companyID,
		CreatedAt: since,
	})
	if err != nil {
		h.logger.Error("analytics: failed to get volume", "error", err)
		volume = []store.GetApprovalVolumeByDayRow{}
	}

	// Pending age distribution
	pendingAge, err := h.queries.GetPendingAgeDistribution(c.Request.Context(), companyID)
	if err != nil {
		h.logger.Error("analytics: failed to get pending age", "error", err)
		pendingAge = []store.GetPendingAgeDistributionRow{}
	}

	// SLA compliance
	sla, err := h.queries.GetSLAComplianceRate(c.Request.Context(), store.GetSLAComplianceRateParams{
		CompanyID: companyID,
		CreatedAt: since,
	})
	if err != nil {
		h.logger.Error("analytics: failed to get SLA compliance", "error", err)
		sla = store.GetSLAComplianceRateRow{}
	}

	// Format volume for frontend
	volumeData := make([]map[string]any, len(volume))
	for i, v := range volume {
		volumeData[i] = map[string]any{
			"day":      v.Day.Format("Jan 2"),
			"total":    v.Total,
			"approved": toInt(v.Approved),
			"rejected": toInt(v.Rejected),
			"pending":  toInt(v.Pending),
		}
	}

	// Agent decision stats
	agentStats, err := h.queries.GetAgentDecisionStats(c.Request.Context(), companyID)
	if err != nil {
		h.logger.Error("analytics: failed to get agent stats", "error", err)
		agentStats = store.GetAgentDecisionStatsRow{}
	}

	overrideRate := "0%"
	if agentStats.Total > 0 {
		overrideRate = fmt.Sprintf("%.0f%%", float64(agentStats.Overridden)/float64(agentStats.Total)*100)
	}

	// Agent accuracy by confidence tier
	accuracy, err := h.queries.GetDecisionAccuracyRate(c.Request.Context(), companyID)
	if err != nil {
		accuracy = []store.GetDecisionAccuracyRateRow{}
	}

	// Agent decision volume by day
	decisionVolume, err := h.queries.GetDecisionVolumeByDay(c.Request.Context(), companyID)
	if err != nil {
		decisionVolume = []store.GetDecisionVolumeByDayRow{}
	}

	decisionVolumeData := make([]map[string]any, len(decisionVolume))
	for i, v := range decisionVolume {
		decisionVolumeData[i] = map[string]any{
			"day":           v.Day.Format("Jan 2"),
			"total":         v.Total,
			"auto_executed": v.AutoExecuted,
			"recommended":   v.Recommended,
			"escalated":     v.Escalated,
		}
	}

	response.OK(c, gin.H{
		"summary": gin.H{
			"total":    summary.TotalApprovals,
			"pending":  toInt(summary.PendingCount),
			"approved": toInt(summary.ApprovedCount),
			"rejected": toInt(summary.RejectedCount),
		},
		"avg_approval_time": avgTimes,
		"auto_approval": gin.H{
			"auto_approved": toInt(autoStats.AutoApproved),
			"auto_rejected": toInt(autoStats.AutoRejected),
			"total":         autoStats.TotalExecutions,
		},
		"volume_by_day": volumeData,
		"pending_age":   pendingAge,
		"sla_compliance": gin.H{
			"total":      sla.Total,
			"within_sla": toInt(sla.WithinSla),
			"overdue":    toInt(sla.Overdue),
		},
		"agent_decisions": gin.H{
			"total":          agentStats.Total,
			"auto_executed":  agentStats.AutoExecuted,
			"recommended":    agentStats.Recommended,
			"escalated":      agentStats.Escalated,
			"overridden":     agentStats.Overridden,
			"override_rate":  overrideRate,
			"avg_confidence": fmt.Sprintf("%v", agentStats.AvgConfidence),
		},
		"agent_accuracy":          accuracy,
		"agent_decision_volume":   decisionVolumeData,
	})
}

// toInt converts an interface{} (from COALESCE) to int64.
func toInt(v interface{}) int64 {
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
