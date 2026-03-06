package loan

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers all loan routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/loans/types", h.ListLoanTypes)
	protected.POST("/loans/types", auth.AdminOnly(), h.CreateLoanType)
	protected.GET("/loans", auth.AdminOnly(), h.ListLoans)
	protected.GET("/loans/my", h.ListMyLoans)
	protected.POST("/loans", h.ApplyLoan)
	protected.GET("/loans/:id", h.GetLoan)
	protected.POST("/loans/:id/approve", auth.AdminOnly(), h.ApproveLoan)
	protected.POST("/loans/:id/cancel", h.CancelLoan)
	protected.POST("/loans/:id/payments", auth.AdminOnly(), h.RecordPayment)
}
