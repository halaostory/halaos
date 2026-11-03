package payroll

import (
	"strings"
	"testing"
	"time"

	"github.com/halaostory/halaos/internal/store"
)

func journalTotals() store.GetPayrollRunJournalTotalsRow {
	return store.GetPayrollRunJournalTotalsRow{
		ID: 12, CycleName: "Nov 2026 A", TotalEmployees: 3,
		PayDate:     time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC),
		PeriodStart: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), PeriodEnd: time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC),
		Gross: 90000, Wtax: 6200.5, SssEe: 2250, SssEr: 4750, SssEc: 30,
		PhilhealthEe: 2250, PhilhealthEr: 2250, PagibigEe: 300, PagibigEr: 300,
		Net: 90000 - 6200.5 - 2250 - 2250 - 300 - 1500, // 1,500 loan deduction
	}
}

func TestBuildJournal_Balanced(t *testing.T) {
	j := BuildJournal(journalTotals(), map[string]GLAccount{AcctNetPay: {Code: "1010", Name: "BDO Payroll Account"}}, "PHP")
	if j.TotalDebit != j.TotalCredit {
		t.Fatalf("unbalanced: %v vs %v\n%+v", j.TotalDebit, j.TotalCredit, j.Lines)
	}
	want := map[string][2]float64{
		AcctSalaries: {90000, 0}, AcctEmployerContr: {7330, 0}, AcctWithholding: {0, 6200.5},
		AcctSSS: {0, 7030}, AcctPhilHealth: {0, 4500}, AcctPagIBIG: {0, 600}, AcctOtherDeduction: {0, 1500}, AcctNetPay: {0, 77499.5},
	}
	for _, l := range j.Lines {
		w, ok := want[l.Role]
		if !ok || l.Debit != w[0] || l.Credit != w[1] {
			t.Errorf("%s: %v/%v want %v", l.Role, l.Debit, l.Credit, w)
		}
	}
	if j.Lines[len(j.Lines)-1].AccountCode != "1010" {
		t.Errorf("custom account not applied: %+v", j.Lines[len(j.Lines)-1])
	}
}

func TestJournalCSV(t *testing.T) {
	j := BuildJournal(journalTotals(), nil, "PHP")
	qb := string(j.QuickBooksCSV())
	if !strings.HasPrefix(qb, "JournalNo,JournalDate,Currency,Memo,AccountName,Debits,Credits,Description\n") ||
		!strings.Contains(qb, "PR-12,11/15/2026,PHP") || !strings.Contains(qb, "Salaries and Wages,90000.00,,Gross pay") {
		t.Errorf("quickbooks:\n%s", qb)
	}
	xero := string(j.XeroCSV())
	if !strings.Contains(xero, "15/11/2026,Gross pay,6100,Tax Exempt,90000.00") || !strings.Contains(xero, ",2100,Tax Exempt,-77499.50") {
		t.Errorf("xero:\n%s", xero)
	}
}
