package performance

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers all performance management routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/performance/cycles", auth.ManagerOrAbove(), h.ListCycles)
	protected.POST("/performance/cycles", auth.AdminOnly(), h.CreateCycle)
	protected.POST("/performance/cycles/:id/initiate", auth.AdminOnly(), h.InitiateReviews)
	protected.GET("/performance/cycles/:id/reviews", auth.ManagerOrAbove(), h.ListReviewsByCycle)
	protected.GET("/performance/reviews/my", h.ListMyReviews)
	protected.GET("/performance/reviews/:id", h.GetReview)
	protected.PUT("/performance/reviews/:id/self", h.SubmitSelfReview)
	protected.PUT("/performance/reviews/:id/manager", auth.ManagerOrAbove(), h.SubmitManagerReview)
	protected.GET("/performance/goals", h.ListGoals)
	protected.POST("/performance/goals", auth.ManagerOrAbove(), h.CreateGoal)
	protected.PUT("/performance/goals/:id", h.UpdateGoal)
}
