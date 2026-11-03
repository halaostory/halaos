package payroll

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Payroll history import: when a company switches from another provider
// mid-year, its earlier payslips are imported so the year-end forms (BIR
// 2316, 1604-C alphalist) and year-to-date reports cover the whole year.
// Each pay period becomes a locked, paid cycle of type "imported" that
// those datasets read like any other completed run. (13th month pay is
// computed from salary and months worked, so it does not depend on it.)

// HistoryRow is one employee's pay for one period.
type HistoryRow struct {
	Line           int       `json:"line"`
	EmployeeNo     string    `json:"employee_no"`
	EmployeeID     int64     `json:"employee_id"`
	EmployeeName   string    `json:"employee_name"`
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
	PayDate        time.Time `json:"pay_date"`
	Basic          float64   `json:"basic_pay"`
	Gross          float64   `json:"gross_pay"`
	Taxable        float64   `json:"taxable_income"`
	Deductions     float64   `json:"total_deductions"`
	Net            float64   `json:"net_pay"`
	SSSEE          float64   `json:"sss_ee"`
	SSSER          float64   `json:"sss_er"`
	SSSEC          float64   `json:"sss_ec"`
	PhilHealthEE   float64   `json:"philhealth_ee"`
	PhilHealthER   float64   `json:"philhealth_er"`
	PagIBIGEE      float64   `json:"pagibig_ee"`
	PagIBIGER      float64   `json:"pagibig_er"`
	WithholdingTax float64   `json:"withholding_tax"`
}

// HistoryIssue is a problem with one line of the file.
type HistoryIssue struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// HistoryPlan is what an import would do.
type HistoryPlan struct {
	Rows      int            `json:"rows"`
	Employees int            `json:"employees"`
	Periods   []PeriodTotal  `json:"periods"`
	Issues    []HistoryIssue `json:"issues"`
	Imported  bool           `json:"imported"`
}

// PeriodTotal summarises one pay period of the file.
type PeriodTotal struct {
	PeriodStart string  `json:"period_start"`
	PeriodEnd   string  `json:"period_end"`
	PayDate     string  `json:"pay_date"`
	Employees   int     `json:"employees"`
	Gross       float64 `json:"gross"`
	Net         float64 `json:"net"`
	Tax         float64 `json:"withholding_tax"`
}

// HistoryColumns lists the accepted CSV columns (required ones first).
var HistoryColumns = []string{"employee_no", "period_start", "period_end", "pay_date", "gross_pay", "net_pay",
	"basic_pay", "taxable_income", "total_deductions", "withholding_tax",
	"sss_ee", "sss_er", "sss_ec", "philhealth_ee", "philhealth_er", "pagibig_ee", "pagibig_er"}

var requiredHistory = map[string]bool{"employee_no": true, "period_start": true, "period_end": true, "pay_date": true, "gross_pay": true, "net_pay": true}

