package finalpay

import "testing"

func TestCanTransition(t *testing.T) {
	ok := [][2]string{{"calculated", "approved"}, {"approved", "released"}, {"approved", "calculated"}}
	bad := [][2]string{{"calculated", "released"}, {"released", "approved"}, {"released", "calculated"}, {"approved", "bogus"}}
	for _, c := range ok {
		if !CanTransition(c[0], c[1]) {
			t.Errorf("%v should be allowed", c)
		}
	}
	for _, c := range bad {
		if CanTransition(c[0], c[1]) {
			t.Errorf("%v must be rejected", c)
		}
	}
}

func TestTaxableIncome_PH(t *testing.T) {
	// Resignation: unpaid salary + separation pay taxable; benefits under the cap exempt.
	if got := TaxableIncome("PHL", "resignation", 10000, 20000, 5000, 3000); got != 13000 {
		t.Errorf("resignation taxable %v", got)
	}
	// Benefits above the ₱90,000 cap become taxable; layoff separation pay is exempt.
	if got := TaxableIncome("PHL", "layoff", 10000, 80000, 30000, 150000); got != 30000 {
		t.Errorf("layoff taxable %v", got)
	}
	if got := TaxableIncome("USA", "layoff", 10000, 0, 500, 2000); got != 12500 {
		t.Errorf("non-PH taxable %v", got)
	}
}
