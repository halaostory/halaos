package compliance

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/halaostory/halaos/internal/store"
)

// PagibigMCRFEmployee is one member line of the monthly remittance.
type PagibigMCRFEmployee struct {
	PagibigNo  string  `json:"pagibig_no"`
	EmployeeNo string  `json:"employee_no"`
	LastName   string  `json:"last_name"`
	FirstName  string  `json:"first_name"`
	MiddleName string  `json:"middle_name"`
	EEShare    float64 `json:"ee_share"`
	ERShare    float64 `json:"er_share"`
	Total      float64 `json:"total"`
}

// PagibigMCRFForm is the Member Contribution Remittance Form data.
type PagibigMCRFForm struct {
	FormType   string                `json:"form_type"`
	TaxYear    int32                 `json:"tax_year"`
	Period     string                `json:"period"`
	Company    CompanyInfo           `json:"company"`
	EmployerID string                `json:"employer_id"`
	Employees  []PagibigMCRFEmployee `json:"employees"`
	Totals     struct {
		TotalEE    float64 `json:"total_ee"`
		TotalER    float64 `json:"total_er"`
		GrandTotal float64 `json:"grand_total"`
	} `json:"totals"`
}

// GeneratePagibigMCRF aggregates Pag-IBIG contributions for a month.
func (fg *FormGenerator) GeneratePagibigMCRF(ctx context.Context, companyID int64, year int32, month int) (*PagibigMCRFForm, error) {
	company, err := fg.queries.GetCompanyByID(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("get company: %w", err)
	}
	periodStart := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	items, err := fg.queries.GetPayrollItemsWithEmployeeForPeriod(ctx, store.GetPayrollItemsWithEmployeeForPeriodParams{
		CompanyID:   companyID,
		PeriodStart: periodStart,
		PeriodEnd:   periodStart.AddDate(0, 1, -1),
	})
	if err != nil {
		return nil, fmt.Errorf("get payroll items: %w", err)
	}
	agg := map[int64]*PagibigMCRFEmployee{}
	for _, it := range items {
		e, ok := agg[it.EmployeeID]
		if !ok {
			e = &PagibigMCRFEmployee{
				PagibigNo: ptrOr(it.PagibigNo, ""), EmployeeNo: it.EmployeeNo,
				LastName: it.LastName, FirstName: it.FirstName, MiddleName: ptrOr(it.MiddleName, ""),
			}
			agg[it.EmployeeID] = e
		}
		e.EEShare += numToFloat(it.PagibigEe)
		e.ERShare += numToFloat(it.PagibigEr)
	}
	form := &PagibigMCRFForm{
		FormType: "PAGIBIG_MCRF", TaxYear: year, Period: fmt.Sprintf("%04d-%02d", year, month),
		Company: CompanyInfo{
			Name: company.Name, LegalName: ptrOr(company.LegalName, company.Name),
			TIN: ptrOr(company.Tin, ""), BIR_RDO: ptrOr(company.BirRdo, ""), Address: ptrOr(company.Address, ""),
		},
		EmployerID: ptrOr(company.PagibigErNo, ""),
		Employees:  []PagibigMCRFEmployee{},
	}
	for _, e := range agg {
		e.EEShare = math.Round(e.EEShare*100) / 100
		e.ERShare = math.Round(e.ERShare*100) / 100
		e.Total = math.Round((e.EEShare+e.ERShare)*100) / 100
		form.Employees = append(form.Employees, *e)
		form.Totals.TotalEE += e.EEShare
		form.Totals.TotalER += e.ERShare
	}
	sort.Slice(form.Employees, func(i, j int) bool {
		if form.Employees[i].LastName != form.Employees[j].LastName {
			return form.Employees[i].LastName < form.Employees[j].LastName
		}
		return form.Employees[i].FirstName < form.Employees[j].FirstName
	})
	form.Totals.TotalEE = math.Round(form.Totals.TotalEE*100) / 100
	form.Totals.TotalER = math.Round(form.Totals.TotalER*100) / 100
	form.Totals.GrandTotal = math.Round((form.Totals.TotalEE+form.Totals.TotalER)*100) / 100
	return form, nil
}
