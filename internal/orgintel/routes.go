package orgintel

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers all org intelligence routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/org-intelligence/overview", auth.ManagerOrAbove(), h.GetOverview)
	protected.GET("/org-intelligence/trends", auth.ManagerOrAbove(), h.GetTrends)
	protected.GET("/org-intelligence/risk-distribution", auth.ManagerOrAbove(), h.GetRiskDistribution)
	protected.GET("/org-intelligence/employee/:id/trends", auth.ManagerOrAbove(), h.GetEmployeeTrends)
	protected.GET("/org-intelligence/department/:id/trends", auth.ManagerOrAbove(), h.GetDepartmentTrends)
	protected.GET("/org-intelligence/correlations", auth.ManagerOrAbove(), h.GetCorrelations)
	protected.GET("/org-intelligence/executive-briefing", auth.AdminOnly(), h.GetExecutiveBriefing)
	protected.POST("/org-intelligence/executive-briefing/generate", auth.AdminOnly(), h.GenerateExecutiveBriefing)
}
