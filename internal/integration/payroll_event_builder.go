package integration

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/store"
)

// PayrollEventBuilder constructs accounting events from payroll DB data.
type PayrollEventBuilder struct {
	queries *store.Queries
}

// NewPayrollEventBuilder creates a new builder.
func NewPayrollEventBuilder(queries *store.Queries) *PayrollEventBuilder {
	return &PayrollEventBuilder{queries: queries}
}

// BuildPayrollRunCompleted loads a completed payroll run and constructs the event.
func (b *PayrollEventBuilder) BuildPayrollRunCompleted(ctx context.Context, companyID, cycleID, runID int64) (*PayrollRunCompletedEvent, error) {
	cycle, err := b.queries.GetPayrollCycle(ctx, store.GetPayrollCycleParams{
		ID: cycleID, CompanyID: companyID,
	})
	if err != nil {
		return nil, fmt.Errorf("get cycle: %w", err)
	}

	run, err := b.queries.GetPayrollRun(ctx, store.GetPayrollRunParams{
		ID: runID, CompanyID: companyID,
	})
	if err != nil {
		return nil, fmt.Errorf("get run: %w", err)
	}

	company, err := b.queries.GetCompanyByID(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("get company: %w", err)
	}

	items, err := b.queries.GetPayrollItemsForAccounting(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("get payroll items: %w", err)
	}

	jurisdiction := company.Country
	if jurisdiction == "" {
		jurisdiction = "PH"
	}

	event := &PayrollRunCompletedEvent{
		EventID:      uuid.New().String(),
		EventType:    EventPayrollRunCompleted,
		EventVersion: 1,
		OccurredAt:   time.Now().UTC(),
		HRCompanyID:  companyID,
		Jurisdiction: jurisdiction,
		Currency:     company.Currency,
		PayrollRunID: runID,
		CycleName:    cycle.Name,
		PayDate:      formatDate(cycle.PayDate),
		PeriodStart:  formatDate(cycle.PeriodStart),
		PeriodEnd:    formatDate(cycle.PeriodEnd),
	}

	// Aggregate totals
	totalGross := new(big.Float)
	totalDeductions := new(big.Float)
	totalNet := new(big.Float)
	headCount := 0

	// Department aggregation
	deptMap := map[int64]*DeptSummary{}

	// Statutory payable accumulators
	sssEE, sssER, sssEC := new(big.Float), new(big.Float), new(big.Float)
	philEE, philER := new(big.Float), new(big.Float)
	pagEE, pagER := new(big.Float), new(big.Float)
	totalWHT := new(big.Float)

	for _, item := range items {
		headCount++
		gross := numericToFloat(item.GrossPay)
		deductions := numericToFloat(item.TotalDeductions)
		net := numericToFloat(item.NetPay)

		totalGross.Add(totalGross, gross)
		totalDeductions.Add(totalDeductions, deductions)
		totalNet.Add(totalNet, net)

		// Department breakdown
		deptID := int64(0)
		deptName := item.DepartmentName
		if item.DepartmentID != nil {
			deptID = *item.DepartmentID
		}
		ds, ok := deptMap[deptID]
		if !ok {
			ds = &DeptSummary{
				DepartmentID:   deptID,
				DepartmentName: deptName,
			}
			deptMap[deptID] = ds
		}
		ds.HeadCount++
		gf, _ := gross.Float64()
		nf, _ := net.Float64()
		addToString(&ds.GrossPay, gf)
		addToString(&ds.NetPay, nf)

		// Employee line
		empLine := EmployeePayLine{
			EmployeeID:   item.EmployeeID,
			EmployeeNo:   item.EmployeeNo,
			FullName:     item.FirstName + " " + item.LastName,
			TIN:          ptrStr(item.Tin),
			DepartmentID: deptID,
			NetPay:       numericStr(item.NetPay),
		}

		// Earnings breakdown
		empLine.Earnings = append(empLine.Earnings, EarningLine{
			Code: "basic_pay", Label: "Basic Pay", Amount: numericStr(item.BasicPay),
		})
		if isPositive(item.HolidayPay) {
			empLine.Earnings = append(empLine.Earnings, EarningLine{
				Code: "holiday_pay", Label: "Holiday Pay", Amount: numericStr(item.HolidayPay),
			})
		}
		if isPositive(item.NightDiff) {
			empLine.Earnings = append(empLine.Earnings, EarningLine{
				Code: "night_diff", Label: "Night Differential", Amount: numericStr(item.NightDiff),
			})
		}
		if isPositive(item.BonusPay) {
			empLine.Earnings = append(empLine.Earnings, EarningLine{
				Code: "bonus_pay", Label: "Bonus", Amount: numericStr(item.BonusPay),
			})
		}

		// Deductions
		if isPositive(item.SssEe) {
			empLine.Deductions = append(empLine.Deductions, DeductionLine{
				Code: "sss_employee", Label: "SSS Employee", Amount: numericStr(item.SssEe),
			})
		}
		if isPositive(item.PhilhealthEe) {
			empLine.Deductions = append(empLine.Deductions, DeductionLine{
				Code: "philhealth_employee", Label: "PhilHealth Employee", Amount: numericStr(item.PhilhealthEe),
			})
		}
		if isPositive(item.PagibigEe) {
			empLine.Deductions = append(empLine.Deductions, DeductionLine{
				Code: "pagibig_employee", Label: "Pag-IBIG Employee", Amount: numericStr(item.PagibigEe),
			})
		}
		if isPositive(item.WithholdingTax) {
			empLine.Deductions = append(empLine.Deductions, DeductionLine{
				Code: "withholding_tax", Label: "Withholding Tax", Amount: numericStr(item.WithholdingTax),
			})
		}

		event.EmployeeLines = append(event.EmployeeLines, empLine)

		// Withholding line
		if isPositive(item.WithholdingTax) {
			event.WithholdingLines = append(event.WithholdingLines, WithholdingLine{
				EmployeeID: item.EmployeeID,
				EmployeeNo: item.EmployeeNo,
				FullName:   item.FirstName + " " + item.LastName,
				TIN:        ptrStr(item.Tin),
				TaxAmount:  numericStr(item.WithholdingTax),
			})
		}

		// Accumulate statutory
		sssEE.Add(sssEE, numericToFloat(item.SssEe))
		sssER.Add(sssER, numericToFloat(item.SssEr))
		sssEC.Add(sssEC, numericToFloat(item.SssEc))
		philEE.Add(philEE, numericToFloat(item.PhilhealthEe))
		philER.Add(philER, numericToFloat(item.PhilhealthEr))
		pagEE.Add(pagEE, numericToFloat(item.PagibigEe))
		pagER.Add(pagER, numericToFloat(item.PagibigEr))
		totalWHT.Add(totalWHT, numericToFloat(item.WithholdingTax))
	}

	event.Totals = PayrollTotals{
		GrossPay:           floatStr(totalGross),
		TotalDeductions:    floatStr(totalDeductions),
		TotalContributions: floatStr(new(big.Float).Add(new(big.Float).Add(sssER, philER), pagER)),
		NetPay:             floatStr(totalNet),
		HeadCount:          headCount,
	}

	for _, ds := range deptMap {
		event.DepartmentBreakdown = append(event.DepartmentBreakdown, *ds)
	}

	// Statutory payables
	addStatutory := func(code, label string, ee, er *big.Float) {
		total := new(big.Float).Add(ee, er)
		if total.Sign() > 0 {
			event.StatutoryPayables = append(event.StatutoryPayables, StatutoryPayable{
				Code:           code,
				Label:          label,
				EmployeeAmount: floatStr(ee),
				EmployerAmount: floatStr(er),
				TotalAmount:    floatStr(total),
			})
		}
	}
	sssTotal := new(big.Float).Add(sssER, sssEC)
	addStatutory("sss", "SSS", sssEE, sssTotal)
	addStatutory("philhealth", "PhilHealth", philEE, philER)
	addStatutory("pagibig", "Pag-IBIG", pagEE, pagER)

	_ = run // used for validation context
	return event, nil
}

// helpers

func numericToFloat(n pgtype.Numeric) *big.Float {
	if !n.Valid {
		return new(big.Float)
	}
	f := new(big.Float).SetInt(n.Int)
	if n.Exp < 0 {
		divisor := new(big.Float).SetFloat64(1)
		for i := 0; i < int(-n.Exp); i++ {
			divisor.Mul(divisor, new(big.Float).SetFloat64(10))
		}
		f.Quo(f, divisor)
	} else if n.Exp > 0 {
		for i := 0; i < int(n.Exp); i++ {
			f.Mul(f, new(big.Float).SetFloat64(10))
		}
	}
	return f
}

func numericStr(n pgtype.Numeric) string {
	return floatStr(numericToFloat(n))
}

func floatStr(f *big.Float) string {
	return f.Text('f', 2)
}

func isPositive(n pgtype.Numeric) bool {
	return numericToFloat(n).Sign() > 0
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func addToString(s *string, val float64) {
	if *s == "" {
		*s = "0.00"
	}
	current, _, _ := new(big.Float).Parse(*s, 10)
	current.Add(current, new(big.Float).SetFloat64(val))
	*s = current.Text('f', 2)
}

func formatDate(t time.Time) string {
	return t.Format("2006-01-02")
}
