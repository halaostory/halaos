package disciplinary

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers all disciplinary management routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.POST("/disciplinary/incidents", auth.ManagerOrAbove(), h.CreateIncident)
	protected.GET("/disciplinary/incidents", auth.ManagerOrAbove(), h.ListIncidents)
	protected.GET("/disciplinary/incidents/:id", auth.ManagerOrAbove(), h.GetIncident)
	protected.PUT("/disciplinary/incidents/:id/status", auth.ManagerOrAbove(), h.UpdateIncidentStatus)
	protected.POST("/disciplinary/actions", auth.ManagerOrAbove(), h.CreateAction)
	protected.GET("/disciplinary/actions", auth.ManagerOrAbove(), h.ListActions)
	protected.POST("/disciplinary/actions/:id/acknowledge", h.AcknowledgeAction)
	protected.POST("/disciplinary/actions/:id/appeal", h.AppealAction)
	protected.POST("/disciplinary/actions/:id/resolve-appeal", auth.ManagerOrAbove(), h.ResolveAppeal)
	protected.GET("/disciplinary/employee/:id/summary", auth.ManagerOrAbove(), h.GetEmployeeSummary)
}
