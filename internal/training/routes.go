package training

import (
	"github.com/gin-gonic/gin"
	"github.com/halaostory/halaos/internal/auth"
)

// RegisterRoutes registers all training and certification routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/trainings", h.ListTrainings)
	protected.POST("/trainings", auth.AdminOnly(), h.CreateTraining)
	protected.PUT("/trainings/:id/status", auth.AdminOnly(), h.UpdateTrainingStatus)
	protected.GET("/trainings/:id/participants", h.ListParticipants)
	protected.POST("/trainings/:id/participants", auth.AdminOnly(), h.AddParticipant)
	protected.GET("/certifications", h.ListCertifications)
	protected.POST("/certifications", auth.AdminOnly(), h.CreateCertification)
	protected.DELETE("/certifications/:id", auth.AdminOnly(), h.DeleteCertification)
	protected.GET("/certifications/expiring", auth.ManagerOrAbove(), h.ListExpiringCertifications)
}
