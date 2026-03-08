package dashboard

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/dashboard/stats", h.GetStats)
	protected.GET("/dashboard/attendance", h.GetAttendance)
	protected.GET("/dashboard/department-distribution", h.GetDepartmentDistribution)
	protected.GET("/dashboard/payroll-trend", auth.AdminOnly(), h.GetPayrollTrend)
	protected.GET("/dashboard/leave-summary", h.GetLeaveSummary)
	protected.GET("/dashboard/action-items", auth.ManagerOrAbove(), h.GetActionItems)
	protected.GET("/dashboard/celebrations", h.GetCelebrations)
	protected.GET("/dashboard/suggestions", auth.ManagerOrAbove(), h.GetSuggestions)
	protected.GET("/dashboard/flight-risk", auth.ManagerOrAbove(), h.GetFlightRisk)
	protected.GET("/dashboard/team-health", auth.ManagerOrAbove(), h.GetTeamHealth)

	// AI briefing endpoint - accessible to all authenticated users.
	// Manager/admin data is included conditionally based on role.
	protected.GET("/ai/briefing", h.GetBriefing)

	// AI form pre-fill endpoint - accessible to all authenticated users.
	// Returns suggestions based on the user's history.
	protected.GET("/ai/form-prefill", h.GetFormPrefill)
}
