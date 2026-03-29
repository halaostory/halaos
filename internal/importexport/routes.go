package importexport

import (
	"github.com/gin-gonic/gin"
	"github.com/halaostory/halaos/internal/auth"
)

// RegisterRoutes registers all import/export routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/export/attendance/csv", auth.AdminOnly(), h.ExportAttendanceCSV)
	protected.GET("/export/leave-balances/csv", auth.AdminOnly(), h.ExportLeaveBalancesCSV)
	protected.POST("/import/employees/csv", auth.AdminOnly(), h.ImportEmployeesCSV)
	protected.POST("/import/employees/preview", auth.AdminOnly(), h.PreviewImportCSV)
}
