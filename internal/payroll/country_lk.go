package payroll

import (
	"context"
	"time"

	"github.com/tonypk/aigonhr/internal/store"
)

// computePayLK calculates pay using Sri Lanka rates.
// OT: 1.5x regular, 2.0x holiday. Night diff: not mandated.
func (calc *Calculator) computePayLK(pd *EmployeePayData, workingDaysInPeriod int) {
	dailyRate := pd.BasicSalary / float64(workingDaysInPeriod)
	hourlyRate := dailyRate / 8.0

	pd.BasicPay = dailyRate * pd.DaysWorked

	// LK overtime rates: 1.5x regular, 2.0x holiday
	hasTypedOT := pd.OTRegular > 0 || pd.OTHoliday > 0
	if hasTypedOT {
		pd.OvertimePay = hourlyRate*1.5*pd.OTRegular +
			hourlyRate*2.0*pd.OTHoliday
	} else {
		pd.OvertimePay = hourlyRate * 1.5 * pd.OvertimeHours
	}

	// Night differential: not statutorily mandated in Sri Lanka (set to 0)
	pd.NightDiff = 0

	// Holiday premium: regular holiday = 100% extra (total 2x)
	pd.HolidayPay = dailyRate * 1.0 * float64(pd.RegularHolidayDays)

	// Deductions
	pd.LateDeduction = (float64(pd.LateMinutes) / 60.0) * hourlyRate
	pd.UndertimeDeduction = (float64(pd.UndertimeMinutes) / 60.0) * hourlyRate
	pd.LeaveDeduction = dailyRate * pd.UnpaidLeaveDays

	// Gross pay
	pd.GrossPay = pd.BasicPay + pd.OvertimePay + pd.HolidayPay -
		pd.LateDeduction - pd.UndertimeDeduction - pd.LeaveDeduction
	if pd.GrossPay < 0 {
		pd.GrossPay = 0
	}

	pd.BasicPay = round2(pd.BasicPay)
	pd.OvertimePay = round2(pd.OvertimePay)
	pd.HolidayPay = round2(pd.HolidayPay)
	pd.LateDeduction = round2(pd.LateDeduction)
	pd.UndertimeDeduction = round2(pd.UndertimeDeduction)
	pd.LeaveDeduction = round2(pd.LeaveDeduction)
	pd.GrossPay = round2(pd.GrossPay)
}

// computeContributionsLK calculates EPF (8% EE, 12% ER) and ETF (3% ER).
func (calc *Calculator) computeContributionsLK(ctx context.Context, pd *EmployeePayData, effectiveDate time.Time) {
	gross := pd.GrossPay

	// Look up contribution rates from DB, with fallback to statutory defaults
	epfEERate := 0.08
	epfERRate := 0.12
	etfERRate := 0.03

	rates, err := calc.queries.ListCountryContributionRates(ctx, store.ListCountryContributionRatesParams{
		Country:       "LKA",
		EffectiveFrom: effectiveDate,
	})
	if err == nil {
		for _, r := range rates {
			rate := numericToFloat(r.Rate)
			switch r.ContributionType {
			case "epf_employee":
				epfEERate = rate
			case "epf_employer":
				epfERRate = rate
			case "etf_employer":
				etfERRate = rate
			}
		}
	} else {
		calc.logger.Warn("LK contribution rates lookup failed, using defaults", "error", err)
	}

	pd.EPFEE = round2(gross * epfEERate)
	pd.EPFER = round2(gross * epfERRate)
	pd.ETFER = round2(gross * etfERRate)
}

// computeWithholdingTaxLK calculates APIT using formula: T = P × rate − fixed_amount.
func (calc *Calculator) computeWithholdingTaxLK(ctx context.Context, pd *EmployeePayData, effectiveDate time.Time) {
	// Taxable income = gross - EPF employee share
	pd.TaxableIncome = round2(pd.GrossPay - pd.EPFEE)
	if pd.TaxableIncome <= 0 {
		pd.WithholdingTax = 0
		return
	}

	bracket, err := calc.queries.GetCountryTaxBracket(ctx, store.GetCountryTaxBracketParams{
		Country:       "LKA",
		Frequency:     "monthly",
		BracketMin:    numericFromFloat(pd.TaxableIncome),
		EffectiveFrom: effectiveDate,
	})
	if err != nil {
		calc.logger.Warn("LK APIT bracket lookup failed", "taxable_income", pd.TaxableIncome, "error", err)
		pd.WithholdingTax = 0
		return
	}

	rate := numericToFloat(bracket.TaxRate)
	fixedAmount := numericToFloat(bracket.FixedAmount)

	// APIT formula: T = P × rate − fixed_amount
	pd.WithholdingTax = round2(pd.TaxableIncome*rate - fixedAmount)
	if pd.WithholdingTax < 0 {
		pd.WithholdingTax = 0
	}
}
