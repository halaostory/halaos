package payroll

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/store"
)

// Calculator handles payroll computation for a company.
type Calculator struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

func NewCalculator(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Calculator {
	return &Calculator{queries: queries, pool: pool, logger: logger}
}

// EmployeePayData holds all intermediate calculation data for one employee.
type EmployeePayData struct {
	EmployeeID  int64
	BasicSalary float64 // monthly basic

	// Attendance
	DaysWorked       float64
	HoursWorked      float64
	OvertimeHours    float64
	LateMinutes      int64
	UndertimeMinutes int64
	UnpaidLeaveDays  float64
	PaidLeaveDays    float64

	// Night differential & holiday
	NightHours         float64
	RegularHolidayDays int64
	SpecialHolidayDays int64
	OTRegular          float64
	OTRestDay          float64
	OTHoliday          float64
	OTSpecialHoliday   float64

	// Computed
	BasicPay           float64
	OvertimePay        float64
	NightDiff          float64
	HolidayPay         float64
	LateDeduction      float64
	UndertimeDeduction float64
	LeaveDeduction     float64
	GrossPay           float64

	// Government contributions (employee share)
	SSSEE          float64
	SSSER          float64
	SSSEC          float64
	PhilHealthEE   float64
	PhilHealthER   float64
	PagIBIGEE      float64
	PagIBIGER      float64
	WithholdingTax float64

	// Final
	TotalDeductions float64
	TaxableIncome   float64
	NetPay          float64

	// Breakdown for JSON storage
	Breakdown map[string]interface{}
}

// RunPayroll executes payroll calculation for a given run.
func (calc *Calculator) RunPayroll(ctx context.Context, runID int64, companyID int64) error {
	calc.logger.Info("starting payroll calculation", "run_id", runID, "company_id", companyID)

	// Mark run as running
	if err := calc.queries.UpdatePayrollRun(ctx, store.UpdatePayrollRunParams{
		ID:     runID,
		Status: "running",
	}); err != nil {
		return fmt.Errorf("mark run as running: %w", err)
	}

	// Get the run and cycle details
	run, err := calc.queries.GetPayrollRun(ctx, store.GetPayrollRunParams{
		ID:        runID,
		CompanyID: companyID,
	})
	if err != nil {
		return calc.failRun(ctx, runID, fmt.Errorf("get payroll run: %w", err))
	}

	cycle, err := calc.queries.GetPayrollCycle(ctx, store.GetPayrollCycleParams{
		ID:        run.CycleID,
		CompanyID: companyID,
	})
	if err != nil {
		return calc.failRun(ctx, runID, fmt.Errorf("get payroll cycle: %w", err))
	}

	periodStart := cycle.PeriodStart
	periodEnd := cycle.PeriodEnd
	effectiveDate := periodEnd

	// Get active employees
	employees, err := calc.queries.ListActiveEmployees(ctx, companyID)
	if err != nil {
		return calc.failRun(ctx, runID, fmt.Errorf("list active employees: %w", err))
	}

	if len(employees) == 0 {
		calc.logger.Warn("no active employees found", "company_id", companyID)
		return calc.completeRun(ctx, runID, 0, 0, 0, 0)
	}

	// Build data maps
	salaryMap, err := calc.buildSalaryMap(ctx, companyID, effectiveDate)
	if err != nil {
		return calc.failRun(ctx, runID, fmt.Errorf("build salary map: %w", err))
	}

	attendanceMap, err := calc.buildAttendanceMap(ctx, companyID, periodStart, periodEnd)
	if err != nil {
		return calc.failRun(ctx, runID, fmt.Errorf("build attendance map: %w", err))
	}

	otMap, err := calc.buildOvertimeMap(ctx, companyID, periodStart, periodEnd)
	if err != nil {
		return calc.failRun(ctx, runID, fmt.Errorf("build overtime map: %w", err))
	}

	otTypeMap, err := calc.buildOTByTypeMap(ctx, companyID, periodStart, periodEnd)
	if err != nil {
		return calc.failRun(ctx, runID, fmt.Errorf("build ot type map: %w", err))
	}

	leaveMap, err := calc.buildLeaveMap(ctx, companyID, periodStart, periodEnd)
	if err != nil {
		return calc.failRun(ctx, runID, fmt.Errorf("build leave map: %w", err))
	}

	nightMap, err := calc.buildNightHoursMap(ctx, companyID, periodStart, periodEnd)
	if err != nil {
		return calc.failRun(ctx, runID, fmt.Errorf("build night hours map: %w", err))
	}

	holidayAttMap, err := calc.buildHolidayAttendanceMap(ctx, companyID, periodStart, periodEnd)
	if err != nil {
		return calc.failRun(ctx, runID, fmt.Errorf("build holiday attendance map: %w", err))
	}

	// Calculate working days in period (Mon-Fri)
	workingDaysInPeriod := countWorkingDays(periodStart, periodEnd)

	var totalGross, totalDeductions, totalNet float64
	var processedCount int32

	for _, emp := range employees {
		salary, hasSalary := salaryMap[emp.ID]
		if !hasSalary {
			calc.logger.Warn("employee has no salary record, skipping", "employee_id", emp.ID)
			continue
		}

		pd := EmployeePayData{
			EmployeeID:  emp.ID,
			BasicSalary: salary,
			Breakdown:   make(map[string]interface{}),
		}

		// Attendance data
		if att, ok := attendanceMap[emp.ID]; ok {
			pd.DaysWorked = att.DaysWorked
			pd.HoursWorked = att.HoursWorked
			pd.LateMinutes = att.LateMinutes
			pd.UndertimeMinutes = att.UndertimeMinutes
		} else {
			pd.DaysWorked = float64(workingDaysInPeriod)
			pd.HoursWorked = float64(workingDaysInPeriod) * 8
		}

		// Overtime
		if otHours, ok := otMap[emp.ID]; ok {
			pd.OvertimeHours = otHours
		}

		// OT by type
		if ot, ok := otTypeMap[emp.ID]; ok {
			pd.OTRegular = ot.Regular
			pd.OTRestDay = ot.RestDay
			pd.OTHoliday = ot.Holiday
			pd.OTSpecialHoliday = ot.SpecialHoliday
		}

		// Leave
		if lv, ok := leaveMap[emp.ID]; ok {
			pd.UnpaidLeaveDays = lv.UnpaidDays
			pd.PaidLeaveDays = lv.PaidDays
		}

		// Night hours
		if nh, ok := nightMap[emp.ID]; ok {
			pd.NightHours = nh
		}

		// Holiday attendance
		if ha, ok := holidayAttMap[emp.ID]; ok {
			pd.RegularHolidayDays = ha.RegularDays
			pd.SpecialHolidayDays = ha.SpecialNonWorkDays
		}

		// Compute pay
		calc.computePay(&pd, workingDaysInPeriod)

		// Government contributions
		calc.computeContributions(ctx, &pd, effectiveDate)

		// Withholding tax
		calc.computeWithholdingTax(ctx, &pd, effectiveDate)

		// Finals
		pd.TotalDeductions = pd.SSSEE + pd.PhilHealthEE + pd.PagIBIGEE +
			pd.WithholdingTax + pd.LateDeduction + pd.UndertimeDeduction + pd.LeaveDeduction
		pd.NetPay = pd.GrossPay - pd.TotalDeductions

		pd.Breakdown["basic_pay"] = pd.BasicPay
		pd.Breakdown["overtime_pay"] = pd.OvertimePay
		pd.Breakdown["night_diff"] = pd.NightDiff
		pd.Breakdown["holiday_pay"] = pd.HolidayPay
		pd.Breakdown["late_deduction"] = pd.LateDeduction
		pd.Breakdown["undertime_deduction"] = pd.UndertimeDeduction
		pd.Breakdown["leave_deduction"] = pd.LeaveDeduction
		pd.Breakdown["sss_ee"] = pd.SSSEE
		pd.Breakdown["sss_er"] = pd.SSSER
		pd.Breakdown["philhealth_ee"] = pd.PhilHealthEE
		pd.Breakdown["philhealth_er"] = pd.PhilHealthER
		pd.Breakdown["pagibig_ee"] = pd.PagIBIGEE
		pd.Breakdown["pagibig_er"] = pd.PagIBIGER
		pd.Breakdown["withholding_tax"] = pd.WithholdingTax
		pd.Breakdown["days_worked"] = pd.DaysWorked
		pd.Breakdown["working_days_in_period"] = workingDaysInPeriod
		pd.Breakdown["unpaid_leave_days"] = pd.UnpaidLeaveDays
		pd.Breakdown["night_hours"] = pd.NightHours
		pd.Breakdown["regular_holiday_days"] = pd.RegularHolidayDays
		pd.Breakdown["special_holiday_days"] = pd.SpecialHolidayDays

		breakdownJSON, _ := json.Marshal(pd.Breakdown)

		// Insert payroll item
		_, err := calc.queries.CreatePayrollItem(ctx, store.CreatePayrollItemParams{
			RunID:              runID,
			EmployeeID:         emp.ID,
			BasicPay:           numericFromFloat(pd.BasicPay),
			GrossPay:           numericFromFloat(pd.GrossPay),
			TaxableIncome:      numericFromFloat(pd.TaxableIncome),
			TotalDeductions:    numericFromFloat(pd.TotalDeductions),
			NetPay:             numericFromFloat(pd.NetPay),
			SssEe:              numericFromFloat(pd.SSSEE),
			SssEr:              numericFromFloat(pd.SSSER),
			SssEc:              numericFromFloat(pd.SSSEC),
			PhilhealthEe:       numericFromFloat(pd.PhilHealthEE),
			PhilhealthEr:       numericFromFloat(pd.PhilHealthER),
			PagibigEe:          numericFromFloat(pd.PagIBIGEE),
			PagibigEr:          numericFromFloat(pd.PagIBIGER),
			WithholdingTax:     numericFromFloat(pd.WithholdingTax),
			Breakdown:          json.RawMessage(breakdownJSON),
			WorkDays:           numericFromFloat(pd.DaysWorked),
			HoursWorked:        numericFromFloat(pd.HoursWorked),
			OtHours:            numericFromFloat(pd.OvertimeHours),
			LateDeduction:      numericFromFloat(pd.LateDeduction),
			UndertimeDeduction: numericFromFloat(pd.UndertimeDeduction),
			HolidayPay:         numericFromFloat(pd.HolidayPay),
			NightDiff:          numericFromFloat(pd.NightDiff),
		})
		if err != nil {
			calc.logger.Error("failed to create payroll item", "employee_id", emp.ID, "error", err)
			continue
		}

		totalGross += pd.GrossPay
		totalDeductions += pd.TotalDeductions
		totalNet += pd.NetPay
		processedCount++

		// Create payslip
		payslipPayload, _ := json.Marshal(map[string]interface{}{
			"employee_id":   emp.ID,
			"employee_no":   emp.EmployeeNo,
			"employee_name": emp.FirstName + " " + emp.LastName,
			"period_start":  periodStart.Format("2006-01-02"),
			"period_end":    periodEnd.Format("2006-01-02"),
			"pay_date":      cycle.PayDate.Format("2006-01-02"),
			"basic_pay":     pd.BasicPay,
			"gross_pay":     pd.GrossPay,
			"net_pay":       pd.NetPay,
			"deductions":    pd.TotalDeductions,
			"breakdown":     pd.Breakdown,
		})

		_, err = calc.queries.CreatePayslip(ctx, store.CreatePayslipParams{
			CompanyID:   companyID,
			RunID:       runID,
			EmployeeID:  emp.ID,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			PayDate:     cycle.PayDate,
			Payload:     json.RawMessage(payslipPayload),
		})
		if err != nil {
			calc.logger.Error("failed to create payslip", "employee_id", emp.ID, "error", err)
		}
	}

	return calc.completeRun(ctx, runID, processedCount, totalGross, totalDeductions, totalNet)
}

// computePay calculates basic pay, overtime, night diff, holiday premium, and deductions.
func (calc *Calculator) computePay(pd *EmployeePayData, workingDaysInPeriod int) {
	dailyRate := pd.BasicSalary / float64(workingDaysInPeriod)
	hourlyRate := dailyRate / 8.0

	// Basic pay = daily rate * days worked
	pd.BasicPay = dailyRate * pd.DaysWorked

	// Overtime pay with type-based Philippine labor law rates:
	// Regular day OT: 125% of hourly rate
	// Rest day OT: 169% (130% × 130%)
	// Regular holiday OT: 260% (200% × 130%)
	// Special holiday OT: 169% (130% × 130%)
	hasTypedOT := pd.OTRegular > 0 || pd.OTRestDay > 0 || pd.OTHoliday > 0 || pd.OTSpecialHoliday > 0
	if hasTypedOT {
		pd.OvertimePay = hourlyRate*1.25*pd.OTRegular +
			hourlyRate*1.69*pd.OTRestDay +
			hourlyRate*2.60*pd.OTHoliday +
			hourlyRate*1.69*pd.OTSpecialHoliday
	} else {
		// Fallback: all OT at regular 1.25x rate
		pd.OvertimePay = hourlyRate * 1.25 * pd.OvertimeHours
	}

	// Night differential: 10% of hourly rate per night hour (Philippine Labor Code Art. 86)
	pd.NightDiff = hourlyRate * 0.10 * pd.NightHours

	// Holiday premium pay (additional pay for working on holidays):
	// Regular Holiday: +100% of daily rate (total 200%)
	// Special Non-Working Day: +30% of daily rate (total 130%)
	pd.HolidayPay = dailyRate*1.0*float64(pd.RegularHolidayDays) +
		dailyRate*0.30*float64(pd.SpecialHolidayDays)

	// Late deduction = (late minutes / 60) * hourly rate
	pd.LateDeduction = (float64(pd.LateMinutes) / 60.0) * hourlyRate

	// Undertime deduction
	pd.UndertimeDeduction = (float64(pd.UndertimeMinutes) / 60.0) * hourlyRate

	// Unpaid leave deduction
	pd.LeaveDeduction = dailyRate * pd.UnpaidLeaveDays

	// Gross pay
	pd.GrossPay = pd.BasicPay + pd.OvertimePay + pd.NightDiff + pd.HolidayPay -
		pd.LateDeduction - pd.UndertimeDeduction - pd.LeaveDeduction
	if pd.GrossPay < 0 {
		pd.GrossPay = 0
	}

	// Round to 2 decimal places
	pd.BasicPay = round2(pd.BasicPay)
	pd.OvertimePay = round2(pd.OvertimePay)
	pd.NightDiff = round2(pd.NightDiff)
	pd.HolidayPay = round2(pd.HolidayPay)
	pd.LateDeduction = round2(pd.LateDeduction)
	pd.UndertimeDeduction = round2(pd.UndertimeDeduction)
	pd.LeaveDeduction = round2(pd.LeaveDeduction)
	pd.GrossPay = round2(pd.GrossPay)
}

// computeContributions looks up SSS, PhilHealth, PagIBIG tables.
func (calc *Calculator) computeContributions(ctx context.Context, pd *EmployeePayData, effectiveDate time.Time) {
	monthlySalary := pd.BasicSalary

	// SSS
	sss, err := calc.queries.GetSSSContribution(ctx, store.GetSSSContributionParams{
		MscMin:        numericFromFloat(monthlySalary),
		EffectiveFrom: effectiveDate,
	})
	if err != nil {
		calc.logger.Warn("SSS table lookup failed, using defaults", "salary", monthlySalary, "error", err)
	} else {
		pd.SSSEE = numericToFloat(sss.EeShare)
		pd.SSSER = numericToFloat(sss.ErShare)
		pd.SSSEC = numericToFloat(sss.Ec)
	}

	// PhilHealth
	ph, err := calc.queries.GetPhilHealthContribution(ctx, store.GetPhilHealthContributionParams{
		SalaryMin:     numericFromFloat(monthlySalary),
		EffectiveFrom: effectiveDate,
	})
	if err != nil {
		calc.logger.Warn("PhilHealth table lookup failed", "salary", monthlySalary, "error", err)
		// Fallback: 5% premium rate, split 50/50
		premium := monthlySalary * 0.05
		pd.PhilHealthEE = round2(premium / 2)
		pd.PhilHealthER = round2(premium / 2)
	} else {
		premiumRate := numericToFloat(ph.PremiumRate)
		premium := monthlySalary * premiumRate
		floor := numericToFloat(ph.FloorPremium)
		ceiling := numericToFloat(ph.CeilingPremium)
		if floor > 0 && premium < floor {
			premium = floor
		}
		if ceiling > 0 && premium > ceiling {
			premium = ceiling
		}
		eeRate := numericToFloat(ph.EeShareRate)
		erRate := numericToFloat(ph.ErShareRate)
		pd.PhilHealthEE = round2(premium * eeRate / (eeRate + erRate))
		pd.PhilHealthER = round2(premium * erRate / (eeRate + erRate))
	}

	// PagIBIG
	pi, err := calc.queries.GetPagIBIGContribution(ctx, store.GetPagIBIGContributionParams{
		SalaryMin:     numericFromFloat(monthlySalary),
		EffectiveFrom: effectiveDate,
	})
	if err != nil {
		calc.logger.Warn("PagIBIG table lookup failed", "salary", monthlySalary, "error", err)
		// Fallback: standard 200 each
		pd.PagIBIGEE = 200
		pd.PagIBIGER = 200
	} else {
		eeRate := numericToFloat(pi.EeRate)
		erRate := numericToFloat(pi.ErRate)
		maxEE := numericToFloat(pi.MaxEe)
		maxER := numericToFloat(pi.MaxEr)
		pd.PagIBIGEE = round2(math.Min(monthlySalary*eeRate, maxEE))
		pd.PagIBIGER = round2(math.Min(monthlySalary*erRate, maxER))
	}
}

// computeWithholdingTax calculates BIR withholding tax using TRAIN Law brackets.
func (calc *Calculator) computeWithholdingTax(ctx context.Context, pd *EmployeePayData, effectiveDate time.Time) {
	// Taxable income = gross pay - employee contributions (SSS + PhilHealth + PagIBIG)
	pd.TaxableIncome = round2(pd.GrossPay - pd.SSSEE - pd.PhilHealthEE - pd.PagIBIGEE)
	if pd.TaxableIncome <= 0 {
		pd.WithholdingTax = 0
		return
	}

	bracket, err := calc.queries.GetBIRTaxBracket(ctx, store.GetBIRTaxBracketParams{
		Frequency:     "monthly",
		BracketMin:    numericFromFloat(pd.TaxableIncome),
		EffectiveFrom: effectiveDate,
	})
	if err != nil {
		calc.logger.Warn("BIR tax bracket lookup failed", "taxable_income", pd.TaxableIncome, "error", err)
		pd.WithholdingTax = 0
		return
	}

	fixedTax := numericToFloat(bracket.FixedTax)
	rate := numericToFloat(bracket.Rate)
	excessOver := numericToFloat(bracket.ExcessOver)

	pd.WithholdingTax = round2(fixedTax + (pd.TaxableIncome-excessOver)*rate)
	if pd.WithholdingTax < 0 {
		pd.WithholdingTax = 0
	}
}

// --- Run status helpers ---

func (calc *Calculator) failRun(ctx context.Context, runID int64, origErr error) error {
	msg := origErr.Error()
	_ = calc.queries.UpdatePayrollRun(ctx, store.UpdatePayrollRunParams{
		ID:           runID,
		Status:       "failed",
		ErrorMessage: &msg,
	})
	return origErr
}

func (calc *Calculator) completeRun(ctx context.Context, runID int64, count int32, gross, deductions, net float64) error {
	return calc.queries.UpdatePayrollRun(ctx, store.UpdatePayrollRunParams{
		ID:              runID,
		TotalEmployees:  count,
		TotalGross:      numericFromFloat(gross),
		TotalDeductions: numericFromFloat(deductions),
		TotalNet:        numericFromFloat(net),
		Status:          "completed",
	})
}
