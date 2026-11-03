package payroll

import "testing"

func TestPeriodFraction(t *testing.T) {
	cases := map[string]float64{"monthly": 1, "semi_monthly": 0.5, "": 1, "weekly": 12.0 / 52}
	for f, want := range cases {
		if got := periodFraction(f); got != want {
			t.Errorf("%q: %v", f, got)
		}
	}
}

func TestPeriodTax_SemiMonthlyIsHalfOfMonthly(t *testing.T) {
	// toy monthly table: 15% over 20,833
	monthly := func(x float64) float64 {
		if x <= 20833 {
			return 0
		}
		return (x - 20833) * 0.15
	}
	// 40,000/month paid semi-monthly: 20,000 taxable per cutoff
	if got, want := periodTax(20000, 0.5, monthly), round2(monthly(40000)/2); got != want {
		t.Errorf("semi-monthly tax %v want %v", got, want)
	}
	if periodTax(0, 0.5, monthly) != 0 || periodTax(20000, 1, monthly) != 0 {
		t.Error("below threshold / zero")
	}
}

func TestScheduleScope(t *testing.T) {
	legacy := scheduleScope{}
	if !legacy.includes(0) || !legacy.includes(5) {
		t.Error("no schedules: everyone is paid")
	}
	def := scheduleScope{ScheduleID: 1, IsDefault: true}
	if !def.includes(0) || !def.includes(1) || def.includes(2) {
		t.Error("default schedule: unassigned + its own")
	}
	other := scheduleScope{ScheduleID: 2}
	if other.includes(0) || other.includes(1) || !other.includes(2) {
		t.Error("non-default schedule: only assigned employees")
	}
}

func TestComputePay_SemiMonthlyPaysHalf(t *testing.T) {
	calc := &Calculator{}
	pd := EmployeePayData{BasicSalary: 42000, DaysWorked: 11}
	calc.computePay(&pd, 11, 0.5)
	if pd.BasicPay != 21000 || pd.GrossPay != 21000 {
		t.Errorf("semi-monthly basic %v gross %v (was paying the full month before)", pd.BasicPay, pd.GrossPay)
	}
	pd = EmployeePayData{BasicSalary: 42000, DaysWorked: 22}
	calc.computePay(&pd, 22, 1)
	if pd.BasicPay != 42000 {
		t.Errorf("monthly basic %v", pd.BasicPay)
	}
}
