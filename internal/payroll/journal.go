package payroll

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Payroll journal entries (Warp: "after each payroll run, journal entries
// are created" for QuickBooks / Xero / NetSuite). One balanced entry per
// run: wages and employer contributions are expensed; withholding tax,
// statutory contributions and other deductions become liabilities; net pay
// is credited to salaries payable (or the bank account that pays it).

// GLAccount is one account in the company's chart of accounts.
type GLAccount struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Journal account roles.
const (
	AcctSalaries       = "salaries_expense"
	AcctEmployerContr  = "employer_contributions_expense"
	AcctWithholding    = "withholding_tax_payable"
	AcctSSS            = "sss_payable"
	AcctPhilHealth     = "philhealth_payable"
	AcctPagIBIG        = "pagibig_payable"
	AcctOtherDeduction = "other_deductions_payable"
	AcctNetPay         = "salaries_payable"
)

// JournalRoles lists the roles in display order.
var JournalRoles = []string{AcctSalaries, AcctEmployerContr, AcctWithholding, AcctSSS, AcctPhilHealth, AcctPagIBIG, AcctOtherDeduction, AcctNetPay}

// DefaultGLAccounts is a Philippine starter chart; companies remap it.
var DefaultGLAccounts = map[string]GLAccount{
	AcctSalaries:       {"6100", "Salaries and Wages"},
	AcctEmployerContr:  {"6110", "Employer Statutory Contributions"},
	AcctWithholding:    {"2110", "Withholding Tax Payable - Compensation"},
	AcctSSS:            {"2120", "SSS Contributions Payable"},
	AcctPhilHealth:     {"2130", "PhilHealth Contributions Payable"},
	AcctPagIBIG:        {"2140", "Pag-IBIG Contributions Payable"},
	AcctOtherDeduction: {"2190", "Other Payroll Deductions Payable"},
	AcctNetPay:         {"2100", "Salaries Payable"},
}

// JournalLine is one debit or credit.
type JournalLine struct {
	Role        string  `json:"role"`
	AccountCode string  `json:"account_code"`
	AccountName string  `json:"account_name"`
	Description string  `json:"description"`
	Debit       float64 `json:"debit"`
	Credit      float64 `json:"credit"`
}

// Journal is a balanced entry for one payroll run.
type Journal struct {
	RunID       int64         `json:"run_id"`
	Number      string        `json:"number"`
	Date        string        `json:"date"`
	Memo        string        `json:"memo"`
	Currency    string        `json:"currency"`
	Lines       []JournalLine `json:"lines"`
	TotalDebit  float64       `json:"total_debit"`
	TotalCredit float64       `json:"total_credit"`
}


// BuildJournal turns run totals into a balanced journal entry.
func BuildJournal(t store.GetPayrollRunJournalTotalsRow, accounts map[string]GLAccount, currency string) Journal {
	acct := func(role string) GLAccount {
		if a, ok := accounts[role]; ok && a.Code != "" {
			return a
		}
		return DefaultGLAccounts[role]
	}
	er := round2(t.SssEr + t.SssEc + t.PhilhealthEr + t.PagibigEr)
	sss := round2(t.SssEe + t.SssEr + t.SssEc)
	ph := round2(t.PhilhealthEe + t.PhilhealthEr)
	pi := round2(t.PagibigEe + t.PagibigEr)
	gross, net, wtax := round2(t.Gross), round2(t.Net), round2(t.Wtax)
	// Whatever the employee had deducted beyond tax and statutory shares
	// (loans, benefits, cash advances) is owed to someone else.
	other := round2(gross - net - wtax - t.SssEe - t.PhilhealthEe - t.PagibigEe)
	period := t.PeriodStart.Format("Jan 2") + "–" + t.PeriodEnd.Format("Jan 2, 2006")
	j := Journal{
		RunID: t.ID, Number: fmt.Sprintf("PR-%d", t.ID), Date: t.PayDate.Format("2006-01-02"), Currency: currency,
		Memo: fmt.Sprintf("Payroll %s (%s), %d %s", t.CycleName, period, t.TotalEmployees, map[bool]string{true: "employee", false: "employees"}[t.TotalEmployees == 1]),
	}
	add := func(role, desc string, debit, credit float64) {
		if debit == 0 && credit == 0 {
			return
		}
		a := acct(role)
		j.Lines = append(j.Lines, JournalLine{Role: role, AccountCode: a.Code, AccountName: a.Name, Description: desc, Debit: debit, Credit: credit})
	}
	add(AcctSalaries, "Gross pay", gross, 0)
	if other < 0 { // non-deductible additions paid on top of gross (e.g. reimbursements)
		add(AcctSalaries, "Additions to net pay", -other, 0)
		other = 0
	}
	add(AcctEmployerContr, "Employer SSS/EC, PhilHealth, Pag-IBIG", er, 0)
	add(AcctWithholding, "Withholding tax on compensation", 0, wtax)
	add(AcctSSS, "SSS (employee + employer + EC)", 0, sss)
	add(AcctPhilHealth, "PhilHealth (employee + employer)", 0, ph)
	add(AcctPagIBIG, "Pag-IBIG (employee + employer)", 0, pi)
	add(AcctOtherDeduction, "Loans, benefits and other deductions", 0, other)
	add(AcctNetPay, "Net pay", 0, net)
	for _, l := range j.Lines {
		j.TotalDebit += l.Debit
		j.TotalCredit += l.Credit
	}
	j.TotalDebit, j.TotalCredit = round2(j.TotalDebit), round2(j.TotalCredit)
	return j
}

// QuickBooksCSV renders the journal in QuickBooks Online's journal import layout.
func (j Journal) QuickBooksCSV() []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"JournalNo", "JournalDate", "Currency", "Memo", "AccountName", "Debits", "Credits", "Description"})
	date, _ := time.Parse("2006-01-02", j.Date)
	for _, l := range j.Lines {
		_ = w.Write([]string{j.Number, date.Format("01/02/2006"), j.Currency, j.Memo, l.AccountName, amt(l.Debit), amt(l.Credit), l.Description})
	}
	w.Flush()
	return buf.Bytes()
}

