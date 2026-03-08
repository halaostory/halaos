package recruitment

import (
	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers recruitment/ATS endpoints.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	r := protected.Group("/recruitment")

	// Job postings — admin/manager
	r.GET("/jobs", auth.ManagerOrAbove(), h.ListJobPostings)
	r.POST("/jobs", auth.ManagerOrAbove(), h.CreateJobPosting)
	r.GET("/jobs/:id", auth.ManagerOrAbove(), h.GetJobPosting)
	r.PUT("/jobs/:id", auth.ManagerOrAbove(), h.UpdateJobPosting)

	// Applicants — admin/manager
	r.GET("/applicants", auth.ManagerOrAbove(), h.ListApplicants)
	r.POST("/applicants", auth.ManagerOrAbove(), h.CreateApplicant)
	r.GET("/applicants/:id", auth.ManagerOrAbove(), h.GetApplicant)
	r.PUT("/applicants/:id/status", auth.ManagerOrAbove(), h.UpdateApplicantStatus)

	// Interviews
	r.POST("/applicants/:id/interviews", auth.ManagerOrAbove(), h.ScheduleInterview)

	// Stats
	r.GET("/stats", auth.ManagerOrAbove(), h.GetRecruitmentStats)
}
