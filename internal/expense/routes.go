package expense

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers all expense reimbursement routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/expenses/categories", h.ListCategories)
	protected.POST("/expenses/categories", auth.AdminOnly(), h.CreateCategory)
	protected.PUT("/expenses/categories/:id", auth.AdminOnly(), h.UpdateCategory)
	protected.GET("/expenses/summary", auth.ManagerOrAbove(), h.GetSummary)
	protected.GET("/expenses", auth.ManagerOrAbove(), h.List)
	protected.GET("/expenses/my", h.ListMy)
	protected.GET("/expenses/:id", h.Get)
	protected.POST("/expenses", h.Create)
	protected.POST("/expenses/:id/submit", h.Submit)
	protected.POST("/expenses/:id/approve", auth.ManagerOrAbove(), h.Approve)
	protected.POST("/expenses/:id/reject", auth.ManagerOrAbove(), h.Reject)
	protected.POST("/expenses/:id/mark-paid", auth.AdminOnly(), h.MarkPaid)
}