// XeroCSV renders the journal in Xero's manual journal import layout
// (positive amounts are debits, negative are credits).
func (j Journal) XeroCSV() []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"*Narration", "*Date", "Description", "*AccountCode", "*TaxRate", "*Amount"})
	date, _ := time.Parse("2006-01-02", j.Date)
	for _, l := range j.Lines {
		v := l.Debit - l.Credit
		_ = w.Write([]string{j.Memo, date.Format("02/01/2006"), l.Description, l.AccountCode, "Tax Exempt", strconv.FormatFloat(v, 'f', 2, 64)})
	}
	w.Flush()
	return buf.Bytes()
}

func amt(v float64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func (h *Handler) glAccounts(c *gin.Context, companyID int64) map[string]GLAccount {
	out := map[string]GLAccount{}
	for k, v := range DefaultGLAccounts {
		out[k] = v
	}
	raw, err := h.queries.GetGLAccountMapping(c.Request.Context(), companyID)
	if err == nil {
		var saved map[string]GLAccount
		if json.Unmarshal(raw, &saved) == nil {
			for k, v := range saved {
				if _, ok := DefaultGLAccounts[k]; ok && v.Code != "" {
					out[k] = v
				}
			}
		}
	}
	return out
}

// GetGLMapping handles GET /payroll/gl-accounts.
func (h *Handler) GetGLMapping(c *gin.Context) {
	response.OK(c, gin.H{"roles": JournalRoles, "accounts": h.glAccounts(c, auth.GetCompanyID(c))})
}

// UpdateGLMapping handles PUT /payroll/gl-accounts.
func (h *Handler) UpdateGLMapping(c *gin.Context) {
	var req map[string]GLAccount
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	clean := map[string]GLAccount{}
	for k, v := range req {
		if _, ok := DefaultGLAccounts[k]; !ok {
			response.BadRequest(c, "Unknown account role: "+k)
			return
		}
		if v.Code == "" || len(v.Code) > 40 || len(v.Name) > 120 {
			response.BadRequest(c, "Each account needs a code (max 40 chars) and a name (max 120)")
			return
		}
		clean[k] = v
	}
	raw, _ := json.Marshal(clean)
	uid := auth.GetUserID(c)
	if err := h.queries.UpsertGLAccountMapping(c.Request.Context(), store.UpsertGLAccountMappingParams{
		CompanyID: auth.GetCompanyID(c), Accounts: raw, UpdatedBy: &uid,
	}); err != nil {
		response.InternalError(c, "Failed to save accounts")
		return
	}
	h.GetGLMapping(c)
}

// GetRunJournal handles GET /payroll/runs/:id/journal?format=json|quickbooks|xero.
func (h *Handler) GetRunJournal(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid id")
		return
	}
	companyID := auth.GetCompanyID(c)
	runID := id
	if c.Param("cycle") == "1" { // /payroll/cycles/:id/journal: latest calculated run
		rid, err := h.queries.LatestCompletedRunForCycle(c.Request.Context(), store.LatestCompletedRunForCycleParams{CycleID: id, CompanyID: companyID})
		if err != nil {
			response.NotFound(c, "This cycle has no calculated payroll run yet")
			return
		}
		runID = rid
	}
	tot, err := h.queries.GetPayrollRunJournalTotals(c.Request.Context(), store.GetPayrollRunJournalTotalsParams{ID: runID, CompanyID: companyID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(c, "Payroll run not found")
			return
		}
		response.InternalError(c, "Failed to build journal")
		return
	}
	currency := "PHP"
	if comp, err := h.queries.GetCompanyByID(c.Request.Context(), companyID); err == nil && comp.Currency != "" {
		currency = comp.Currency
	}
	j := BuildJournal(tot, h.glAccounts(c, companyID), currency)
	switch c.Query("format") {
	case "quickbooks":
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="journal-%s-quickbooks.csv"`, j.Number))
		c.Data(200, "text/csv", j.QuickBooksCSV())
	case "xero":
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="journal-%s-xero.csv"`, j.Number))
		c.Data(200, "text/csv", j.XeroCSV())
	default:
		response.OK(c, j)
	}
}
