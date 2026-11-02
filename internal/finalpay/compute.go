package finalpay

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/store"
)

// Inputs are the facts a final pay computation needs.
type Inputs struct {
	Country              string
	BasicMonthly         float64
	HireDate             time.Time
	SeparationDate       time.Time // last working day
	SeparationType       string    // resignation, termination, layoff, retirement, end_of_contract, death, other
	LastPaidThrough      time.Time // last day covered by a completed payroll (zero = never paid)
	YTDBasicPaid         float64   // basic pay already paid this calendar year
	ConvertibleLeaveDays float64
	OutstandingLoans     float64
	AgeAtSeparation      int // 0 if unknown
}

// Result is the computed breakdown (amounts rounded to centavos/cents).
type Result struct {
	DailyRate       float64           `json:"daily_rate"`
	UnpaidDays      int               `json:"unpaid_days"`
	UnpaidSalary    float64           `json:"unpaid_salary"`
	Prorated13th    float64           `json:"prorated_13th"`
	LeaveConversion float64           `json:"unused_leave_conversion"`
	SeparationPay   float64           `json:"separation_pay"`
	OtherDeductions float64           `json:"other_deductions"`
	Total           float64           `json:"total_final_pay"`
	YearsOfService  float64           `json:"years_of_service"`
	Notes           []string          `json:"notes"`
	Basis           map[string]string `json:"basis"`
}

func r2(f float64) float64 { return math.Round(f*100) / 100 }

// workdays counts Mon–Fri days in [from, to].
func workdays(from, to time.Time) int {
	n := 0
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			n++
		}
	}
	return n
}

