package audit

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers all audit routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/audit/logs", auth.AdminOnly(), h.ListActivityLogs)
}
