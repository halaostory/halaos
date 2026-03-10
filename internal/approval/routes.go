package approval

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers all approval routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/approvals/pending", auth.ManagerOrAbove(), h.ListPending)
	protected.POST("/approvals/:id/approve", auth.ManagerOrAbove(), h.Approve)
	protected.POST("/approvals/:id/reject", auth.ManagerOrAbove(), h.Reject)
	protected.GET("/approvals/context", auth.ManagerOrAbove(), h.GetApprovalContext)
}
