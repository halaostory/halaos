package leave

import (
	"github.com/gin-gonic/gin"
	"github.com/halaostory/halaos/internal/auth"
)

func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/leaves/types", h.ListTypes)
	protected.POST("/leaves/types", auth.AdminOnly(), h.CreateType)
	protected.GET("/leaves/balances", h.GetBalances)
	protected.POST("/leaves/requests", h.CreateRequest)
	protected.GET("/leaves/requests", h.ListRequests)
	protected.POST("/leaves/requests/:id/approve", auth.ManagerOrAbove(), h.ApproveRequest)
	protected.POST("/leaves/requests/:id/reject", auth.ManagerOrAbove(), h.RejectRequest)
	protected.POST("/leaves/requests/:id/cancel", h.CancelRequest)

	// Balance Management
	protected.GET("/leaves/balances/all", auth.AdminOnly(), h.ListAllBalances)
	protected.PUT("/leaves/balances/adjust", auth.AdminOnly(), h.AdjustBalance)
	protected.POST("/leaves/carryover", auth.AdminOnly(), h.Carryover)
	protected.GET("/leaves/calendar", h.GetCalendar)

	// Encashment
	protected.GET("/leaves/encashment/convertible", h.GetConvertibleBalances)
	protected.POST("/leaves/encashment", h.CreateEncashment)
	protected.GET("/leaves/encashment", auth.ManagerOrAbove(), h.ListEncashments)
	protected.POST("/leaves/encashment/:id/approve", auth.ManagerOrAbove(), h.ApproveEncashment)
	protected.POST("/leaves/encashment/:id/reject", auth.ManagerOrAbove(), h.RejectEncashment)
	protected.POST("/leaves/encashment/:id/paid", auth.AdminOnly(), h.MarkEncashmentPaid)
}
