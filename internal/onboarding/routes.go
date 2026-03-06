package onboarding

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers all onboarding routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/onboarding/templates", auth.AdminOnly(), h.ListTemplates)
	protected.POST("/onboarding/templates", auth.AdminOnly(), h.CreateTemplate)
	protected.POST("/onboarding/initiate", auth.AdminOnly(), h.InitiateWorkflow)
	protected.GET("/onboarding/tasks/pending", auth.ManagerOrAbove(), h.ListPendingTasks)
	protected.GET("/onboarding/employees/:employee_id/tasks", auth.ManagerOrAbove(), h.ListTasks)
	protected.GET("/onboarding/employees/:employee_id/progress", h.GetProgress)
	protected.PUT("/onboarding/tasks/:id", auth.ManagerOrAbove(), h.UpdateTaskStatus)
}
