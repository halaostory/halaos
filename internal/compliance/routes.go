package compliance

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/compliance/sss-table", auth.AdminOnly(), h.ListSSSTable)
	protected.GET("/compliance/philhealth-table", auth.AdminOnly(), h.ListPhilHealthTable)
	protected.GET("/compliance/pagibig-table", auth.AdminOnly(), h.ListPagIBIGTable)
	protected.GET("/compliance/bir-tax-table", auth.AdminOnly(), h.ListBIRTaxTable)
	protected.GET("/compliance/government-forms", auth.AdminOnly(), h.ListGovernmentForms)
	protected.POST("/compliance/government-forms", auth.AdminOnly(), h.CreateGovernmentForm)
	protected.POST("/compliance/government-forms/generate", auth.AdminOnly(), h.GenerateFormHandler)

	// Tax Filing & Remittance
	protected.GET("/tax-filings", auth.AdminOnly(), h.ListTaxFilings)
	protected.POST("/tax-filings", auth.AdminOnly(), h.CreateTaxFiling)
	protected.PUT("/tax-filings/:id/status", auth.AdminOnly(), h.UpdateTaxFilingStatus)
	protected.GET("/tax-filings/overdue", auth.AdminOnly(), h.ListOverdueFilings)
	protected.GET("/tax-filings/upcoming", auth.AdminOnly(), h.ListUpcomingFilings)
	protected.POST("/tax-filings/generate-annual", auth.AdminOnly(), h.GenerateAnnualFilings)
	protected.GET("/tax-filings/remittances", auth.AdminOnly(), h.ListRemittanceRecords)

	// Salary structures & components
	protected.GET("/salary/structures", auth.AdminOnly(), h.ListSalaryStructures)
	protected.POST("/salary/structures", auth.AdminOnly(), h.CreateSalaryStructure)
	protected.GET("/salary/components", auth.AdminOnly(), h.ListSalaryComponents)
	protected.POST("/salary/components", auth.AdminOnly(), h.CreateSalaryComponent)
}
