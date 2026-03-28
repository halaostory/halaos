package breaks

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.POST("/attendance/breaks/start", h.StartBreak)
	protected.POST("/attendance/breaks/end", h.EndBreak)
	protected.GET("/attendance/breaks", h.ListBreaks)
	protected.GET("/attendance/breaks/active", h.GetActiveBreak)
	protected.GET("/attendance/break-policies", auth.ManagerOrAbove(), h.ListPolicies)
	protected.PUT("/attendance/break-policies", auth.AdminOnly(), h.UpsertPolicies)
	protected.GET("/attendance/report/monthly", auth.ManagerOrAbove(), h.MonthlyReport)
}
