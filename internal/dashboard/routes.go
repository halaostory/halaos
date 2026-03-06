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
	protected.GET("/suggestions", auth.ManagerOrAbove(), h.GetSuggestions)
}
