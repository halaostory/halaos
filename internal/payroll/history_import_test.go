package payroll

import (
	"strings"
	"testing"

	"github.com/halaostory/halaos/internal/store"
)

func TestParseHistoryCSV(t *testing.T) {
	emps := map[string]store.ListEmployeeNumbersRow{
		"EMP-001": {ID: 1, EmployeeNo: "EMP-001", FirstName: "Ana", LastName: "Cruz"},
		"EMP-002": {ID: 2, EmployeeNo: "EMP-002", FirstName: "Ben", LastName: "Lim"},
	}
	csv := "\ufeffEmployee_No,period_start,period_end,pay_date,gross_pay,net_pay,withholding_tax,sss_ee,philhealth_ee,pagibig_ee\n" +
		"emp-001,2026-01-01,2026-01-31,2026-01-31,\"42,000.00\",35000,3500,1350,1050,100\n" +
		"EMP-002,01/01/2026,01/31/2026,01/31/2026,30000,26000,1200,1000,750,100\n" +
		"EMP-003,2026-01-01,2026-01-31,2026-01-31,30000,26000,0,0,0,0\n" +
		"EMP-001,2026-01-01,2026-01-31,2026-01-31,42000,35000,0,0,0,0\n" +
		"EMP-002,2026-02-01,2026-02-28,2026-02-28,30000,31000,0,0,0,0\n" +
		"EMP-002,2026-03-01,2026-02-28,2026-03-31,30000,abc,0,0,0,0\n" +
		",,,,,,,,,\n"
	rows, issues := ParseHistoryCSV(strings.NewReader(csv), emps)
	if len(rows) != 2 {
		t.Fatalf("rows %+v", rows)
	}
	a := rows[0]
	if a.EmployeeID != 1 || a.Gross != 42000 || a.Basic != 42000 || a.Taxable != 42000-1350-1050-100 || a.Deductions != 7000 {
		t.Errorf("defaults: %+v", a)
	}
	if rows[1].PeriodStart.Format("2006-01-02") != "2026-01-01" {
		t.Errorf("US date: %v", rows[1].PeriodStart)
	}
	want := map[int]string{4: "unknown employee_no EMP-003", 5: "duplicate of line 2", 6: "net_pay is more than gross_pay", 7: "net_pay is not a valid amount"}
	if len(issues) != len(want) {
		t.Fatalf("issues %+v", issues)
	}
	for _, is := range issues {
		if !strings.Contains(is.Message, want[is.Line]) {
			t.Errorf("line %d: %q want %q", is.Line, is.Message, want[is.Line])
		}
	}
	keys, groups := groupPeriods(rows)
	if len(keys) != 1 || len(groups[keys[0]]) != 2 {
		t.Errorf("grouping %+v", keys)
	}
}

func TestParseHistoryCSV_MissingColumns(t *testing.T) {
	_, issues := ParseHistoryCSV(strings.NewReader("employee_no,gross_pay\nEMP-001,100\n"), nil)
	if len(issues) != 4 || !strings.Contains(issues[0].Message, "missing column") {
		t.Errorf("%+v", issues)
	}
}
