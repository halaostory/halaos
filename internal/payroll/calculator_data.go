package payroll

import (
	"context"
	"time"

	"github.com/tonypk/aigonhr/internal/store"
)

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

type overtimeByType struct {
	Regular        float64
	RestDay        float64
	Holiday        float64
	SpecialHoliday float64
}

type holidayAttendance struct {
	RegularDays        int64
	SpecialNonWorkDays int64
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
	eveningStart := day.Add(22 * time.Hour) // today 22:00
	eveningEnd := day.Add(30 * time.Hour)   // tomorrow 06:00
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

// buildBonusMap returns employee_id → total approved bonus amount for a payroll cycle.
func (calc *Calculator) buildBonusMap(ctx context.Context, companyID int64, cycleID int64) (map[int64]float64, error) {
	rows, err := calc.queries.GetApprovedBonusesForPayroll(ctx, store.GetApprovedBonusesForPayrollParams{
		CompanyID:      companyID,
		PayrollCycleID: &cycleID,
	})
	if err != nil {
		return nil, err
	}

	m := make(map[int64]float64, len(rows))
	for _, r := range rows {
		m[r.EmployeeID] = numericToFloat(r.TotalBonus)
	}
	return m, nil
}
