package policy

import (
	"github.com/gin-gonic/gin"
	"github.com/halaostory/halaos/internal/auth"
)

// RegisterRoutes registers all company policy and acknowledgment routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/policies", h.ListPolicies)
	protected.GET("/policies/:id", h.GetPolicy)
	protected.POST("/policies", auth.AdminOnly(), h.CreatePolicy)
	protected.PUT("/policies/:id", auth.AdminOnly(), h.UpdatePolicy)
	protected.DELETE("/policies/:id", auth.AdminOnly(), h.DeletePolicy)
	protected.POST("/policies/:id/acknowledge", h.AcknowledgePolicy)
	protected.GET("/policies/:id/acknowledgments", auth.ManagerOrAbove(), h.ListAcknowledgments)
	protected.GET("/policies/:id/stats", auth.ManagerOrAbove(), h.GetAckStats)
	protected.GET("/policies/pending", h.ListPending)
}
