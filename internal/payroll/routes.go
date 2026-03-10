package payroll

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/payroll/cycles", auth.AdminOnly(), h.ListCycles)
	protected.POST("/payroll/cycles", auth.AdminOnly(), h.CreateCycle)
	protected.POST("/payroll/runs", auth.AdminOnly(), h.RunPayroll)
	protected.GET("/payroll/runs/:id/items", auth.AdminOnly(), h.ListPayrollItems)
	protected.GET("/payroll/cycles/:id/items", auth.AdminOnly(), h.ListCycleItems)
	protected.POST("/payroll/cycles/:id/approve", auth.AdminOnly(), h.ApproveCycle)
	protected.POST("/payroll/cycles/:id/lock", auth.AdminOnly(), h.LockCycle)
	protected.POST("/payroll/cycles/:id/unlock", auth.AdminOnly(), h.UnlockCycle)
	protected.GET("/payroll/runs/:id/anomalies", auth.AdminOnly(), h.GetRunAnomalies)
	protected.GET("/payroll/cycles/:id/anomalies", auth.AdminOnly(), h.GetCycleAnomalies)
	protected.GET("/payroll/payslips", h.ListPayslips)
	protected.GET("/payroll/payslips/:id", h.GetPayslip)
	protected.GET("/payroll/payslips/:id/pdf", h.DownloadPayslipPDF)
	protected.GET("/payroll/13th-month", auth.AdminOnly(), h.List13thMonthPay)
	protected.POST("/payroll/13th-month/calculate", auth.AdminOnly(), h.Calculate13thMonth)

	// Auto-payroll config
	protected.GET("/payroll/auto-config", auth.AdminOnly(), h.GetAutoConfig)
	protected.PUT("/payroll/auto-config", auth.AdminOnly(), h.UpdateAutoConfig)
	protected.GET("/payroll/auto-logs", auth.AdminOnly(), h.ListAutoLogs)
}
