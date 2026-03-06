package grievance

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers all grievance management routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/grievances/summary", auth.ManagerOrAbove(), h.GetSummary)
	protected.GET("/grievances", auth.ManagerOrAbove(), h.List)
	protected.GET("/grievances/:id", auth.ManagerOrAbove(), h.Get)
	protected.POST("/grievances", h.Create)
	protected.GET("/grievances/my", h.ListMy)
	protected.PUT("/grievances/:id/status", auth.ManagerOrAbove(), h.UpdateStatus)
	protected.POST("/grievances/:id/assign", auth.ManagerOrAbove(), h.Assign)
	protected.POST("/grievances/:id/resolve", auth.ManagerOrAbove(), h.Resolve)
	protected.POST("/grievances/:id/withdraw", h.Withdraw)
	protected.GET("/grievances/:id/comments", auth.ManagerOrAbove(), h.ListComments)
	protected.POST("/grievances/:id/comments", auth.ManagerOrAbove(), h.AddComment)
}
