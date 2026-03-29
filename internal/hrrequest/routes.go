package hrrequest

import (
	"github.com/gin-gonic/gin"
	"github.com/halaostory/halaos/internal/auth"
)

// RegisterRoutes registers HR service request routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	// Employee self-service
	protected.POST("/hr-requests", h.CreateRequest)
	protected.GET("/hr-requests/my", h.ListMyRequests)

	// Admin/HR management
	protected.GET("/hr-requests", auth.ManagerOrAbove(), h.ListAllRequests)
	protected.GET("/hr-requests/:id", h.GetRequest)
	protected.PUT("/hr-requests/:id/status", auth.ManagerOrAbove(), h.UpdateStatus)
	protected.GET("/hr-requests/stats", auth.ManagerOrAbove(), h.GetStats)
}
