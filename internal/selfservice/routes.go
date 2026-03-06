package selfservice

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all self-service routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/self-service/info", h.GetMyInfo)
	protected.GET("/self-service/team", h.GetMyTeam)
	protected.GET("/self-service/compensation", h.GetMyCompensation)
	protected.GET("/self-service/onboarding", h.GetMyOnboarding)
}
