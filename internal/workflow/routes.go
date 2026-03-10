package workflow

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers workflow rule routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	wf := protected.Group("/workflow")
	wf.Use(auth.AdminOnly())
	{
		wf.GET("/rules", h.List)
		wf.POST("/rules", h.Create)
		wf.PUT("/rules/:id", h.Update)
		wf.DELETE("/rules/:id", h.Deactivate)
		wf.GET("/rules/:id/executions", h.ListExecutions)
		wf.GET("/executions", h.ListExecutions) // all executions

		// SLA config
		wf.GET("/sla-configs", h.ListSLAConfigs)
		wf.PUT("/sla-configs", h.UpsertSLAConfig)

	}

	// Analytics — accessible by managers too
	protected.GET("/workflow/analytics", auth.ManagerOrAbove(), h.GetWorkflowAnalytics)
}
