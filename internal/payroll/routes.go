package payroll

import (
	"github.com/gin-gonic/gin"
	"github.com/halaostory/halaos/internal/auth"
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

	// KPI Bonus
	protected.GET("/payroll/bonus/structures", auth.AdminOnly(), h.ListBonusStructures)
	protected.POST("/payroll/bonus/structures", auth.AdminOnly(), h.CreateBonusStructure)
	protected.GET("/payroll/bonus/structures/:id", auth.AdminOnly(), h.GetBonusStructure)
	protected.PUT("/payroll/bonus/structures/:id/status", auth.AdminOnly(), h.UpdateBonusStructureStatus)
	protected.GET("/payroll/bonus/structures/:id/allocations", auth.AdminOnly(), h.ListBonusAllocations)
	protected.POST("/payroll/bonus/structures/:id/calculate", auth.AdminOnly(), h.CalculateBonusAllocations)
	protected.POST("/payroll/bonus/structures/:id/allocations", auth.AdminOnly(), h.CreateBonusAllocation)
	protected.POST("/payroll/bonus/allocations/approve", auth.AdminOnly(), h.ApproveBonusAllocations)

	// Benefit Deductions (US)
	protected.GET("/payroll/benefit-deductions", auth.AdminOnly(), h.ListBenefitDeductions)
	protected.POST("/payroll/benefit-deductions", auth.AdminOnly(), h.CreateBenefitDeduction)
	protected.PUT("/payroll/benefit-deductions/:id", auth.AdminOnly(), h.UpdateBenefitDeduction)
	protected.DELETE("/payroll/benefit-deductions/:id", auth.AdminOnly(), h.DeleteBenefitDeduction)

	// Company Registration Numbers (US)
	protected.GET("/company/registration-numbers", auth.AdminOnly(), h.ListRegistrationNumbers)
	protected.POST("/company/registration-numbers", auth.AdminOnly(), h.UpsertRegistrationNumber)
	protected.DELETE("/company/registration-numbers/:id", auth.AdminOnly(), h.DeleteRegistrationNumber)
}
