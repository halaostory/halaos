package payroll

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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
	DaysWorked        float64
	HoursWorked       float64
	OvertimeHours     float64
	LateMinutes       int64
	UndertimeMinutes  int64
	UnpaidLeaveDays   float64
	PaidLeaveDays     float64

	// Night differential & holiday
	NightHours            float64
	RegularHolidayDays    int64
	SpecialHolidayDays    int64
	OTRegular             float64
	OTRestDay             float64
	OTHoliday             float64
	OTSpecialHoliday      float64

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
	SSSEE         float64
	SSSER         float64
	SSSEC         float64
	PhilHealthEE  float64
	PhilHealthER  float64
	PagIBIGEE     float64
	PagIBIGER     float64
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

	// Build salary map: employeeID -> monthly basic salary
	salaryMap, err := calc.buildSalaryMap(ctx, companyID, effectiveDate)
	if err != nil {
		return calc.failRun(ctx, runID, fmt.Errorf("build salary map: %w", err))
	}

	// Build attendance map
	attendanceMap, err := calc.buildAttendanceMap(ctx, companyID, periodStart, periodEnd)
	if err != nil {
		return calc.failRun(ctx, runID, fmt.Errorf("build attendance map: %w", err))
	}

	// Build overtime map (totals for backward compat)
	otMap, err := calc.buildOvertimeMap(ctx, companyID, periodStart, periodEnd)
	if err != nil {
		return calc.failRun(ctx, runID, fmt.Errorf("build overtime map: %w", err))
	}

	// Build overtime-by-type map
	otTypeMap, err := calc.buildOTByTypeMap(ctx, companyID, periodStart, periodEnd)
	if err != nil {
		return calc.failRun(ctx, runID, fmt.Errorf("build ot type map: %w", err))
	}

	// Build leave map
	leaveMap, err := calc.buildLeaveMap(ctx, companyID, periodStart, periodEnd)
	if err != nil {
		return calc.failRun(ctx, runID, fmt.Errorf("build leave map: %w", err))
	}

	// Build night hours map (10 PM - 6 AM)
	nightMap, err := calc.buildNightHoursMap(ctx, companyID, periodStart, periodEnd)
	if err != nil {
		return calc.failRun(ctx, runID, fmt.Errorf("build night hours map: %w", err))
	}

	// Build holiday attendance map
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

// --- Data aggregation helpers ---

type attendanceSummary struct {
	DaysWorked       float64
	HoursWorked      float64
	LateMinutes      int64
	UndertimeMinutes int64
}

type leaveSummary struct {
	PaidDays   float64
	UnpaidDays float64
}

func (calc *Calculator) buildSalaryMap(ctx context.Context, companyID int64, effectiveDate time.Time) (map[int64]float64, error) {
	salaries, err := calc.queries.ListCurrentSalaries(ctx, store.ListCurrentSalariesParams{
		CompanyID:     companyID,
		EffectiveFrom: effectiveDate,
	})
	if err != nil {
		return nil, err
	}

	m := make(map[int64]float64, len(salaries))
	for _, s := range salaries {
		if _, exists := m[s.EmployeeID]; !exists {
			m[s.EmployeeID] = numericToFloat(s.BasicSalary)
		}
	}
	return m, nil
}

func (calc *Calculator) buildAttendanceMap(ctx context.Context, companyID int64, periodStart, periodEnd time.Time) (map[int64]attendanceSummary, error) {
	rows, err := calc.queries.GetAttendanceSummaryForPeriod(ctx, store.GetAttendanceSummaryForPeriodParams{
		CompanyID:   companyID,
		ClockInAt:   pgTimestamptz(periodStart),
		ClockInAt_2: pgTimestamptz(periodEnd.AddDate(0, 0, 1)),
	})
	if err != nil {
		return nil, err
	}

	m := make(map[int64]attendanceSummary, len(rows))
	for _, r := range rows {
		m[r.EmployeeID] = attendanceSummary{
			DaysWorked:       float64(r.DaysWorked),
			HoursWorked:      interfaceToFloat(r.TotalWorkHours),
			LateMinutes:      interfaceToInt64(r.TotalLateMinutes),
			UndertimeMinutes: interfaceToInt64(r.TotalUndertimeMinutes),
		}
	}
	return m, nil
}

func (calc *Calculator) buildOvertimeMap(ctx context.Context, companyID int64, periodStart, periodEnd time.Time) (map[int64]float64, error) {
	rows, err := calc.queries.GetApprovedOTHours(ctx, store.GetApprovedOTHoursParams{
		CompanyID: companyID,
		OtDate:    periodStart,
		OtDate_2:  periodEnd,
	})
	if err != nil {
		return nil, err
	}

	m := make(map[int64]float64, len(rows))
	for _, r := range rows {
		m[r.EmployeeID] = float64(r.TotalHours)
	}
	return m, nil
}

func (calc *Calculator) buildLeaveMap(ctx context.Context, companyID int64, periodStart, periodEnd time.Time) (map[int64]leaveSummary, error) {
	rows, err := calc.queries.GetApprovedLeaveDaysForPeriod(ctx, store.GetApprovedLeaveDaysForPeriodParams{
		CompanyID: companyID,
		EndDate:   periodEnd,
		StartDate: periodStart,
	})
	if err != nil {
		return nil, err
	}

	m := make(map[int64]leaveSummary, len(rows))
	for _, r := range rows {
		m[r.EmployeeID] = leaveSummary{
			PaidDays:   interfaceToFloat(r.PaidLeaveDays),
			UnpaidDays: interfaceToFloat(r.UnpaidLeaveDays),
		}
	}
	return m, nil
}

