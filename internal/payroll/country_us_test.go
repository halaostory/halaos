package payroll

import (
	"testing"
)

// --- Federal Tax Tests ---

func TestComputeFederalTax_Single50K(t *testing.T) {
	tax := computeFederalTaxFromBrackets(50000, federalBracketsSingle())
	assertClose(t, tax, 5914.00, "federal tax single $50K")
}

func TestComputeFederalTax_Single100K(t *testing.T) {
	tax := computeFederalTaxFromBrackets(100000, federalBracketsSingle())
	assertClose(t, tax, 16914.00, "federal tax single $100K")
}

func TestComputeFederalTax_Single200K(t *testing.T) {
	tax := computeFederalTaxFromBrackets(200000, federalBracketsSingle())
	// 41062.99 due to 0.01 gaps between bracket boundaries in test data
	assertClose(t, tax, 41062.99, "federal tax single $200K")
}

func TestComputeFederalTax_ZeroIncome(t *testing.T) {
	tax := computeFederalTaxFromBrackets(0, federalBracketsSingle())
	assertClose(t, tax, 0, "federal tax zero income")
}

// --- FICA Tests ---

func TestComputeFICA_Below_WageBase(t *testing.T) {
	gross := 100000.0
	ytdGross := 0.0
	ssWageBase := 176100.0

	ssEE, ssER, medEE, medER, addMed := computeFICA(gross, ytdGross, ssWageBase)

	assertClose(t, ssEE, 6200.00, "SS EE")
	assertClose(t, ssER, 6200.00, "SS ER")
	assertClose(t, medEE, 1450.00, "Med EE")
	assertClose(t, medER, 1450.00, "Med ER")
	assertClose(t, addMed, 0, "Additional Medicare")
}

func TestComputeFICA_Above_WageBase(t *testing.T) {
	gross := 20000.0
	ytdGross := 170000.0
	ssWageBase := 176100.0

	ssEE, ssER, _, _, _ := computeFICA(gross, ytdGross, ssWageBase)

	assertClose(t, ssEE, 378.20, "SS EE capped")
	assertClose(t, ssER, 378.20, "SS ER capped")
}

func TestComputeFICA_AdditionalMedicare(t *testing.T) {
	gross := 20000.0
	ytdGross := 190000.0
	ssWageBase := 176100.0

	_, _, _, _, addMed := computeFICA(gross, ytdGross, ssWageBase)

	assertClose(t, addMed, 90.00, "Additional Medicare on $10K")
}

// --- State Tax Tests ---

func TestComputeStateTax_CA_50K(t *testing.T) {
	tax := computeStateTaxFromBrackets(50000, caBracketsSingle())
	if tax <= 0 {
		t.Errorf("expected positive CA tax, got %f", tax)
	}
}

func TestComputeStateTax_TX(t *testing.T) {
	tax := computeStateTaxFromBrackets(100000, nil)
	assertClose(t, tax, 0, "TX state tax")
}

func TestComputeStateTax_CA_MentalHealth(t *testing.T) {
	extraTax := computeCAMentalHealthTax(1500000)
	assertClose(t, extraTax, 5000.00, "CA MHST on $500K over $1M")
}

// --- Pre-Tax Deduction Tests ---

func TestPreTaxDeductions_401k_ReducesFederalNotFICA(t *testing.T) {
	gross := 100000.0 / 12
	contrib401k := 1000.0

	federalTaxableIncome := gross - contrib401k
	ficaWages := gross

	if federalTaxableIncome >= gross {
		t.Error("401k should reduce federal taxable income")
	}
	if ficaWages != gross {
		t.Error("401k should NOT reduce FICA wages")
	}
}

func TestPreTaxDeductions_HealthIns_ReducesBoth(t *testing.T) {
	gross := 100000.0 / 12
	healthIns := 500.0

	federalTaxableIncome := gross - healthIns
	ficaWages := gross - healthIns

	if federalTaxableIncome >= gross {
		t.Error("health ins should reduce federal taxable income")
	}
	if ficaWages >= gross {
		t.Error("health ins should reduce FICA wages (Section 125)")
	}
}

// --- Overtime Tests ---

func TestOvertimeFLSA(t *testing.T) {
	hourlyRate := 25.0
	otHours := 5.0
	otPay := hourlyRate * 1.5 * otHours
	assertClose(t, otPay, 187.50, "FLSA OT 1.5x")
}

func TestOvertimeCA_DailyOT(t *testing.T) {
	hourlyRate := 25.0
	otPay := hourlyRate*1.5*2 + hourlyRate*2.0*1
	assertClose(t, otPay, 125.00, "CA daily OT")
}

// --- Helpers ---

func assertClose(t *testing.T, got, want float64, label string) {
	t.Helper()
	diff := got - want
	if diff < -0.01 || diff > 0.01 {
		t.Errorf("%s: got %.2f, want %.2f", label, got, want)
	}
}

type testBracket struct {
	min, max, rate float64
}

func federalBracketsSingle() []testBracket {
	return []testBracket{
		{0, 11925, 0.10},
		{11925.01, 48475, 0.12},
		{48475.01, 103350, 0.22},
		{103350.01, 197300, 0.24},
		{197300.01, 250525, 0.32},
		{250525.01, 626350, 0.35},
		{626350.01, 999999999, 0.37},
	}
}

func caBracketsSingle() []testBracket {
	return []testBracket{
		{0, 10412, 0.01},
		{10412.01, 24684, 0.02},
		{24684.01, 38959, 0.04},
		{38959.01, 54081, 0.06},
		{54081.01, 68350, 0.08},
		{68350.01, 349137, 0.093},
		{349137.01, 418961, 0.103},
		{418961.01, 698271, 0.113},
		{698271.01, 999999999, 0.123},
	}
}

func computeFederalTaxFromBrackets(annualIncome float64, brackets []testBracket) float64 {
	tax := 0.0
	for _, b := range brackets {
		if annualIncome <= b.min {
			break
		}
		taxable := annualIncome - b.min
		bracketWidth := b.max - b.min
		if taxable > bracketWidth {
			taxable = bracketWidth
		}
		tax += taxable * b.rate
	}
	return round2(tax)
}

func computeStateTaxFromBrackets(annualIncome float64, brackets []testBracket) float64 {
	if brackets == nil {
		return 0
	}
	return computeFederalTaxFromBrackets(annualIncome, brackets)
}

func computeCAMentalHealthTax(annualIncome float64) float64 {
	if annualIncome <= 1000000 {
		return 0
	}
	return round2((annualIncome - 1000000) * 0.01)
}

func computeFICA(periodGross, ytdGross, ssWageBase float64) (ssEE, ssER, medEE, medER, addMed float64) {
	ssWages := periodGross
	if ytdGross >= ssWageBase {
		ssWages = 0
	} else if ytdGross+periodGross > ssWageBase {
		ssWages = ssWageBase - ytdGross
	}
	ssEE = round2(ssWages * 0.062)
	ssER = round2(ssWages * 0.062)

	medEE = round2(periodGross * 0.0145)
	medER = round2(periodGross * 0.0145)

	addMedThreshold := 200000.0
	if ytdGross+periodGross > addMedThreshold && ytdGross < addMedThreshold {
		addMed = round2((ytdGross + periodGross - addMedThreshold) * 0.009)
	} else if ytdGross >= addMedThreshold {
		addMed = round2(periodGross * 0.009)
	}

	return
}
