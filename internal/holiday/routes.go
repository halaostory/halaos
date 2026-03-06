package holiday

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers all holiday routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/holidays", h.List)
	protected.POST("/holidays", auth.AdminOnly(), h.Create)
	protected.DELETE("/holidays/:id", auth.AdminOnly(), h.Delete)
}
