package workflow

import (
	"github.com/gin-gonic/gin"
	"github.com/halaostory/halaos/internal/auth"
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

		// Triggers (admin only)
		wf.GET("/triggers", h.ListTriggers)
		wf.POST("/triggers", h.CreateTrigger)
		wf.PUT("/triggers/:id", h.UpdateTrigger)
		wf.DELETE("/triggers/:id", h.DeactivateTrigger)
	}

	// Decisions — accessible by managers too
	decisions := protected.Group("/workflow")
	decisions.Use(auth.ManagerOrAbove())
	{
		decisions.GET("/decisions", h.ListDecisions)
		decisions.GET("/decisions/:id", h.GetDecision)
		decisions.POST("/decisions/:id/override", h.OverrideDecision)
	}

	// Analytics — accessible by managers too
	protected.GET("/workflow/analytics", auth.ManagerOrAbove(), h.GetWorkflowAnalytics)
}
