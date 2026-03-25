package onboarding_checklist

import "github.com/gin-gonic/gin"

func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/onboarding-checklist/my-progress", h.GetMyProgress)
	protected.POST("/onboarding-checklist/complete-step", h.CompleteStep)
	protected.POST("/onboarding-checklist/dismiss", h.Dismiss)
}
