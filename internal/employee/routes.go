package employee

import (
	"github.com/gin-gonic/gin"
	"github.com/halaostory/halaos/internal/auth"
)

func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/employees", auth.ManagerOrAbove(), h.ListEmployees)
	protected.POST("/employees", auth.AdminOnly(), h.CreateEmployee)
	protected.GET("/employees/:id", h.recordAccess(accessBasic), h.GetEmployee)
	protected.PUT("/employees/:id", auth.AdminOnly(), h.recordAccess(accessBasic), h.UpdateEmployee)
	protected.GET("/employees/:id/profile", h.recordAccess(accessPrivate), h.GetProfile)
	protected.PUT("/employees/:id/profile", auth.AdminOnly(), h.recordAccess(accessBasic), h.UpdateProfile)
	protected.GET("/employees/:id/documents", h.recordAccess(accessPrivate), h.ListDocuments)
	protected.POST("/employees/:id/documents", auth.AdminOnly(), h.recordAccess(accessBasic), h.UploadDocument)
	protected.GET("/employees/:id/documents/:doc_id/download", h.recordAccess(accessPrivate), h.DownloadDocument)
	protected.DELETE("/employees/:id/documents/:doc_id", auth.AdminOnly(), h.recordAccess(accessBasic), h.DeleteDocument)
	protected.GET("/employees/documents/expiring", auth.ManagerOrAbove(), h.ListExpiringDocuments)
	protected.GET("/employees/:id/salary", auth.AdminOnly(), h.recordAccess(accessBasic), h.GetSalary)
	protected.POST("/employees/:id/salary", auth.AdminOnly(), h.recordAccess(accessBasic), h.AssignSalary)
	protected.POST("/employees/salary/bulk-update", auth.AdminOnly(), h.BulkUpdateSalary)
	protected.PUT("/employees/:id/status", auth.AdminOnly(), h.recordAccess(accessBasic), h.ChangeStatus)
	protected.GET("/employees/:id/timeline", h.recordAccess(accessBasic), h.GetTimeline)
	protected.GET("/employees/:id/coe", auth.ManagerOrAbove(), h.recordAccess(accessBasic), h.GenerateCOE)
	protected.POST("/employees/:id/letters", auth.ManagerOrAbove(), h.recordAccess(accessBasic), h.GenerateLetter)
	protected.GET("/directory", h.ListDirectory)
	protected.GET("/directory/org-chart", h.GetOrgChart)
}
