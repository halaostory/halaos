package compliance

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/store"
)

// FormGenerator generates Philippine government compliance forms.
type FormGenerator struct {
	queries *store.Queries
}

func NewFormGenerator(queries *store.Queries) *FormGenerator {
	return &FormGenerator{queries: queries}
}

// --- Helper ---

func numToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, _ := n.Float64Value()
	if !f.Valid {
		return 0
	}
	return math.Round(f.Float64*100) / 100
}

func ptrOr(s *string, def string) string {
	if s != nil {
		return *s
	}
	return def
}

// --- BIR 2316: Annual Certificate of Compensation Payment/Tax Withheld ---

type BIR2316Employee struct {
	EmployeeNo      string  `json:"employee_no"`
	FullName        string  `json:"full_name"`
	TIN             string  `json:"tin"`
	TotalBasicPay   float64 `json:"total_basic_pay"`
	TotalGrossPay   float64 `json:"total_gross_pay"`
	TotalSSS        float64 `json:"total_sss"`
	TotalPhilHealth float64 `json:"total_philhealth"`
	TotalPagIBIG    float64 `json:"total_pagibig"`
	TotalTaxWithheld float64 `json:"total_tax_withheld"`
	NetPayAfterTax  float64 `json:"net_pay_after_tax"`
	ThirteenthMonth float64 `json:"thirteenth_month"`
}

type BIR2316Form struct {
	FormType  string            `json:"form_type"`
	TaxYear   int32             `json:"tax_year"`
	Company   CompanyInfo       `json:"company"`
	Employees []BIR2316Employee `json:"employees"`
	Summary   FormSummary       `json:"summary"`
}

type CompanyInfo struct {
	Name      string `json:"name"`
	LegalName string `json:"legal_name"`
	TIN       string `json:"tin"`
	BIR_RDO   string `json:"bir_rdo"`
	Address   string `json:"address"`
}

type FormSummary struct {
	TotalEmployees    int     `json:"total_employees"`
	TotalGrossPay     float64 `json:"total_gross_pay"`
	TotalSSS          float64 `json:"total_sss"`
	TotalPhilHealth   float64 `json:"total_philhealth"`
	TotalPagIBIG      float64 `json:"total_pagibig"`
	TotalTaxWithheld  float64 `json:"total_tax_withheld"`
	TotalNetPay       float64 `json:"total_net_pay"`
}

func (fg *FormGenerator) GenerateBIR2316(ctx context.Context, companyID int64, taxYear int32) (*BIR2316Form, error) {
	company, err := fg.queries.GetCompanyByID(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("get company: %w", err)
	}

	yearStart := time.Date(int(taxYear), 1, 1, 0, 0, 0, 0, time.UTC)
	yearEnd := time.Date(int(taxYear)+1, 1, 1, 0, 0, 0, 0, time.UTC)
	items, err := fg.queries.GetPayrollItemsForYear(ctx, store.GetPayrollItemsForYearParams{
		CompanyID:     companyID,
		PeriodStart:   yearStart,
		PeriodStart_2: yearEnd,
	})
	if err != nil {
		return nil, fmt.Errorf("get payroll items: %w", err)
	}

	// Aggregate by employee
	type empAgg struct {
		row        store.GetPayrollItemsForYearRow
		basicPay   float64
		grossPay   float64
		sss        float64
		philhealth float64
		pagibig    float64
		tax        float64
		netPay     float64
	}
	aggMap := make(map[int64]*empAgg)

	for _, item := range items {
		agg, ok := aggMap[item.EmployeeID]
		if !ok {
			agg = &empAgg{row: item}
			aggMap[item.EmployeeID] = agg
		}
		agg.basicPay += numToFloat(item.BasicPay)
		agg.grossPay += numToFloat(item.GrossPay)
		agg.sss += numToFloat(item.SssEe)
		agg.philhealth += numToFloat(item.PhilhealthEe)
		agg.pagibig += numToFloat(item.PagibigEe)
		agg.tax += numToFloat(item.WithholdingTax)
		agg.netPay += numToFloat(item.NetPay)
	}

	// Get 13th month pay data
	thirteenthPays, err := fg.queries.List13thMonthPay(ctx, store.List13thMonthPayParams{
		CompanyID: companyID,
		Year:      taxYear,
	})
	if err != nil {
		thirteenthPays = nil
	}
	thirteenthMap := make(map[int64]float64)
	for _, tp := range thirteenthPays {
		thirteenthMap[tp.EmployeeID] = numToFloat(tp.Amount)
	}

	form := &BIR2316Form{
		FormType: "BIR_2316",
		TaxYear:  taxYear,
		Company: CompanyInfo{
			Name:      company.Name,
			LegalName: ptrOr(company.LegalName, company.Name),
			TIN:       ptrOr(company.Tin, ""),
			BIR_RDO:   ptrOr(company.BirRdo, ""),
			Address:   ptrOr(company.Address, ""),
		},
	}

	var summary FormSummary
	for _, agg := range aggMap {
		r := agg.row
		fullName := r.LastName + ", " + r.FirstName
		if r.MiddleName != nil && *r.MiddleName != "" {
			fullName += " " + *r.MiddleName
		}

		emp := BIR2316Employee{
			EmployeeNo:      r.EmployeeNo,
			FullName:        fullName,
			TIN:             ptrOr(r.Tin, ""),
			TotalBasicPay:   math.Round(agg.basicPay*100) / 100,
			TotalGrossPay:   math.Round(agg.grossPay*100) / 100,
			TotalSSS:        math.Round(agg.sss*100) / 100,
			TotalPhilHealth: math.Round(agg.philhealth*100) / 100,
			TotalPagIBIG:    math.Round(agg.pagibig*100) / 100,
			TotalTaxWithheld: math.Round(agg.tax*100) / 100,
			NetPayAfterTax:  math.Round(agg.netPay*100) / 100,
			ThirteenthMonth: thirteenthMap[r.EmployeeID],
		}
		form.Employees = append(form.Employees, emp)
		summary.TotalGrossPay += emp.TotalGrossPay
		summary.TotalSSS += emp.TotalSSS
		summary.TotalPhilHealth += emp.TotalPhilHealth
		summary.TotalPagIBIG += emp.TotalPagIBIG
		summary.TotalTaxWithheld += emp.TotalTaxWithheld
		summary.TotalNetPay += emp.NetPayAfterTax
	}
	summary.TotalEmployees = len(form.Employees)
	form.Summary = summary

	return form, nil
}

