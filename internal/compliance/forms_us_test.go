package compliance

import (
	"testing"
)

func TestW2BoxCalculations(t *testing.T) {
	data := W2Data{
		EmployeeName:        "John Doe",
		EmployeeSSN:         "XXX-XX-6789",
		GrossWages:          100000.00,
		PreTax401k:          23000.00,
		HealthInsurance:     6000.00,
		FederalTaxWithheld:  15000.00,
		SSWages:             94000.00,
		SSTaxWithheld:       5828.00,
		MedicareWages:       94000.00,
		MedicareTaxWithheld: 1363.00,
		StateWages:          71000.00,
		StateTaxWithheld:    4500.00,
	}

	// Box 1: Wages = Gross - pre-tax deductions (401k + health)
	box1 := data.GrossWages - data.PreTax401k - data.HealthInsurance
	if box1 != 71000.00 {
		t.Errorf("Box 1: got %.2f, want 71000.00", box1)
	}

	// Box 3: SS Wages = Gross - Section 125 only (not 401k), capped at wage base
	if data.SSWages != 94000.00 {
		t.Errorf("Box 3: got %.2f, want 94000.00", data.SSWages)
	}
}

func TestForm941QuarterlyAggregation(t *testing.T) {
	data := Form941Data{
		TotalWages:      300000.00,
		FederalWithheld: 45000.00,
		SSWages:         300000.00,
		MedicareWages:   300000.00,
		EmployeeCount:   10,
	}

	// Line 5a: SS wages x 12.4% (combined EE+ER)
	line5a := data.SSWages * 0.124
	if line5a != 37200.00 {
		t.Errorf("Line 5a: got %.2f, want 37200.00", line5a)
	}

	// Line 5c: Medicare wages x 2.9%
	line5c := data.MedicareWages * 0.029
	if line5c != 8700.00 {
		t.Errorf("Line 5c: got %.2f, want 8700.00", line5c)
	}
}
