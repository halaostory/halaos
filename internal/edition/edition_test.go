package edition

import "testing"

func TestPeopleEdition(t *testing.T) {
	c := New(People, "", "")
	if c.ProductName != "HalaPeople" {
		t.Errorf("name %q", c.ProductName)
	}
	for _, m := range []string{ModCore, ModPayroll, ModCompliance, ModLifecycle, ModESign, ModIT, ModTimeOff, ModAI} {
		if !c.Enabled(m) {
			t.Errorf("%s should ship in people", m)
		}
	}
	for _, m := range []string{ModAttendance, ModTalent, ModEngagement, ModIntelligence, ModWorkspace} {
		if c.Enabled(m) {
			t.Errorf("%s must not ship in people", m)
		}
	}
	if !c.CountryLive("PHL") || c.CountryLive("SGP") || c.CountryLive("USA") {
		t.Error("people: only PH is live; SG is coming soon; US is not offered")
	}
}

func TestDefaultsToHalaOS(t *testing.T) {
	for _, e := range []Edition{"", "bogus", HalaOS} {
		c := New(e, "", "")
		if c.Edition != HalaOS || len(c.Modules) != len(All) {
			t.Errorf("%q → %+v", e, c)
		}
	}
	if c := New(People, "Acme HR", "x"); c.ProductName != "Acme HR" || c.Tagline != "x" {
		t.Error("overrides")
	}
}
