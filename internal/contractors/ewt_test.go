package contractors

import (
	"testing"

	"github.com/halaostory/halaos/internal/store"
)

func TestCompute(t *testing.T) {
	b, err := Compute(50000, 0.05, false)
	if err != nil || b.Base != 50000 || b.EWT != 2500 || b.Net != 47500 || b.VAT != 0 {
		t.Errorf("non-VAT: %+v %v", b, err)
	}
	b, _ = Compute(112000, 0.10, true)
	if b.Base != 100000 || b.VAT != 12000 || b.EWT != 10000 || b.Net != 102000 {
		t.Errorf("VAT-registered: %+v", b)
	}
	if _, err := Compute(0, 0.05, false); err == nil {
		t.Error("zero amount must fail")
	}
	if _, err := Compute(100, 0.9, false); err == nil {
		t.Error("absurd rate must fail")
	}
}

func TestATCs(t *testing.T) {
	if DefaultATC("individual").Code != "WI010" || DefaultATC("company").Rate != 0.10 {
		t.Error("defaults")
	}
	if a, ok := LookupATC(" wc011 "); !ok || a.Rate != 0.15 {
		t.Error("lookup")
	}
	if !ValidTIN("123-456-789-000") || ValidTIN("12-34") {
		t.Error("tin")
	}
}

func TestQuarterBoundsAndRows2307(t *testing.T) {
	s, e, err := QuarterBounds("2026-q3")
	if err != nil || s.Format("2006-01-02") != "2026-07-01" || e.Format("2006-01-02") != "2026-09-30" {
		t.Fatalf("%v %v %v", s, e, err)
	}
	if _, _, err := QuarterBounds("2026-Q5"); err == nil {
		t.Error("bad quarter")
	}
	rows := Rows2307([]store.ContractorWithholdingByPayeeRow{
		{AtcCode: "WI010", Month: 7, Base: 50000, Ewt: 2500},
		{AtcCode: "WI010", Month: 9, Base: 20000, Ewt: 1000},
		{AtcCode: "WI120", Month: 8, Base: 10000, Ewt: 200},
	}, s)
	if len(rows) != 2 || rows[0].Months != [3]float64{50000, 0, 20000} || rows[0].Withheld != 3500 || rows[1].Months[1] != 10000 {
		t.Errorf("%+v", rows)
	}
}
