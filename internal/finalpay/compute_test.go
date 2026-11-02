package finalpay

import (
	"math"
	"testing"
	"time"
)

func d(s string) time.Time { t, _ := time.Parse("2006-01-02", s); return t }

func near(a, b float64) bool { return math.Abs(a-b) < 0.011 }

func TestCompute_PHResignation(t *testing.T) {
	r := Compute(Inputs{
		Country: "PHL", BasicMonthly: 26100, HireDate: d("2023-03-01"), SeparationDate: d("2026-11-06"),
		SeparationType: "resignation", LastPaidThrough: d("2026-10-31"), YTDBasicPaid: 261000,
		ConvertibleLeaveDays: 5, OutstandingLoans: 1000,
	})
	if !near(r.DailyRate, 1200) {
		t.Fatalf("daily rate %v", r.DailyRate)
	}
	if r.UnpaidDays != 5 || !near(r.UnpaidSalary, 6000) { // Nov 2–6 2026 = Mon–Fri
		t.Errorf("unpaid %d %v", r.UnpaidDays, r.UnpaidSalary)
	}
	if !near(r.Prorated13th, (261000+6000)/12.0) {
		t.Errorf("13th %v", r.Prorated13th)
	}
	if !near(r.LeaveConversion, 6000) || r.SeparationPay != 0 || !near(r.OtherDeductions, 1000) {
		t.Errorf("%+v", r)
	}
	if !near(r.Total, 6000+22250+6000-1000) {
		t.Errorf("total %v", r.Total)
	}
}

func TestCompute_PHLayoffCreditsHalfYear(t *testing.T) {
	r := Compute(Inputs{Country: "PHL", BasicMonthly: 30000, HireDate: d("2022-01-01"), SeparationDate: d("2026-08-01"), SeparationType: "layoff"})
	// 4 years 7 months → 5 credited years
	if !near(r.SeparationPay, 150000) {
		t.Errorf("layoff pay %v (%v yrs)", r.SeparationPay, r.YearsOfService)
	}
	short := Compute(Inputs{Country: "PHL", BasicMonthly: 30000, HireDate: d("2026-05-01"), SeparationDate: d("2026-07-01"), SeparationType: "layoff"})
	if !near(short.SeparationPay, 30000) {
		t.Errorf("minimum one month: %v", short.SeparationPay)
	}
}

func TestCompute_PHRetirementEligibility(t *testing.T) {
	ok := Compute(Inputs{Country: "PHL", BasicMonthly: 26100, HireDate: d("2016-01-01"), SeparationDate: d("2026-01-02"), SeparationType: "retirement", AgeAtSeparation: 60})
	if !near(ok.SeparationPay, 22.5*1200*10) {
		t.Errorf("retirement %v", ok.SeparationPay)
	}
	young := Compute(Inputs{Country: "PHL", BasicMonthly: 26100, HireDate: d("2016-01-01"), SeparationDate: d("2026-01-02"), SeparationType: "retirement", AgeAtSeparation: 45})
	if young.SeparationPay != 0 {
		t.Errorf("not eligible: %v", young.SeparationPay)
	}
}

func TestCompute_NeverPaidStartsAtHireAndNonPH(t *testing.T) {
	r := Compute(Inputs{Country: "USA", BasicMonthly: 5220, HireDate: d("2026-11-02"), SeparationDate: d("2026-11-04"), SeparationType: "resignation"})
	if r.UnpaidDays != 3 || r.Prorated13th != 0 || r.SeparationPay != 0 || len(r.Notes) == 0 {
		t.Errorf("%+v", r)
	}
}

// A long-tenured employee with no payroll history in HalaOS must not get
// years of "unpaid salary".
func TestCompute_NoPayrollHistoryAssumesPaidToLastMonth(t *testing.T) {
	r := Compute(Inputs{Country: "PHL", BasicMonthly: 26100, HireDate: d("2021-03-01"), SeparationDate: d("2026-11-02"), SeparationType: "resignation"})
	if r.UnpaidDays != 1 || !near(r.UnpaidSalary, 1200) { // Nov 1 (Sun) – Nov 2 (Mon)
		t.Errorf("unpaid %d %v", r.UnpaidDays, r.UnpaidSalary)
	}
	days := workdays(d("2026-01-01"), d("2026-11-02"))
	if !near(r.Prorated13th, r2(float64(days)*1200/12)) {
		t.Errorf("13th %v want %v", r.Prorated13th, float64(days)*1200/12)
	}
	if r.Total > 30000 {
		t.Errorf("total %v is implausible", r.Total)
	}
}
