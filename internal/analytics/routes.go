package analytics

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers all analytics routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/analytics/summary", auth.AdminOnly(), h.GetSummary)
	protected.GET("/analytics/headcount-trend", auth.AdminOnly(), h.GetHeadcountTrend)
	protected.GET("/analytics/turnover", auth.AdminOnly(), h.GetTurnoverStats)
	protected.GET("/analytics/department-costs", auth.AdminOnly(), h.GetDepartmentCosts)
	protected.GET("/analytics/attendance-patterns", auth.AdminOnly(), h.GetAttendancePatterns)
	protected.GET("/analytics/employment-types", auth.AdminOnly(), h.GetEmploymentTypeBreakdown)
	protected.GET("/analytics/leave-utilization", auth.AdminOnly(), h.GetLeaveUtilization)

	// Suggestions
	protected.GET("/analytics/suggestions", auth.ManagerOrAbove(), h.GetSuggestions)
}
