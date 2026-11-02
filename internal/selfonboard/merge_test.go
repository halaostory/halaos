package selfonboard

import (
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	out, missing := Render("Dear {{employee.first_name}}, welcome to {{ company.name }} as {{employee.position}} — {{bogus}}",
		map[string]string{"employee.first_name": "Ana", "company.name": "Acme", "employee.position": ""})
	if !strings.HasPrefix(out, "Dear Ana, welcome to Acme as {{employee.position}}") {
		t.Errorf("out = %q", out)
	}
	if strings.Join(missing, ",") != "bogus,employee.position" {
		t.Errorf("missing = %v", missing)
	}
}

func TestTokens(t *testing.T) {
	a, ha := NewToken()
	b, _ := NewToken()
	if len(a) != 64 || a == b || HashToken(a) != ha || ha == a {
		t.Error("token generation")
	}
}

func TestValidateGovIDs(t *testing.T) {
	errs := ValidateGovIDs("PHL", map[string]string{"tin": "123-456-789", "sss_no": "34-1234567-8", "philhealth_no": "1234", "pagibig_no": ""})
	if len(errs) != 1 || errs["philhealth_no"] == "" {
		t.Errorf("errs = %v", errs)
	}
	if len(ValidateGovIDs("USA", map[string]string{"tin": "1"})) != 0 {
		t.Error("non-PH ids are not validated")
	}
}

func TestNameMatchesAndMask(t *testing.T) {
	if !NameMatches("  ana  maria CRUZ ", "Ana Cruz") || NameMatches("Ana", "Ana Cruz") || NameMatches("", "Ana Cruz") {
		t.Error("name match")
	}
	if Mask("1234567890") != "••••••7890" || Mask("12") != "••••" || Mask("") != "" {
		t.Errorf("mask %q", Mask("1234567890"))
	}
}
