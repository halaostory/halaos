package notification

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all notification routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/notifications", h.ListNotifications)
	protected.GET("/notifications/unread-count", h.CountUnread)
	protected.POST("/notifications/:id/read", h.MarkRead)
	protected.POST("/notifications/read-all", h.MarkAllRead)
	protected.DELETE("/notifications/:id", h.Delete)
}
