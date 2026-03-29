package payroll

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/halaostory/halaos/internal/store"
)

const taxExemptCap = 90000.0 // First 90K of 13th month is tax-exempt per TRAIN Law

// Calculate13thMonthPay computes 13th month pay for all active employees in a year.
// Formula: Total Basic Salary Earned During the Year / 12
// Pro-rated for employees who worked less than 12 months.
func (calc *Calculator) Calculate13thMonthPay(ctx context.Context, companyID int64, year int32) ([]store.ThirteenthMonthPay, error) {
	calc.logger.Info("computing 13th month pay", "company_id", companyID, "year", year)

	// 13th month pay is mandatory only in the Philippines
	company, err := calc.queries.GetCompanyByID(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("get company: %w", err)
	}
	if company.Country != "PHL" {
		calc.logger.Info("13th month pay not applicable for country, skipping", "country", company.Country)
		return nil, nil
	}

	employees, err := calc.queries.ListActiveEmployees(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("list employees: %w", err)
	}

	yearStart := time.Date(int(year), 1, 1, 0, 0, 0, 0, time.UTC)
	yearEnd := time.Date(int(year), 12, 31, 23, 59, 59, 0, time.UTC)

	// Get salary map at year end
	salaryMap, err := calc.buildSalaryMap(ctx, companyID, yearEnd)
	if err != nil {
		return nil, fmt.Errorf("build salary map: %w", err)
	}

	var results []store.ThirteenthMonthPay

	for _, emp := range employees {
		salary, ok := salaryMap[emp.ID]
		if !ok {
			calc.logger.Warn("no salary for employee, skipping 13th month", "employee_id", emp.ID)
			continue
		}

		// Calculate months worked (pro-rate for mid-year hires)
		hireDate := emp.HireDate
		startDate := yearStart
		if hireDate.After(yearStart) {
			startDate = hireDate
		}

		monthsWorked := monthsBetween(startDate, yearEnd)
		if monthsWorked > 12 {
			monthsWorked = 12
		}
		if monthsWorked < 0 {
			continue
		}

		// Total basic salary earned = monthly salary * months worked
		totalBasic := round2(salary * monthsWorked)

		// 13th month amount = total basic / 12
		amount := round2(totalBasic / 12.0)

		// Tax calculation
		taxableAmount := 0.0
		if amount > taxExemptCap {
			taxableAmount = round2(amount - taxExemptCap)
		}

		record, err := calc.queries.Upsert13thMonthPay(ctx, store.Upsert13thMonthPayParams{
			CompanyID:        companyID,
			EmployeeID:       emp.ID,
			Year:             year,
			TotalBasicSalary: numericFromFloat(totalBasic),
			MonthsWorked:     numericFromFloat(monthsWorked),
			Amount:           numericFromFloat(amount),
			TaxExemptAmount:  numericFromFloat(math.Min(amount, taxExemptCap)),
			TaxableAmount:    numericFromFloat(taxableAmount),
		})
		if err != nil {
			calc.logger.Error("failed to upsert 13th month", "employee_id", emp.ID, "error", err)
			continue
		}

		results = append(results, record)
	}

	calc.logger.Info("13th month pay computed", "company_id", companyID, "year", year, "count", len(results))
	return results, nil
}

// monthsBetween calculates fractional months between two dates.
func monthsBetween(start, end time.Time) float64 {
	if end.Before(start) {
		return 0
	}

	years := end.Year() - start.Year()
	months := int(end.Month()) - int(start.Month())
	totalMonths := years*12 + months

	// Add fractional part for partial month
	daysInMonth := daysInMonthOf(end)
	dayFraction := float64(end.Day()) / float64(daysInMonth)

	startDayFraction := float64(start.Day()-1) / float64(daysInMonthOf(start))

	result := float64(totalMonths) + dayFraction - startDayFraction
	if result < 0 {
		return 0
	}
	return math.Round(result*100) / 100
}

func daysInMonthOf(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
}
