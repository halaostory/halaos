package benefits

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers all benefits administration routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/benefits/plans", h.ListPlans)
	protected.GET("/benefits/plans/:id", h.GetPlan)
	protected.POST("/benefits/plans", auth.AdminOnly(), h.CreatePlan)
	protected.PUT("/benefits/plans/:id", auth.AdminOnly(), h.UpdatePlan)
	protected.GET("/benefits/summary", auth.ManagerOrAbove(), h.GetSummary)
	protected.GET("/benefits/enrollments", auth.ManagerOrAbove(), h.ListEnrollments)
	protected.GET("/benefits/my-enrollments", h.ListMyEnrollments)
	protected.POST("/benefits/enrollments", auth.ManagerOrAbove(), h.CreateEnrollment)
	protected.POST("/benefits/enrollments/:id/cancel", auth.ManagerOrAbove(), h.CancelEnrollment)
	protected.GET("/benefits/enrollments/:id/dependents", h.ListDependents)
	protected.POST("/benefits/enrollments/:id/dependents", h.AddDependent)
	protected.DELETE("/benefits/dependents/:id", h.DeleteDependent)
	protected.GET("/benefits/claims", auth.ManagerOrAbove(), h.ListClaims)
	protected.POST("/benefits/claims", h.CreateClaim)
	protected.POST("/benefits/claims/:id/approve", auth.ManagerOrAbove(), h.ApproveClaim)
	protected.POST("/benefits/claims/:id/reject", auth.ManagerOrAbove(), h.RejectClaim)
}
