package compliance

import (
	"context"
	"fmt"

	"github.com/halaostory/halaos/internal/store"
)

// W2Data represents the data for a W-2 form.
type W2Data struct {
	// Employee info
	EmployeeName string `json:"employee_name"`
	EmployeeSSN  string `json:"employee_ssn"`
	EmployeeAddr string `json:"employee_address"`

	// Employer info
	EmployerName string `json:"employer_name"`
	EmployerEIN  string `json:"employer_ein"`
	EmployerAddr string `json:"employer_address"`

	// Box 1-6
	GrossWages          float64 `json:"box1_wages"`
	FederalTaxWithheld  float64 `json:"box2_federal_tax"`
	SSWages             float64 `json:"box3_ss_wages"`
	SSTaxWithheld       float64 `json:"box4_ss_tax"`
	MedicareWages       float64 `json:"box5_medicare_wages"`
	MedicareTaxWithheld float64 `json:"box6_medicare_tax"`

	// Pre-tax deductions for Box 12
	PreTax401k      float64 `json:"box12d_401k"`
	HealthInsurance float64 `json:"box12dd_health"`

	// State
	StateWages       float64 `json:"box16_state_wages"`
	StateTaxWithheld float64 `json:"box17_state_tax"`
	StateName        string  `json:"state_name"`
}

// Form941Data represents quarterly Form 941 data.
type Form941Data struct {
	Quarter           int     `json:"quarter"`
	Year              int32   `json:"year"`
	EmployeeCount     int     `json:"employee_count"`
	TotalWages        float64 `json:"line2_wages"`
	FederalWithheld   float64 `json:"line3_federal_withheld"`
	SSWages           float64 `json:"line5a_ss_wages"`
	MedicareWages     float64 `json:"line5c_medicare_wages"`
	AddlMedicareWages float64 `json:"line5d_addl_medicare_wages"`
	TotalTaxes        float64 `json:"line10_total_taxes"`
}

// GenerateW2 creates W-2 data for an employee for a tax year.
func (fg *FormGenerator) GenerateW2(ctx context.Context, companyID int64, employeeID int64, taxYear int32) (*W2Data, error) {
	ytd, err := fg.queries.GetYTDPayrollTotals(ctx, store.GetYTDPayrollTotalsParams{
		EmployeeID: employeeID,
		CompanyID:  companyID,
		Year:       taxYear,
	})
	if err != nil {
		return nil, fmt.Errorf("get YTD totals: %w", err)
	}

	_ = ytd // Use YTD data to populate W-2 boxes
	// Full implementation aggregates all payroll_items for the year

	return &W2Data{}, nil
}

// Generate941 creates Form 941 data for a quarter.
func (fg *FormGenerator) Generate941(ctx context.Context, companyID int64, taxYear int32, quarter int) (*Form941Data, error) {
	if quarter < 1 || quarter > 4 {
		return nil, fmt.Errorf("quarter must be 1-4")
	}

	// Aggregate payroll runs for the quarter
	// Full implementation queries payroll_items joined with payroll_runs/cycles
	// filtered by period_start within the quarter date range

	return &Form941Data{
		Quarter: quarter,
		Year:    taxYear,
	}, nil
}