// ParseHistoryCSV reads the file. Employees are resolved by employee number;
// numbers may use thousands separators.
func ParseHistoryCSV(r io.Reader, employees map[string]store.ListEmployeeNumbersRow) ([]HistoryRow, []HistoryIssue) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1
	header, err := cr.Read()
	if err != nil {
		return nil, []HistoryIssue{{Line: 1, Message: "the file is empty or not a CSV"}}
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))] = i
	}
	var issues []HistoryIssue
	for name := range requiredHistory {
		if _, ok := col[name]; !ok {
			issues = append(issues, HistoryIssue{Line: 1, Message: "missing column " + name})
		}
	}
	if len(issues) > 0 {
		sort.Slice(issues, func(i, j int) bool { return issues[i].Message < issues[j].Message })
		return nil, issues
	}
	var rows []HistoryRow
	seen := map[string]int{}
	line := 1
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		line++
		if err != nil {
			issues = append(issues, HistoryIssue{Line: line, Message: err.Error()})
			continue
		}
		get := func(name string) string {
			if i, ok := col[name]; ok && i < len(rec) {
				return strings.TrimSpace(rec[i])
			}
			return ""
		}
		if strings.Join(rec, "") == "" {
			continue
		}
		var bad []string
		num := func(name string) float64 {
			s := strings.ReplaceAll(get(name), ",", "")
			if s == "" {
				return 0
			}
			v, err := strconv.ParseFloat(s, 64)
			if err != nil || v < 0 || math.IsInf(v, 0) {
				bad = append(bad, name+" is not a valid amount")
				return 0
			}
			return math.Round(v*100) / 100
		}
		date := func(name string) time.Time {
			for _, layout := range []string{"2006-01-02", "01/02/2006", "1/2/2006"} {
				if d, err := time.Parse(layout, get(name)); err == nil {
					return d
				}
			}
			bad = append(bad, name+" must be a date (YYYY-MM-DD)")
			return time.Time{}
		}
		row := HistoryRow{Line: line, EmployeeNo: get("employee_no"),
			PeriodStart: date("period_start"), PeriodEnd: date("period_end"), PayDate: date("pay_date"),
			Gross: num("gross_pay"), Net: num("net_pay"), Basic: num("basic_pay"), Taxable: num("taxable_income"),
			Deductions: num("total_deductions"), WithholdingTax: num("withholding_tax"),
			SSSEE: num("sss_ee"), SSSER: num("sss_er"), SSSEC: num("sss_ec"),
			PhilHealthEE: num("philhealth_ee"), PhilHealthER: num("philhealth_er"),
			PagIBIGEE: num("pagibig_ee"), PagIBIGER: num("pagibig_er")}
		emp, ok := employees[strings.ToUpper(row.EmployeeNo)]
		if !ok {
			bad = append(bad, "unknown employee_no "+row.EmployeeNo)
		} else {
			row.EmployeeID, row.EmployeeName = emp.ID, strings.TrimSpace(emp.FirstName+" "+emp.LastName)
		}
		if len(bad) == 0 {
			ee := row.SSSEE + row.PhilHealthEE + row.PagIBIGEE
			switch {
			case row.PeriodEnd.Before(row.PeriodStart):
				bad = append(bad, "period_end is before period_start")
			case row.PayDate.After(time.Now().AddDate(0, 0, 1)):
				bad = append(bad, "pay_date is in the future; import only past payroll")
			case row.Gross == 0:
				bad = append(bad, "gross_pay is zero")
			case row.Net > row.Gross+0.01:
				bad = append(bad, "net_pay is more than gross_pay")
			}
			if row.Basic == 0 {
				row.Basic = row.Gross
			}
			if row.Taxable == 0 {
				row.Taxable = math.Max(0, math.Round((row.Gross-ee)*100)/100)
			}
			if row.Deductions == 0 {
				row.Deductions = math.Round((row.Gross-row.Net)*100) / 100
			}
			key := fmt.Sprintf("%d|%s|%s", row.EmployeeID, row.PeriodStart.Format("2006-01-02"), row.PeriodEnd.Format("2006-01-02"))
			if prev, dup := seen[key]; dup {
				bad = append(bad, fmt.Sprintf("duplicate of line %d", prev))
			}
			seen[key] = line
		}
		if len(bad) > 0 {
			issues = append(issues, HistoryIssue{Line: line, Message: strings.Join(bad, "; ")})
			continue
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 && len(issues) == 0 {
		issues = append(issues, HistoryIssue{Line: 1, Message: "no payroll rows found"})
	}
	return rows, issues
}

func num2(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(v, 'f', 2, 64))
	return n
}

type periodKey struct{ start, end, pay string }

func groupPeriods(rows []HistoryRow) ([]periodKey, map[periodKey][]HistoryRow) {
	groups := map[periodKey][]HistoryRow{}
	var keys []periodKey
	for _, r := range rows {
		k := periodKey{r.PeriodStart.Format("2006-01-02"), r.PeriodEnd.Format("2006-01-02"), r.PayDate.Format("2006-01-02")}
		if _, ok := groups[k]; !ok {
			keys = append(keys, k)
		}
		groups[k] = append(groups[k], r)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].start < keys[j].start || (keys[i].start == keys[j].start && keys[i].pay < keys[j].pay)
	})
	return keys, groups
}

