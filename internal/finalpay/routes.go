package finalpay

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers all final pay routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/final-pay", auth.AdminOnly(), h.List)
	protected.GET("/final-pay/:employee_id", auth.AdminOnly(), h.Get)
	protected.POST("/final-pay", auth.AdminOnly(), h.Create)
	protected.PUT("/final-pay/:id/status", auth.AdminOnly(), h.UpdateStatus)
}