// --- Night differential & holiday maps ---

type overtimeByType struct {
	Regular        float64
	RestDay        float64
	Holiday        float64
	SpecialHoliday float64
}

type holidayAttendance struct {
	RegularDays       int64
	SpecialNonWorkDays int64
}

func (calc *Calculator) buildOTByTypeMap(ctx context.Context, companyID int64, periodStart, periodEnd time.Time) (map[int64]overtimeByType, error) {
	rows, err := calc.queries.GetApprovedOTHoursByType(ctx, store.GetApprovedOTHoursByTypeParams{
		CompanyID: companyID,
		OtDate:    periodStart,
		OtDate_2:  periodEnd,
	})
	if err != nil {
		return nil, err
	}

	m := make(map[int64]overtimeByType)
	for _, r := range rows {
		ot := m[r.EmployeeID]
		hours := float64(r.TotalHours)
		switch r.OtType {
		case "regular":
			ot.Regular = hours
		case "rest_day":
			ot.RestDay = hours
		case "holiday":
			ot.Holiday = hours
		case "special_holiday":
			ot.SpecialHoliday = hours
		default:
			ot.Regular += hours
		}
		m[r.EmployeeID] = ot
	}
	return m, nil
}

func (calc *Calculator) buildNightHoursMap(ctx context.Context, companyID int64, periodStart, periodEnd time.Time) (map[int64]float64, error) {
	rows, err := calc.queries.GetAttendanceRecordsForPeriod(ctx, store.GetAttendanceRecordsForPeriodParams{
		CompanyID:   companyID,
		ClockInAt:   pgTimestamptz(periodStart),
		ClockInAt_2: pgTimestamptz(periodEnd.AddDate(0, 0, 1)),
	})
	if err != nil {
		return nil, err
	}

	manila, _ := time.LoadLocation("Asia/Manila")
	m := make(map[int64]float64)
	for _, r := range rows {
		if !r.ClockInAt.Valid || !r.ClockOutAt.Valid {
			continue
		}
		hours := calculateNightHours(r.ClockInAt.Time, r.ClockOutAt.Time, manila)
		if hours > 0 {
			m[r.EmployeeID] += hours
		}
	}
	return m, nil
}

// calculateNightHours returns the hours a shift overlaps with 10 PM - 6 AM night window.
func calculateNightHours(clockIn, clockOut time.Time, loc *time.Location) float64 {
	inLocal := clockIn.In(loc)
	outLocal := clockOut.In(loc)

	var total float64

	// Check two possible night windows the shift could overlap:
	// Window 1: evening of clock-in day (22:00 same day to 06:00 next day)
	// Window 2: morning of clock-in day (previous day 22:00 to 06:00 same day)
	day := time.Date(inLocal.Year(), inLocal.Month(), inLocal.Day(), 0, 0, 0, 0, loc)

	// Morning window: yesterday 22:00 to today 06:00
	morningStart := day.Add(-2 * time.Hour) // yesterday 22:00
	morningEnd := day.Add(6 * time.Hour)    // today 06:00
	total += overlapHours(inLocal, outLocal, morningStart, morningEnd)

	// Evening window: today 22:00 to tomorrow 06:00
	eveningStart := day.Add(22 * time.Hour)   // today 22:00
	eveningEnd := day.Add(30 * time.Hour)     // tomorrow 06:00
	total += overlapHours(inLocal, outLocal, eveningStart, eveningEnd)

	return round2(total)
}

// overlapHours returns the overlap in hours between two time ranges.
func overlapHours(s1, e1, s2, e2 time.Time) float64 {
	start := s1
	if s2.After(start) {
		start = s2
	}
	end := e1
	if e2.Before(end) {
		end = e2
	}
	if end.After(start) {
		return end.Sub(start).Hours()
	}
	return 0
}

func (calc *Calculator) buildHolidayAttendanceMap(ctx context.Context, companyID int64, periodStart, periodEnd time.Time) (map[int64]holidayAttendance, error) {
	rows, err := calc.queries.GetHolidayAttendanceForPeriod(ctx, store.GetHolidayAttendanceForPeriodParams{
		CompanyID:   companyID,
		ClockInAt:   pgTimestamptz(periodStart),
		ClockInAt_2: pgTimestamptz(periodEnd.AddDate(0, 0, 1)),
	})
	if err != nil {
		return nil, err
	}

	m := make(map[int64]holidayAttendance)
	for _, r := range rows {
		ha := m[r.EmployeeID]
		switch r.HolidayType {
		case "regular":
			ha.RegularDays = r.DaysWorked
		case "special_non_working":
			ha.SpecialNonWorkDays = r.DaysWorked
		}
		m[r.EmployeeID] = ha
	}
	return m, nil
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

// --- Utility functions ---

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

func countWorkingDays(start, end time.Time) int {
	days := 0
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			days++
		}
	}
	if days == 0 {
		days = 1
	}
	return days
}

func numericFromFloat(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(f, 'f', 2, 64))
	return n
}

func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, _ := n.Float64Value()
	if !f.Valid {
		return 0
	}
	return f.Float64
}

func interfaceToFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case []byte:
		f, _ := strconv.ParseFloat(string(val), 64)
		return f
	default:
		return 0
	}
}

func interfaceToInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int64:
		return val
	case int32:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		i, _ := strconv.ParseInt(val, 10, 64)
		return i
	case []byte:
		i, _ := strconv.ParseInt(string(val), 10, 64)
		return i
	default:
		return 0
	}
}

func pgTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}