// --- SSS R-3: Monthly Contribution Report ---

type SSSR3Employee struct {
	SSSNo    string  `json:"sss_no"`
	FullName string  `json:"full_name"`
	EEShare  float64 `json:"ee_share"`
	ERShare  float64 `json:"er_share"`
	EC       float64 `json:"ec"`
	Total    float64 `json:"total"`
}

type SSSR3Form struct {
	FormType  string          `json:"form_type"`
	TaxYear   int32           `json:"tax_year"`
	Period    string          `json:"period"`
	Company   CompanyInfo     `json:"company"`
	Employees []SSSR3Employee `json:"employees"`
	Totals    struct {
		TotalEE    float64 `json:"total_ee"`
		TotalER    float64 `json:"total_er"`
		TotalEC    float64 `json:"total_ec"`
		GrandTotal float64 `json:"grand_total"`
	} `json:"totals"`
}

func (fg *FormGenerator) GenerateSSSR3(ctx context.Context, companyID int64, year int32, month int) (*SSSR3Form, error) {
	company, err := fg.queries.GetCompanyByID(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("get company: %w", err)
	}

	periodStart := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, -1)

	items, err := fg.queries.GetPayrollItemsWithEmployeeForPeriod(ctx, store.GetPayrollItemsWithEmployeeForPeriodParams{
		CompanyID:   companyID,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
	})
	if err != nil {
		return nil, fmt.Errorf("get payroll items: %w", err)
	}

	// Aggregate by employee (may have multiple payroll items in a month)
	type sssAgg struct {
		row store.GetPayrollItemsWithEmployeeForPeriodRow
		ee  float64
		er  float64
		ec  float64
	}
	aggMap := make(map[int64]*sssAgg)
	for _, item := range items {
		agg, ok := aggMap[item.EmployeeID]
		if !ok {
			agg = &sssAgg{row: item}
			aggMap[item.EmployeeID] = agg
		}
		agg.ee += numToFloat(item.SssEe)
		agg.er += numToFloat(item.SssEr)
		agg.ec += numToFloat(item.SssEc)
	}

	form := &SSSR3Form{
		FormType: "SSS_R3",
		TaxYear:  year,
		Period:   fmt.Sprintf("%04d-%02d", year, month),
		Company: CompanyInfo{
			Name:      company.Name,
			LegalName: ptrOr(company.LegalName, company.Name),
			TIN:       ptrOr(company.Tin, ""),
			BIR_RDO:   ptrOr(company.BirRdo, ""),
			Address:   ptrOr(company.Address, ""),
		},
	}

	for _, agg := range aggMap {
		r := agg.row
		fullName := r.LastName + ", " + r.FirstName
		total := agg.ee + agg.er + agg.ec

		form.Employees = append(form.Employees, SSSR3Employee{
			SSSNo:    ptrOr(r.SssNo, ""),
			FullName: fullName,
			EEShare:  math.Round(agg.ee*100) / 100,
			ERShare:  math.Round(agg.er*100) / 100,
			EC:       math.Round(agg.ec*100) / 100,
			Total:    math.Round(total*100) / 100,
		})
		form.Totals.TotalEE += agg.ee
		form.Totals.TotalER += agg.er
		form.Totals.TotalEC += agg.ec
		form.Totals.GrandTotal += total
	}
	form.Totals.TotalEE = math.Round(form.Totals.TotalEE*100) / 100
	form.Totals.TotalER = math.Round(form.Totals.TotalER*100) / 100
	form.Totals.TotalEC = math.Round(form.Totals.TotalEC*100) / 100
	form.Totals.GrandTotal = math.Round(form.Totals.GrandTotal*100) / 100

	return form, nil
}

