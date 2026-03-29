package payroll

import (
	"context"
	"math"
	"time"

	"github.com/halaostory/halaos/internal/store"
)

// computePayUS calculates gross pay for US employees.
func (calc *Calculator) computePayUS(pd *EmployeePayData, workingDaysInPeriod int, state string) {
	dailyRate := pd.BasicSalary / float64(workingDaysInPeriod)
	hourlyRate := dailyRate / 8.0

	pd.BasicPay = dailyRate * pd.DaysWorked

	hasTypedOT := pd.OTRegular > 0 || pd.OTHoliday > 0
	if state == "CA" && hasTypedOT {
		pd.OvertimePay = hourlyRate*1.5*pd.OTRegular + hourlyRate*2.0*pd.OTHoliday
	} else {
		pd.OvertimePay = hourlyRate * 1.5 * pd.OvertimeHours
	}

	pd.NightDiff = 0
	pd.HolidayPay = dailyRate * 1.0 * float64(pd.RegularHolidayDays)
	pd.LateDeduction = (float64(pd.LateMinutes) / 60.0) * hourlyRate
	pd.UndertimeDeduction = (float64(pd.UndertimeMinutes) / 60.0) * hourlyRate
	pd.LeaveDeduction = dailyRate * pd.UnpaidLeaveDays

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

// computeContributionsUS calculates FICA (SS + Medicare), FUTA, state disability, etc.
func (calc *Calculator) computeContributionsUS(ctx context.Context, pd *EmployeePayData, effectiveDate time.Time, companyID int64, state string) {
	gross := pd.GrossPay

	ssEERate := 0.062
	ssERRate := 0.062
	medEERate := 0.0145
	medERRate := 0.0145
	addMedRate := 0.009
	ssWageBase := 176100.0
	addMedThreshold := 200000.0
	caSDIRate := 0.012

	rates, err := calc.queries.ListCountryContributionRates(ctx, store.ListCountryContributionRatesParams{
		Country:       "USA",
		EffectiveFrom: effectiveDate,
	})
	if err == nil {
		for _, r := range rates {
			rate := numericToFloat(r.Rate)
			switch r.ContributionType {
			case "fica_ss_employee":
				ssEERate = rate
			case "fica_ss_employer":
				ssERRate = rate
			case "fica_medicare_employee":
				medEERate = rate
			case "fica_medicare_employer":
				medERRate = rate
			case "fica_additional_medicare":
				addMedRate = rate
			case "ca_sdi":
				caSDIRate = rate
			}
		}
	}

	if cfg, err := calc.queries.GetCountryPayrollConfig(ctx, store.GetCountryPayrollConfigParams{
		Country:   "USA",
		ConfigKey: "fica_ss_wage_base",
	}); err == nil {
		ssWageBase = interfaceToFloat(cfg.ConfigValue)
	}
	if cfg, err := calc.queries.GetCountryPayrollConfig(ctx, store.GetCountryPayrollConfigParams{
		Country:   "USA",
		ConfigKey: "fica_additional_medicare_threshold",
	}); err == nil {
		addMedThreshold = interfaceToFloat(cfg.ConfigValue)
	}

	// Get YTD totals for wage base cap
	ytdGross := 0.0
	if ytd, err := calc.queries.GetYTDPayrollTotals(ctx, store.GetYTDPayrollTotalsParams{
		EmployeeID: pd.EmployeeID,
		CompanyID:  companyID,
		Year:       int32(effectiveDate.Year()),
	}); err == nil {
		ytdGross = interfaceToFloat(ytd.YtdGross)
	}

	ficaWages := gross - pd.Section125Deductions
	if ficaWages < 0 {
		ficaWages = 0
	}

	ssWages := ficaWages
	if ytdGross >= ssWageBase {
		ssWages = 0
	} else if ytdGross+ficaWages > ssWageBase {
		ssWages = ssWageBase - ytdGross
	}
	pd.SocialSecurityEE = round2(ssWages * ssEERate)
	pd.SocialSecurityER = round2(ssWages * ssERRate)

	pd.MedicareEE = round2(ficaWages * medEERate)
	pd.MedicareER = round2(ficaWages * medERRate)

	if ytdGross+ficaWages > addMedThreshold {
		if ytdGross >= addMedThreshold {
			pd.AdditionalMedicare = round2(ficaWages * addMedRate)
		} else {
			pd.AdditionalMedicare = round2((ytdGross + ficaWages - addMedThreshold) * addMedRate)
		}
	}

	// FUTA
	futaWageBase := 7000.0
	if cfg, err := calc.queries.GetCountryPayrollConfig(ctx, store.GetCountryPayrollConfigParams{
		Country:   "USA",
		ConfigKey: "futa_wage_base",
	}); err == nil {
		futaWageBase = interfaceToFloat(cfg.ConfigValue)
	}
	futaRate := 0.006
	futaKey := "futa_rate_default"
	if state == "CA" {
		futaKey = "futa_rate_ca"
	}
	if cfg, err := calc.queries.GetCountryPayrollConfig(ctx, store.GetCountryPayrollConfigParams{
		Country:   "USA",
		ConfigKey: futaKey,
	}); err == nil {
		futaRate = interfaceToFloat(cfg.ConfigValue)
	}
	futaWages := gross
	if ytdGross >= futaWageBase {
		futaWages = 0
	} else if ytdGross+gross > futaWageBase {
		futaWages = futaWageBase - ytdGross
	}
	pd.FUTA = round2(futaWages * futaRate)

	switch state {
	case "CA":
		pd.StateDisability = round2(gross * caSDIRate)
	case "WA":
		waEERate := 0.0066
		waERRate := 0.0026
		if r, err := calc.queries.GetCountryContributionRate(ctx, store.GetCountryContributionRateParams{
			Country:          "USA",
			ContributionType: "wa_pfml_employee",
			EffectiveFrom:    effectiveDate,
		}); err == nil {
			waEERate = numericToFloat(r.Rate)
		}
		if r, err := calc.queries.GetCountryContributionRate(ctx, store.GetCountryContributionRateParams{
			Country:          "USA",
			ContributionType: "wa_pfml_employer",
			EffectiveFrom:    effectiveDate,
		}); err == nil {
			waERRate = numericToFloat(r.Rate)
		}
		pd.StateDisability = round2(gross * waEERate)
		pd.SUI = round2(gross * waERRate)
	}
}

// computeWithholdingTaxUS calculates federal + state income tax withholding.
func (calc *Calculator) computeWithholdingTaxUS(ctx context.Context, pd *EmployeePayData, effectiveDate time.Time, payPeriodsPerYear int, filingStatus, state string) {
	federalTaxableGross := pd.GrossPay - pd.PreTaxDeductions
	if federalTaxableGross < 0 {
		federalTaxableGross = 0
	}

	annualIncome := federalTaxableGross * float64(payPeriodsPerYear)
	annualIncome += pd.W4OtherIncome
	annualIncome -= pd.W4Deductions
	if annualIncome < 0 {
		annualIncome = 0
	}

	// Federal tax
	fs := filingStatus // need pointer for sqlc
	brackets, err := calc.queries.GetFederalTaxBrackets(ctx, store.GetFederalTaxBracketsParams{
		FilingStatus:  &fs,
		EffectiveFrom: effectiveDate,
	})
	if err != nil || len(brackets) == 0 {
		calc.logger.Warn("US federal brackets not found", "filing_status", filingStatus, "error", err)
		pd.FederalTax = 0
	} else {
		annualTax := computeProgressiveTaxFromBrackets(annualIncome, brackets)
		periodTax := annualTax / float64(payPeriodsPerYear)
		periodTax += pd.W4AdditionalWithholding
		periodCredit := pd.W4DependentsCredit / float64(payPeriodsPerYear)
		periodTax -= periodCredit
		if periodTax < 0 {
			periodTax = 0
		}
		pd.FederalTax = round2(periodTax)
	}

	// State income tax
	switch state {
	case "CA":
		st := "CA"
		stateBrackets, err := calc.queries.GetStateTaxBrackets(ctx, store.GetStateTaxBracketsParams{
			State:         &st,
			FilingStatus:  &fs,
			EffectiveFrom: effectiveDate,
		})
		if err == nil && len(stateBrackets) > 0 {
			annualStateTax := computeProgressiveTaxFromBrackets(annualIncome, stateBrackets)
			if annualIncome > 1000000 {
				annualStateTax += (annualIncome - 1000000) * 0.01
			}
			pd.StateTax = round2(annualStateTax / float64(payPeriodsPerYear))
		}
	case "NY":
		st := "NY"
		stateBrackets, err := calc.queries.GetStateTaxBrackets(ctx, store.GetStateTaxBracketsParams{
			State:         &st,
			FilingStatus:  &fs,
			EffectiveFrom: effectiveDate,
		})
		if err == nil && len(stateBrackets) > 0 {
			annualStateTax := computeProgressiveTaxFromBrackets(annualIncome, stateBrackets)
			pd.StateTax = round2(annualStateTax / float64(payPeriodsPerYear))
		}
	}

	pd.WithholdingTax = pd.FederalTax + pd.StateTax
	pd.TaxableIncome = round2(federalTaxableGross)
}

// computeProgressiveTaxFromBrackets calculates progressive tax from DB bracket rows.
func computeProgressiveTaxFromBrackets(annualIncome float64, brackets []store.CountryTaxBracket) float64 {
	tax := 0.0
	for _, bracket := range brackets {
		min := numericToFloat(bracket.BracketMin)
		max := numericToFloat(bracket.BracketMax)
		rate := numericToFloat(bracket.TaxRate)

		if annualIncome <= min {
			break
		}
		taxable := annualIncome - min
		bracketWidth := max - min
		if bracketWidth > 0 && taxable > bracketWidth {
			taxable = bracketWidth
		}
		tax += taxable * rate
	}
	return math.Round(tax*100) / 100
}
