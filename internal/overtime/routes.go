package overtime

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers all overtime routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.POST("/overtime/requests", h.CreateRequest)
	protected.GET("/overtime/requests", h.ListRequests)
	protected.POST("/overtime/requests/:id/approve", auth.ManagerOrAbove(), h.ApproveRequest)
	protected.POST("/overtime/requests/:id/reject", auth.ManagerOrAbove(), h.RejectRequest)
}