// --- PhilHealth RF1: Monthly Remittance Report ---

type PhilHealthRF1Employee struct {
	PhilHealthNo string  `json:"philhealth_no"`
	FullName     string  `json:"full_name"`
	EEShare      float64 `json:"ee_share"`
	ERShare      float64 `json:"er_share"`
	Total        float64 `json:"total"`
}

type PhilHealthRF1Form struct {
	FormType  string                  `json:"form_type"`
	TaxYear   int32                   `json:"tax_year"`
	Period    string                  `json:"period"`
	Company   CompanyInfo             `json:"company"`
	Employees []PhilHealthRF1Employee `json:"employees"`
	Totals    struct {
		TotalEE    float64 `json:"total_ee"`
		TotalER    float64 `json:"total_er"`
		GrandTotal float64 `json:"grand_total"`
	} `json:"totals"`
}

func (fg *FormGenerator) GeneratePhilHealthRF1(ctx context.Context, companyID int64, year int32, month int) (*PhilHealthRF1Form, error) {
	company, err := fg.queries.GetCompanyByID(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("get company: %w", err)
	}

	periodStart := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, -1)

	items, err := fg.queries.GetPayrollItemsWithEmployeeForPeriod(ctx, store.GetPayrollItemsWithEmployeeForPeriodParams{
		CompanyID:   companyID,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
	})
	if err != nil {
		return nil, fmt.Errorf("get payroll items: %w", err)
	}

	type phAgg struct {
		row store.GetPayrollItemsWithEmployeeForPeriodRow
		ee  float64
		er  float64
	}
	aggMap := make(map[int64]*phAgg)
	for _, item := range items {
		agg, ok := aggMap[item.EmployeeID]
		if !ok {
			agg = &phAgg{row: item}
			aggMap[item.EmployeeID] = agg
		}
		agg.ee += numToFloat(item.PhilhealthEe)
		agg.er += numToFloat(item.PhilhealthEr)
	}

	form := &PhilHealthRF1Form{
		FormType: "PHILHEALTH_RF1",
		TaxYear:  year,
		Period:   fmt.Sprintf("%04d-%02d", year, month),
		Company: CompanyInfo{
			Name:      company.Name,
			LegalName: ptrOr(company.LegalName, company.Name),
			TIN:       ptrOr(company.Tin, ""),
			BIR_RDO:   ptrOr(company.BirRdo, ""),
			Address:   ptrOr(company.Address, ""),
		},
	}

	for _, agg := range aggMap {
		r := agg.row
		fullName := r.LastName + ", " + r.FirstName
		total := agg.ee + agg.er

		form.Employees = append(form.Employees, PhilHealthRF1Employee{
			PhilHealthNo: ptrOr(r.PhilhealthNo, ""),
			FullName:     fullName,
			EEShare:      math.Round(agg.ee*100) / 100,
			ERShare:      math.Round(agg.er*100) / 100,
			Total:        math.Round(total*100) / 100,
		})
		form.Totals.TotalEE += agg.ee
		form.Totals.TotalER += agg.er
		form.Totals.GrandTotal += total
	}
	form.Totals.TotalEE = math.Round(form.Totals.TotalEE*100) / 100
	form.Totals.TotalER = math.Round(form.Totals.TotalER*100) / 100
	form.Totals.GrandTotal = math.Round(form.Totals.GrandTotal*100) / 100

	return form, nil
}

// --- BIR 1601-C: Monthly Remittance of Withholding Tax ---

type BIR1601CEmployee struct {
	EmployeeNo string  `json:"employee_no"`
	FullName   string  `json:"full_name"`
	TIN        string  `json:"tin"`
	GrossPay   float64 `json:"gross_pay"`
	TaxableIncome float64 `json:"taxable_income"`
	TaxWithheld float64 `json:"tax_withheld"`
}

type BIR1601CForm struct {
	FormType  string              `json:"form_type"`
	TaxYear   int32               `json:"tax_year"`
	Period    string              `json:"period"`
	Company   CompanyInfo         `json:"company"`
	Employees []BIR1601CEmployee  `json:"employees"`
	Totals    struct {
		TotalGross      float64 `json:"total_gross"`
		TotalTaxable    float64 `json:"total_taxable"`
		TotalTaxWithheld float64 `json:"total_tax_withheld"`
		EmployeeCount   int     `json:"employee_count"`
	} `json:"totals"`
}

