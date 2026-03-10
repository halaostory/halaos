package pulse

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers pulse survey routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	// Admin: survey management
	protected.POST("/pulse-surveys", auth.AdminOnly(), h.CreateSurvey)
	protected.GET("/pulse-surveys", auth.ManagerOrAbove(), h.ListSurveys)
	protected.GET("/pulse-surveys/:id", auth.ManagerOrAbove(), h.GetSurvey)
	protected.PUT("/pulse-surveys/:id", auth.AdminOnly(), h.UpdateSurvey)
	protected.DELETE("/pulse-surveys/:id", auth.AdminOnly(), h.DeactivateSurvey)
	protected.GET("/pulse-surveys/:id/results", auth.ManagerOrAbove(), h.GetResults)

	// Employee: respond to surveys
	protected.GET("/pulse-surveys/active", h.ListActiveSurveys)
	protected.GET("/pulse-surveys/:id/open-round", h.GetOpenRound)
	protected.POST("/pulse-surveys/rounds/:round_id/respond", h.SubmitResponse)
}
