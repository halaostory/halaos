package company

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/company", h.GetCompany)
	protected.PUT("/company", auth.AdminOnly(), h.UpdateCompany)
	protected.POST("/company/logo", auth.AdminOnly(), h.UploadLogo)
	protected.GET("/company/departments", h.ListDepartments)
	protected.POST("/company/departments", auth.AdminOnly(), h.CreateDepartment)
	protected.PUT("/company/departments/:id", auth.AdminOnly(), h.UpdateDepartment)
	protected.GET("/company/positions", h.ListPositions)
	protected.POST("/company/positions", auth.AdminOnly(), h.CreatePosition)
	protected.PUT("/company/positions/:id", auth.AdminOnly(), h.UpdatePosition)
	protected.GET("/company/cost-centers", h.ListCostCenters)
	protected.POST("/company/cost-centers", auth.AdminOnly(), h.CreateCostCenter)
}
