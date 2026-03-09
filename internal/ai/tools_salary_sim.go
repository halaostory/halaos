package ai

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/store"
)

func (r *ToolRegistry) toolSimulateSalary(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	// Determine target employee
	var employeeID int64
	if eid, ok := input["employee_id"].(float64); ok && eid > 0 {
		if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
			return "", err
		}
		employeeID = int64(eid)
	} else {
		emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
			UserID:    &userID,
			CompanyID: companyID,
		})
		if err != nil {
			return "", fmt.Errorf("employee not found: %w", err)
		}
		employeeID = emp.ID
	}

	// Get current salary
	now := time.Now()
	salary, err := r.queries.GetCurrentSalary(ctx, store.GetCurrentSalaryParams{
		CompanyID:     companyID,
		EmployeeID:    employeeID,
		EffectiveFrom: now,
	})
	if err != nil {
		return "", fmt.Errorf("no salary record found: %w", err)
	}

	basicMonthly := numericToString(salary.BasicSalary)
	basicFloat := parseFloat(basicMonthly)

	// Parse simulation parameters
	workingDays := getFloatOr(input, "working_days", 22)
	overtimeHours := getFloatOr(input, "overtime_hours", 0)
	restDayOTHours := getFloatOr(input, "rest_day_ot_hours", 0)
	holidayOTHours := getFloatOr(input, "holiday_ot_hours", 0)
	nightHours := getFloatOr(input, "night_hours", 0)
	regularHolidayDays := getFloatOr(input, "regular_holiday_days", 0)
	specialHolidayDays := getFloatOr(input, "special_holiday_days", 0)
	lateMinutes := getFloatOr(input, "late_minutes", 0)
	unpaidLeaveDays := getFloatOr(input, "unpaid_leave_days", 0)

	// Calculate
	dailyRate := basicFloat / workingDays
	hourlyRate := dailyRate / 8.0

	basicPay := dailyRate * workingDays
	overtimePay := hourlyRate*1.25*overtimeHours +
		hourlyRate*1.69*restDayOTHours +
		hourlyRate*2.60*holidayOTHours
	nightDiff := hourlyRate * 0.10 * nightHours
	holidayPay := dailyRate*1.0*regularHolidayDays + dailyRate*0.30*specialHolidayDays
	lateDeduction := (lateMinutes / 60.0) * hourlyRate
	leaveDeduction := dailyRate * unpaidLeaveDays

	grossPay := basicPay + overtimePay + nightDiff + holidayPay - lateDeduction - leaveDeduction
	if grossPay < 0 {
		grossPay = 0
	}

	// Government contributions (estimate from monthly basic)
	sssEE := estimateSSS(basicFloat)
	philhealthEE := round2(basicFloat * 0.05 / 2) // 5% total, 50/50 split
	pagibigEE := round2(math.Min(basicFloat*0.02, 200))
	taxableIncome := round2(grossPay - sssEE - philhealthEE - pagibigEE)
	withholdingTax := estimateTax(taxableIncome)

	totalDeductions := sssEE + philhealthEE + pagibigEE + withholdingTax + lateDeduction + leaveDeduction
	netPay := grossPay - sssEE - philhealthEE - pagibigEE - withholdingTax

	result := map[string]any{
		"employee_id":     employeeID,
		"basic_monthly":   round2(basicFloat),
		"working_days":    workingDays,
		"daily_rate":      round2(dailyRate),
		"hourly_rate":     round2(hourlyRate),
		"basic_pay":       round2(basicPay),
		"overtime_pay":    round2(overtimePay),
		"night_diff":      round2(nightDiff),
		"holiday_pay":     round2(holidayPay),
		"gross_pay":       round2(grossPay),
		"sss_ee":          sssEE,
		"philhealth_ee":   philhealthEE,
		"pagibig_ee":      pagibigEE,
		"withholding_tax": round2(withholdingTax),
		"late_deduction":  round2(lateDeduction),
		"leave_deduction": round2(leaveDeduction),
		"total_deductions": round2(totalDeductions),
		"estimated_net":   round2(netPay),
		"note":            "This is an estimate. Actual pay may vary based on exact SSS/PhilHealth/Pag-IBIG bracket lookups and BIR tax tables.",
	}

	return toJSON(result)
}

func getFloatOr(input map[string]any, key string, def float64) float64 {
	if v, ok := input[key].(float64); ok {
		return v
	}
	return def
}

func parseFloat(s string) float64 {
	var f float64
	_, _ = fmt.Sscanf(s, "%f", &f)
	return f
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

// estimateSSS provides a rough SSS EE contribution estimate.
// Uses 2024 WISP schedule approximation.
func estimateSSS(monthlySalary float64) float64 {
	if monthlySalary <= 4250 {
		return 180
	}
	if monthlySalary >= 29750 {
		return 1350
	}
	// Approximate: ~4.5% of salary
	return round2(monthlySalary * 0.045)
}

// estimateTax provides a rough BIR withholding tax estimate using TRAIN Law brackets.
func estimateTax(taxableIncome float64) float64 {
	if taxableIncome <= 20833 {
		return 0
	}
	if taxableIncome <= 33333 {
		return round2((taxableIncome - 20833) * 0.15)
	}
	if taxableIncome <= 66667 {
		return round2(1875 + (taxableIncome-33333)*0.20)
	}
	if taxableIncome <= 166667 {
		return round2(8541.80 + (taxableIncome-66667)*0.25)
	}
	if taxableIncome <= 666667 {
		return round2(33541.80 + (taxableIncome-166667)*0.30)
	}
	return round2(183541.80 + (taxableIncome-666667)*0.35)
}

// salarySimDefs returns tool definitions for salary simulation tools.
func salarySimDefs() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "simulate_salary",
			Description: "Simulate salary calculation with what-if scenarios. Input overtime hours, holiday work, night hours, late minutes, etc. to see estimated gross pay, deductions, and net pay. Useful for questions like 'How much would I earn with 10 hours overtime?'",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_id":          map[string]any{"type": "integer", "description": "Optional employee ID. Omit to simulate for current user."},
					"working_days":         map[string]any{"type": "number", "description": "Working days in period. Default 22."},
					"overtime_hours":       map[string]any{"type": "number", "description": "Regular overtime hours."},
					"rest_day_ot_hours":    map[string]any{"type": "number", "description": "Rest day overtime hours (169% rate)."},
					"holiday_ot_hours":     map[string]any{"type": "number", "description": "Holiday overtime hours (260% rate)."},
					"night_hours":          map[string]any{"type": "number", "description": "Night differential hours (10PM-6AM)."},
					"regular_holiday_days": map[string]any{"type": "number", "description": "Days worked on regular holidays."},
					"special_holiday_days": map[string]any{"type": "number", "description": "Days worked on special non-working holidays."},
					"late_minutes":         map[string]any{"type": "number", "description": "Total late minutes."},
					"unpaid_leave_days":    map[string]any{"type": "number", "description": "Unpaid leave days."},
				},
			}),
		},
	}
}

// registerSalarySimTools registers salary simulation tool executors.
func (r *ToolRegistry) registerSalarySimTools() {
	r.tools["simulate_salary"] = r.toolSimulateSalary
}
