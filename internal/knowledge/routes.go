package knowledge

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers all knowledge base routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/knowledge/search", h.Search)
	protected.GET("/knowledge", auth.AdminOnly(), h.List)
	protected.GET("/knowledge/categories", h.ListCategories)
	protected.GET("/knowledge/:id", h.Get)
	protected.POST("/knowledge", auth.AdminOnly(), h.Create)
	protected.PUT("/knowledge/:id", auth.AdminOnly(), h.Update)
	protected.DELETE("/knowledge/:id", auth.AdminOnly(), h.Delete)
}
