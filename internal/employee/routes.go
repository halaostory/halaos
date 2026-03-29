package employee

import (
	"github.com/gin-gonic/gin"
	"github.com/halaostory/halaos/internal/auth"
)

func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/employees", auth.ManagerOrAbove(), h.ListEmployees)
	protected.POST("/employees", auth.AdminOnly(), h.CreateEmployee)
	protected.GET("/employees/:id", h.GetEmployee)
	protected.PUT("/employees/:id", auth.AdminOnly(), h.UpdateEmployee)
	protected.GET("/employees/:id/profile", h.GetProfile)
	protected.PUT("/employees/:id/profile", auth.AdminOnly(), h.UpdateProfile)
	protected.GET("/employees/:id/documents", h.ListDocuments)
	protected.POST("/employees/:id/documents", auth.AdminOnly(), h.UploadDocument)
	protected.GET("/employees/:id/documents/:doc_id/download", h.DownloadDocument)
	protected.DELETE("/employees/:id/documents/:doc_id", auth.AdminOnly(), h.DeleteDocument)
	protected.GET("/employees/documents/expiring", auth.ManagerOrAbove(), h.ListExpiringDocuments)
	protected.GET("/employees/:id/salary", auth.AdminOnly(), h.GetSalary)
	protected.POST("/employees/:id/salary", auth.AdminOnly(), h.AssignSalary)
	protected.POST("/employees/salary/bulk-update", auth.AdminOnly(), h.BulkUpdateSalary)
	protected.PUT("/employees/:id/status", auth.AdminOnly(), h.ChangeStatus)
	protected.GET("/employees/:id/timeline", h.GetTimeline)
	protected.GET("/employees/:id/coe", auth.ManagerOrAbove(), h.GenerateCOE)
	protected.POST("/employees/:id/letters", auth.ManagerOrAbove(), h.GenerateLetter)
	protected.GET("/directory", h.ListDirectory)
	protected.GET("/directory/org-chart", h.GetOrgChart)
}