// PlanHistory validates rows against the database (no double counting with
// pay already on file) and, unless dryRun, writes them in one transaction.
func (h *Handler) PlanHistory(ctx context.Context, companyID, userID int64, rows []HistoryRow, issues []HistoryIssue, dryRun bool) (*HistoryPlan, error) {
	q := h.queries
	var ok []HistoryRow
	for _, r := range rows {
		n, err := q.CountPayrollItemsInPeriod(ctx, store.CountPayrollItemsInPeriodParams{CompanyID: companyID, EmployeeID: r.EmployeeID, PeriodStart: r.PeriodStart, PeriodEnd: r.PeriodEnd})
		if err != nil {
			return nil, err
		}
		if n > 0 {
			issues = append(issues, HistoryIssue{Line: r.Line, Message: fmt.Sprintf("%s already has pay on file for %s – %s", r.EmployeeName, r.PeriodStart.Format("2006-01-02"), r.PeriodEnd.Format("2006-01-02"))})
			continue
		}
		ok = append(ok, r)
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Line < issues[j].Line })
	plan := &HistoryPlan{Rows: len(ok), Issues: issues, Periods: []PeriodTotal{}}
	if plan.Issues == nil {
		plan.Issues = []HistoryIssue{}
	}
	emps := map[int64]bool{}
	keys, groups := groupPeriods(ok)
	for _, k := range keys {
		pt := PeriodTotal{PeriodStart: k.start, PeriodEnd: k.end, PayDate: k.pay}
		for _, r := range groups[k] {
			emps[r.EmployeeID] = true
			pt.Employees++
			pt.Gross += r.Gross
			pt.Net += r.Net
			pt.Tax += r.WithholdingTax
		}
		pt.Gross, pt.Net, pt.Tax = round2(pt.Gross), round2(pt.Net), round2(pt.Tax)
		plan.Periods = append(plan.Periods, pt)
	}
	plan.Employees = len(emps)
	// All or nothing: a file with problems is never half-imported.
	if dryRun || len(plan.Issues) > 0 || len(ok) == 0 {
		return plan, nil
	}
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qt := q.WithTx(tx)
	for _, k := range keys {
		g := groups[k]
		r0 := g[0]
		cycleID, err := qt.CreateImportedCycle(ctx, store.CreateImportedCycleParams{
			CompanyID: companyID, Name: fmt.Sprintf("Imported %s – %s", r0.PeriodStart.Format("Jan 2"), r0.PeriodEnd.Format("Jan 2, 2006")),
			PeriodStart: r0.PeriodStart, PeriodEnd: r0.PeriodEnd, PayDate: r0.PayDate, CreatedBy: &userID,
		})
		if err != nil {
			return nil, fmt.Errorf("create cycle: %w", err)
		}
		var gross, ded, net float64
		for _, r := range g {
			gross, ded, net = gross+r.Gross, ded+r.Deductions, net+r.Net
		}
		runID, err := qt.CreateImportedRun(ctx, store.CreateImportedRunParams{
			CompanyID: companyID, CycleID: cycleID, TotalEmployees: int32(len(g)),
			TotalGross: num2(gross), TotalDeductions: num2(ded), TotalNet: num2(net), InitiatedBy: &userID,
		})
		if err != nil {
			return nil, fmt.Errorf("create run: %w", err)
		}
		for _, r := range g {
			if err := qt.CreateImportedItem(ctx, store.CreateImportedItemParams{
				RunID: runID, EmployeeID: r.EmployeeID, BasicPay: num2(r.Basic), GrossPay: num2(r.Gross),
				TaxableIncome: num2(r.Taxable), TotalDeductions: num2(r.Deductions), NetPay: num2(r.Net),
				SssEe: num2(r.SSSEE), SssEr: num2(r.SSSER), SssEc: num2(r.SSSEC),
				PhilhealthEe: num2(r.PhilHealthEE), PhilhealthEr: num2(r.PhilHealthER),
				PagibigEe: num2(r.PagIBIGEE), PagibigEr: num2(r.PagIBIGER), WithholdingTax: num2(r.WithholdingTax),
			}); err != nil {
				return nil, fmt.Errorf("line %d: %w", r.Line, err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	plan.Imported = true
	return plan, nil
}

// ImportHistory handles POST /payroll/history/import?dry_run=1 (multipart "file").
func (h *Handler) ImportHistory(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "Upload a CSV file")
		return
	}
	if fh.Size > 10<<20 {
		response.BadRequest(c, "The file is larger than 10 MB")
		return
	}
	f, err := fh.Open()
	if err != nil {
		response.BadRequest(c, "Could not read the file")
		return
	}
	defer f.Close()
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	list, err := h.queries.ListEmployeeNumbers(ctx, companyID)
	if err != nil {
		response.InternalError(c, "Failed to load employees")
		return
	}
	emps := map[string]store.ListEmployeeNumbersRow{}
	for _, e := range list {
		emps[strings.ToUpper(e.EmployeeNo)] = e
	}
	rows, issues := ParseHistoryCSV(f, emps)
	plan, err := h.PlanHistory(ctx, companyID, auth.GetUserID(c), rows, issues, c.Query("dry_run") == "1")
	if err != nil {
		h.logger.Error("payroll history import", "error", err)
		response.InternalError(c, "Import failed; nothing was saved")
		return
	}
	response.OK(c, plan)
}

// ListImportedHistory handles GET /payroll/history.
func (h *Handler) ListImportedHistory(c *gin.Context) {
	rows, err := h.queries.ListImportedCycles(c.Request.Context(), auth.GetCompanyID(c))
	if err != nil {
		response.InternalError(c, "Failed to load history")
		return
	}
	if rows == nil {
		rows = []store.ListImportedCyclesRow{}
	}
	response.OK(c, gin.H{"periods": rows, "columns": HistoryColumns})
}

// DeleteImportedHistory handles DELETE /payroll/history/:id (one imported period).
func (h *Handler) DeleteImportedHistory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid id")
		return
	}
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		response.InternalError(c, "Failed to delete")
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qt := h.queries.WithTx(tx)
	_ = qt.DeleteImportedItems(ctx, store.DeleteImportedItemsParams{ID: id, CompanyID: companyID})
	_ = qt.DeleteImportedRuns(ctx, store.DeleteImportedRunsParams{ID: id, CompanyID: companyID})
	n, err := qt.DeleteImportedCycle(ctx, store.DeleteImportedCycleParams{ID: id, CompanyID: companyID})
	if err != nil || n == 0 {
		response.NotFound(c, "Imported period not found")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		response.InternalError(c, errors.New("failed to delete").Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}
