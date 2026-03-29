package milestone

import (
	"github.com/gin-gonic/gin"
	"github.com/halaostory/halaos/internal/auth"
)

// RegisterRoutes registers all milestone routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/milestones", auth.ManagerOrAbove(), h.ListMilestones)
	protected.GET("/milestones/pending", auth.ManagerOrAbove(), h.ListPending)
	protected.POST("/milestones/:id/acknowledge", auth.ManagerOrAbove(), h.Acknowledge)
	protected.POST("/milestones/:id/action", auth.ManagerOrAbove(), h.Action)
}