// Compute applies the rules. Philippine rules (Labor Code, PD 851, RA 7641)
// are applied for PHL; elsewhere only unpaid salary, leave payout and loan
// deductions are computed and the rest is left for HR review.
func Compute(in Inputs) Result {
	res := Result{Basis: map[string]string{}, Notes: []string{}}
	daysPerYear := 261.0 // 5-day work week
	res.DailyRate = in.BasicMonthly * 12 / daysPerYear
	res.Basis["daily_rate"] = "basic monthly × 12 ÷ 261 working days"

	hasPayroll := !in.LastPaidThrough.IsZero() && in.LastPaidThrough.Year() >= 1900
	start := in.LastPaidThrough.AddDate(0, 0, 1)
	if !hasPayroll {
		// No payroll history in HalaOS (e.g. migrated mid-employment): assume
		// salary was paid through the end of the previous month.
		start = time.Date(in.SeparationDate.Year(), in.SeparationDate.Month(), 1, 0, 0, 0, 0, time.UTC)
		res.Notes = append(res.Notes, "No payroll history in HalaOS — assumed salary was paid through the end of the previous month. Verify unpaid salary and 13th-month figures against your previous payroll records.")
	}
	if start.Before(in.HireDate) {
		start = in.HireDate
	}
	if !start.After(in.SeparationDate) {
		res.UnpaidDays = workdays(start, in.SeparationDate)
	}
	res.UnpaidSalary = r2(float64(res.UnpaidDays) * res.DailyRate)
	res.Basis["unpaid_salary"] = fmt.Sprintf("%d working days from %s to %s", res.UnpaidDays, start.Format("2006-01-02"), in.SeparationDate.Format("2006-01-02"))

	res.LeaveConversion = r2(in.ConvertibleLeaveDays * res.DailyRate)
	res.OtherDeductions = r2(in.OutstandingLoans)

	service := in.SeparationDate.Sub(in.HireDate).Hours() / 24 / 365.25
	if service < 0 {
		service = 0
	}
	res.YearsOfService = math.Round(service*100) / 100

	if in.Country == "PHL" {
		// PD 851: 1/12 of basic salary earned within the calendar year.
		earned := in.YTDBasicPaid + res.UnpaidSalary
		res.Basis["prorated_13th"] = "(basic pay earned this year incl. unpaid salary) ÷ 12"
		if !hasPayroll {
			yearStart := time.Date(in.SeparationDate.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
			if in.HireDate.After(yearStart) {
				yearStart = in.HireDate
			}
			earned = float64(workdays(yearStart, in.SeparationDate)) * res.DailyRate
			res.Basis["prorated_13th"] = fmt.Sprintf("estimated basic pay earned since %s (%d working days × daily rate) ÷ 12", yearStart.Format("2006-01-02"), workdays(yearStart, in.SeparationDate))
		}
		res.Prorated13th = r2(earned / 12)

		// Credited years: a fraction of at least six months counts as a whole year.
		credited := math.Floor(service)
		if service-credited >= 0.5 {
			credited++
		}
		switch in.SeparationType {
		case "layoff":
			// Art. 298 (redundancy / labor-saving devices): one month per year, min one month.
			res.SeparationPay = r2(math.Max(1, credited) * in.BasicMonthly)
			res.Basis["separation_pay"] = fmt.Sprintf("Labor Code Art. 298: 1 month × %.0f credited year(s), minimum 1 month. If the cause is retrenchment/closure, it is ½ month per year — adjust before approval.", math.Max(1, credited))
		case "retirement":
			if service >= 5 && (in.AgeAtSeparation == 0 || in.AgeAtSeparation >= 60) {
				// RA 7641: ½ month = 22.5 days per year of service.
				res.SeparationPay = r2(22.5 * res.DailyRate * credited)
				res.Basis["separation_pay"] = fmt.Sprintf("RA 7641: 22.5 days × %.0f credited year(s)", credited)
				if in.AgeAtSeparation == 0 {
					res.Notes = append(res.Notes, "Birth date unknown — confirm the employee is at least 60 for statutory retirement pay.")
				}
			} else {
				res.Notes = append(res.Notes, "Below RA 7641 eligibility (5 years of service and age 60) — no statutory retirement pay; check the company retirement plan.")
			}
		case "termination":
			res.Notes = append(res.Notes, "No separation pay for just-cause termination (Art. 297) unless granted as financial assistance.")
		}
		res.Notes = append(res.Notes, "Tax refund/withholding is settled through the annualization on BIR Form 2316.")
	} else {
		res.Notes = append(res.Notes, "Statutory separation entitlements for this country are not computed automatically — review before approval.")
	}
	if in.OutstandingLoans > 0 {
		res.Notes = append(res.Notes, "Outstanding loan balances are deducted; confirm the employee's written authorization on file.")
	}
	res.Total = r2(res.UnpaidSalary + res.Prorated13th + res.LeaveConversion + res.SeparationPay - res.OtherDeductions)
	return res
}

func num(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%.2f", f))
	return n
}

func toFloat(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

// LoadInputs gathers the facts for an employee from the database.
func LoadInputs(ctx context.Context, q *store.Queries, companyID, employeeID int64, sepDate time.Time, sepType string) (*Inputs, error) {
	emp, err := q.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{ID: employeeID, CompanyID: companyID})
	if err != nil {
		return nil, fmt.Errorf("employee: %w", err)
	}
	comp, err := q.GetCompanyByID(ctx, companyID)
	if err != nil {
		return nil, err
	}
	sal, err := q.GetCurrentSalary(ctx, store.GetCurrentSalaryParams{CompanyID: companyID, EmployeeID: employeeID, EffectiveFrom: sepDate})
	if err != nil && emp.HireDate.After(sepDate) {
		// Offer rescinded before the start date: use the agreed starting salary.
		sal, err = q.GetCurrentSalary(ctx, store.GetCurrentSalaryParams{CompanyID: companyID, EmployeeID: employeeID, EffectiveFrom: emp.HireDate})
	}
	if err != nil {
		return nil, fmt.Errorf("no salary on record for the separation date")
	}
	in := &Inputs{Country: comp.Country, BasicMonthly: toFloat(sal.BasicSalary), HireDate: emp.HireDate, SeparationDate: sepDate, SeparationType: sepType}
	if cov, err := q.GetEmployeePayrollCoverage(ctx, store.GetEmployeePayrollCoverageParams{Year: int32(sepDate.Year()), EmployeeID: employeeID, CompanyID: companyID}); err == nil {
		in.LastPaidThrough, in.YTDBasicPaid = cov.LastPaidThrough, cov.YtdBasic
	}
	if bals, err := q.GetConvertibleLeaveBalances(ctx, store.GetConvertibleLeaveBalancesParams{CompanyID: companyID, EmployeeID: employeeID, Year: int32(sepDate.Year())}); err == nil {
		for _, b := range bals {
			if b.Remaining > 0 {
				in.ConvertibleLeaveDays += float64(b.Remaining)
			}
		}
	}
	if loans, err := q.GetEmployeeActiveLoanSummary(ctx, employeeID); err == nil {
		for _, l := range loans {
			in.OutstandingLoans += toFloat(l.RemainingBalance)
		}
	}
	if emp.BirthDate.Valid {
		age := sepDate.Year() - emp.BirthDate.Time.Year()
		if sepDate.YearDay() < emp.BirthDate.Time.YearDay() {
			age--
		}
		in.AgeAtSeparation = age
	}
	return in, nil
}

// ComputeAndStore computes final pay and stores it as "calculated" for HR
// review. It is idempotent per (employee, separation date).
func ComputeAndStore(ctx context.Context, q *store.Queries, companyID, employeeID int64, sepDate time.Time, sepType string) (*store.FinalPay, *Result, error) {
	exists, err := q.FinalPayExistsForSeparation(ctx, store.FinalPayExistsForSeparationParams{CompanyID: companyID, EmployeeID: employeeID, SeparationDate: sepDate})
	if err != nil {
		return nil, nil, err
	}
	if exists {
		return nil, nil, nil
	}
	in, err := LoadInputs(ctx, q, companyID, employeeID, sepDate, sepType)
	if err != nil {
		return nil, nil, err
	}
	res := Compute(*in)
	payload, _ := json.Marshal(map[string]any{"computed_by": "halaos.finalpay.v1", "inputs": in, "result": res})
	fp, err := q.CreateFinalPay(ctx, store.CreateFinalPayParams{
		CompanyID: companyID, EmployeeID: employeeID, SeparationDate: sepDate, SeparationReason: sepType,
		UnpaidSalary: num(res.UnpaidSalary), Prorated13th: num(res.Prorated13th), UnusedLeaveConversion: num(res.LeaveConversion),
		SeparationPay: num(res.SeparationPay), TaxRefund: num(0), OtherDeductions: num(res.OtherDeductions),
		TotalFinalPay: num(res.Total), Payload: payload,
	})
	if err != nil {
		return nil, nil, err
	}
	return &fp, &res, nil
}
