package announcement

import (
	"github.com/gin-gonic/gin"
	"github.com/halaostory/halaos/internal/auth"
)

// RegisterRoutes registers all announcement routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/announcements", h.ListActive)
	protected.GET("/announcements/all", auth.AdminOnly(), h.ListAll)
	protected.POST("/announcements", auth.AdminOnly(), h.Create)
	protected.DELETE("/announcements/:id", auth.AdminOnly(), h.Delete)
}
