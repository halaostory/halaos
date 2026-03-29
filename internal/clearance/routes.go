package clearance

import (
	"github.com/gin-gonic/gin"
	"github.com/halaostory/halaos/internal/auth"
)

// RegisterRoutes registers all clearance and resignation routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.POST("/clearance", auth.ManagerOrAbove(), h.Create)
	protected.GET("/clearance", auth.ManagerOrAbove(), h.List)
	protected.GET("/clearance/:id", auth.ManagerOrAbove(), h.Get)
	protected.PUT("/clearance/:id/status", auth.AdminOnly(), h.UpdateStatus)
	protected.PUT("/clearance/items/:id", auth.ManagerOrAbove(), h.UpdateItem)
	protected.GET("/clearance/templates", auth.AdminOnly(), h.ListTemplates)
	protected.POST("/clearance/templates", auth.AdminOnly(), h.CreateTemplate)
	protected.DELETE("/clearance/templates/:id", auth.AdminOnly(), h.DeleteTemplate)
}
