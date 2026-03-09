package integration

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all integration HTTP routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	ig := protected.Group("/integrations")

	// Connection management
	ig.GET("/connections", h.ListConnections)
	ig.POST("/connections", h.CreateConnection)
	ig.GET("/connections/:id", h.GetConnection)
	ig.PUT("/connections/:id", h.UpdateConnection)
	ig.DELETE("/connections/:id", h.DeleteConnection)
	ig.POST("/connections/:id/test", h.TestConnection)

	// Templates
	ig.GET("/templates", h.ListTemplates)
	ig.POST("/templates", h.CreateTemplate)
	ig.PUT("/templates/:id", h.UpdateTemplate)
	ig.DELETE("/templates/:id", h.DeleteTemplate)

	// Jobs
	ig.GET("/jobs", h.ListJobs)
	ig.POST("/jobs/:id/retry", h.RetryJob)
	ig.POST("/jobs/:id/skip", h.SkipJob)

	// Audit
	ig.GET("/audit", h.ListAuditLogs)

	// Employee integrations (nested under employees)
	protected.GET("/employees/:id/integrations", h.ListEmployeeIntegrations)
}
