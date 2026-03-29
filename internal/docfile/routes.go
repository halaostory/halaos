package docfile

import (
	"github.com/gin-gonic/gin"
	"github.com/halaostory/halaos/internal/auth"
)

// RegisterRoutes registers all 201 file document management routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/201file/categories", h.ListCategories)
	protected.POST("/201file/categories", auth.AdminOnly(), h.CreateCategory)
	protected.GET("/201file/employee/:id", auth.ManagerOrAbove(), h.ListDocuments)
	protected.GET("/201file/employee/:id/stats", auth.ManagerOrAbove(), h.GetStats)
	protected.POST("/201file/employee/:id/upload", auth.ManagerOrAbove(), h.Upload)
	protected.GET("/201file/document/:doc_id/download", h.Download)
	protected.PUT("/201file/document/:doc_id", auth.ManagerOrAbove(), h.UpdateDocument)
	protected.DELETE("/201file/document/:doc_id", auth.AdminOnly(), h.DeleteDocument)
	protected.GET("/201file/expiring", auth.ManagerOrAbove(), h.ListExpiring)
	protected.GET("/201file/employee/:id/compliance", auth.ManagerOrAbove(), h.GetCompliance)
	protected.GET("/201file/requirements", auth.AdminOnly(), h.ListRequirements)
	protected.POST("/201file/requirements", auth.AdminOnly(), h.CreateRequirement)
	protected.DELETE("/201file/requirements/:id", auth.AdminOnly(), h.DeleteRequirement)
}
