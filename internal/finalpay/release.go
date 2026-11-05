package finalpay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/halaostory/halaos/internal/store"
)

// ErrTransition is returned for a status change the workflow does not allow.
var ErrTransition = errors.New("invalid final pay status transition")

// allowed status transitions: calculated → approved → released.
var transitions = map[string]map[string]bool{
	"draft":      {"calculated": true, "approved": true},
	"calculated": {"approved": true, "draft": true},
	"approved":   {"released": true, "calculated": true},
}

// CanTransition reports whether from → to is allowed.
func CanTransition(from, to string) bool { return transitions[from][to] }

// PH: 13th month pay and other benefits are tax-exempt up to this amount (TRAIN law).
const phBenefitsExemptCap = 90000

// TaxableIncome estimates the taxable part of a final pay. Philippine rules:
// unpaid salary is taxable; 13th month + leave conversion are exempt up to
// ₱90,000; separation pay for causes beyond the employee's control (layoff,
// death) and RA 7641 retirement pay are exempt.
func TaxableIncome(country, sepType string, unpaid, thirteenth, leave, sepPay float64) float64 {
	if country != "PHL" {
		return r2(unpaid + thirteenth + leave + sepPay)
	}
	t := unpaid + math.Max(0, thirteenth+leave-phBenefitsExemptCap)
	switch sepType {
	case "layoff", "death", "retirement":
	default:
		t += sepPay
	}
	return r2(t)
}

// Released describes the off-cycle payroll created for a final pay.
type Released struct {
	FinalPay store.FinalPay `json:"final_pay"`
	CycleID  int64          `json:"payroll_cycle_id"`
	RunID    int64          `json:"payroll_run_id"`
}

