package report

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers all report routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/reports/dtr", auth.ManagerOrAbove(), h.GetDTR)
	protected.GET("/reports/dtr/csv", auth.ManagerOrAbove(), h.GetDTRCSV)
	protected.GET("/reports/dole-register", auth.AdminOnly(), h.GetDOLERegister)
}