func (fg *FormGenerator) GenerateBIR1601C(ctx context.Context, companyID int64, year int32, month int) (*BIR1601CForm, error) {
	company, err := fg.queries.GetCompanyByID(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("get company: %w", err)
	}

	periodStart := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, -1)

	items, err := fg.queries.GetPayrollItemsWithEmployeeForPeriod(ctx, store.GetPayrollItemsWithEmployeeForPeriodParams{
		CompanyID:   companyID,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
	})
	if err != nil {
		return nil, fmt.Errorf("get payroll items: %w", err)
	}

	type taxAgg struct {
		row     store.GetPayrollItemsWithEmployeeForPeriodRow
		gross   float64
		taxable float64
		tax     float64
	}
	aggMap := make(map[int64]*taxAgg)
	for _, item := range items {
		agg, ok := aggMap[item.EmployeeID]
		if !ok {
			agg = &taxAgg{row: item}
			aggMap[item.EmployeeID] = agg
		}
		agg.gross += numToFloat(item.GrossPay)
		agg.taxable += numToFloat(item.TaxableIncome)
		agg.tax += numToFloat(item.WithholdingTax)
	}

	form := &BIR1601CForm{
		FormType: "BIR_1601C",
		TaxYear:  year,
		Period:   fmt.Sprintf("%04d-%02d", year, month),
		Company: CompanyInfo{
			Name:      company.Name,
			LegalName: ptrOr(company.LegalName, company.Name),
			TIN:       ptrOr(company.Tin, ""),
			BIR_RDO:   ptrOr(company.BirRdo, ""),
			Address:   ptrOr(company.Address, ""),
		},
	}

	for _, agg := range aggMap {
		r := agg.row
		fullName := r.LastName + ", " + r.FirstName

		form.Employees = append(form.Employees, BIR1601CEmployee{
			EmployeeNo:    r.EmployeeNo,
			FullName:      fullName,
			TIN:           ptrOr(r.Tin, ""),
			GrossPay:      math.Round(agg.gross*100) / 100,
			TaxableIncome: math.Round(agg.taxable*100) / 100,
			TaxWithheld:   math.Round(agg.tax*100) / 100,
		})
		form.Totals.TotalGross += agg.gross
		form.Totals.TotalTaxable += agg.taxable
		form.Totals.TotalTaxWithheld += agg.tax
	}
	form.Totals.TotalGross = math.Round(form.Totals.TotalGross*100) / 100
	form.Totals.TotalTaxable = math.Round(form.Totals.TotalTaxable*100) / 100
	form.Totals.TotalTaxWithheld = math.Round(form.Totals.TotalTaxWithheld*100) / 100
	form.Totals.EmployeeCount = len(form.Employees)

	return form, nil
}

// GenerateAndStore generates a form and saves it to the database.
func (fg *FormGenerator) GenerateAndStore(ctx context.Context, companyID int64, formType string, taxYear int32, month int) (*store.GovernmentForm, error) {
	var payload interface{}
	var period *string

	switch formType {
	case "BIR_2316":
		form, err := fg.GenerateBIR2316(ctx, companyID, taxYear)
		if err != nil {
			return nil, err
		}
		payload = form

	case "SSS_R3":
		if month < 1 || month > 12 {
			return nil, fmt.Errorf("month must be 1-12 for SSS R-3")
		}
		form, err := fg.GenerateSSSR3(ctx, companyID, taxYear, month)
		if err != nil {
			return nil, err
		}
		payload = form
		p := form.Period
		period = &p

	case "PHILHEALTH_RF1":
		if month < 1 || month > 12 {
			return nil, fmt.Errorf("month must be 1-12 for PhilHealth RF1")
		}
		form, err := fg.GeneratePhilHealthRF1(ctx, companyID, taxYear, month)
		if err != nil {
			return nil, err
		}
		payload = form
		p := form.Period
		period = &p

	case "BIR_1601C":
		if month < 1 || month > 12 {
			return nil, fmt.Errorf("month must be 1-12 for BIR 1601-C")
		}
		form, err := fg.GenerateBIR1601C(ctx, companyID, taxYear, month)
		if err != nil {
			return nil, err
		}
		payload = form
		p := form.Period
		period = &p

	default:
		return nil, fmt.Errorf("unsupported form type: %s", formType)
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	govForm, err := fg.queries.CreateGovernmentForm(ctx, store.CreateGovernmentFormParams{
		CompanyID: companyID,
		FormType:  formType,
		TaxYear:   taxYear,
		Period:    period,
		Payload:   payloadJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("store form: %w", err)
	}

	return &govForm, nil
}