type txStarter interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Release pays out an approved final pay through a single-employee off-cycle
// payroll (cycle_type final_pay): cycle + run + payroll item + payslip are
// created in one transaction. The cycle is left "computed" so it goes
// through the normal payroll approval (payslip notice, bank file, accounting).
func Release(ctx context.Context, q *store.Queries, db txStarter, companyID, userID, finalPayID int64, now time.Time) (*Released, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qt := q.WithTx(tx)

	fp, err := qt.GetFinalPayByID(ctx, store.GetFinalPayByIDParams{ID: finalPayID, CompanyID: companyID})
	if err != nil {
		return nil, fmt.Errorf("final pay not found")
	}
	if fp.Status != "approved" {
		return nil, fmt.Errorf("%w: only an approved final pay can be released (status is %s)", ErrTransition, fp.Status)
	}
	emp, err := qt.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{ID: fp.EmployeeID, CompanyID: companyID})
	if err != nil {
		return nil, err
	}
	comp, err := qt.GetCompanyByID(ctx, companyID)
	if err != nil {
		return nil, err
	}

	unpaid, thirteenth := toFloat(fp.UnpaidSalary), toFloat(fp.Prorated13th)
	leave, sepPay := toFloat(fp.UnusedLeaveConversion), toFloat(fp.SeparationPay)
	deductions, net := toFloat(fp.OtherDeductions), toFloat(fp.TotalFinalPay)
	gross := r2(unpaid + thirteenth + leave + sepPay)
	taxable := TaxableIncome(comp.Country, fp.SeparationReason, unpaid, thirteenth, leave, sepPay)

	start := time.Date(fp.SeparationDate.Year(), fp.SeparationDate.Month(), 1, 0, 0, 0, 0, time.UTC)
	if emp.HireDate.After(start) {
		start = emp.HireDate
	}
	if start.After(fp.SeparationDate) {
		start = fp.SeparationDate
	}
	name := fmt.Sprintf("Final pay – %s %s", emp.FirstName, emp.LastName)
	if len(name) > 100 {
		name = name[:100]
	}
	cycle, err := qt.CreatePayrollCycle(ctx, store.CreatePayrollCycleParams{
		CompanyID: companyID, Name: name, PeriodStart: start, PeriodEnd: fp.SeparationDate,
		PayDate: now, CycleType: "final_pay", CreatedBy: &userID,
	})
	if err != nil {
		return nil, fmt.Errorf("create cycle: %w", err)
	}
	run, err := qt.CreatePayrollRun(ctx, store.CreatePayrollRunParams{CompanyID: companyID, CycleID: cycle.ID, RunType: "final_pay", InitiatedBy: &userID})
	if err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}
	breakdown, _ := json.Marshal(map[string]any{
		"final_pay_id": fp.ID, "unpaid_salary": unpaid, "prorated_13th": thirteenth, "unused_leave_conversion": leave,
		"separation_pay": sepPay, "loan_deductions": deductions, "taxable_income": taxable,
		"note": "Final pay. Withholding on the taxable portion is settled in the year-end annualization (BIR 2316); enter it here before approval if it must be withheld now.",
	})
	zero := num(0)
	if _, err := qt.CreatePayrollItem(ctx, store.CreatePayrollItemParams{
		RunID: run.ID, EmployeeID: emp.ID, BasicPay: num(unpaid), GrossPay: num(gross), TaxableIncome: num(taxable),
		TotalDeductions: num(deductions), NetPay: num(net),
		SssEe: zero, SssEr: zero, SssEc: zero, PhilhealthEe: zero, PhilhealthEr: zero, PagibigEe: zero, PagibigEr: zero,
		WithholdingTax: zero, Breakdown: breakdown, WorkDays: zero, HoursWorked: zero, OtHours: zero,
		LateDeduction: zero, UndertimeDeduction: zero, HolidayPay: zero, NightDiff: zero, BonusPay: num(thirteenth),
		FederalTax: zero, SocialSecurityEe: zero, SocialSecurityEr: zero, MedicareEe: zero, MedicareEr: zero,
		AdditionalMedicare: zero, StateTax: zero, StateDisability: zero, Futa: zero, Sui: zero, PretaxDeductions: zero,
	}); err != nil {
		return nil, fmt.Errorf("create payroll item: %w", err)
	}
	payslip, _ := json.Marshal(map[string]any{
		"employee_name": emp.FirstName + " " + emp.LastName, "employee_no": emp.EmployeeNo, "type": "final_pay",
		"basic_pay": unpaid, "gross_pay": gross, "net_pay": net, "deductions": deductions,
		"breakdown": map[string]any{"unpaid_salary": unpaid, "prorated_13th": thirteenth, "unused_leave_conversion": leave, "separation_pay": sepPay, "loan_deductions": deductions},
	})
	if _, err := qt.CreatePayslip(ctx, store.CreatePayslipParams{
		CompanyID: companyID, RunID: run.ID, EmployeeID: emp.ID, PeriodStart: start, PeriodEnd: fp.SeparationDate, PayDate: now, Payload: payslip,
	}); err != nil {
		return nil, fmt.Errorf("create payslip: %w", err)
	}
	if err := qt.UpdatePayrollRun(ctx, store.UpdatePayrollRunParams{
		ID: run.ID, TotalEmployees: 1, TotalGross: num(gross), TotalDeductions: num(deductions), TotalNet: num(net), Status: "completed",
	}); err != nil {
		return nil, fmt.Errorf("complete run: %w", err)
	}
	if err := qt.UpdatePayrollCycleStatus(ctx, store.UpdatePayrollCycleStatusParams{ID: cycle.ID, CompanyID: companyID, Status: "computed"}); err != nil {
		return nil, fmt.Errorf("cycle status: %w", err)
	}
	out, err := qt.MarkFinalPayReleased(ctx, store.MarkFinalPayReleasedParams{CycleID: cycle.ID, RunID: run.ID, ID: fp.ID, CompanyID: companyID})
	if err != nil {
		return nil, fmt.Errorf("mark released: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &Released{FinalPay: out, CycleID: cycle.ID, RunID: run.ID}, nil
}
