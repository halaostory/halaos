package routines

import (
	"context"
	"fmt"
	"strings"

	"github.com/halaostory/halaos/internal/govfiling"
	"github.com/halaostory/halaos/internal/lifecycle"
	"github.com/halaostory/halaos/internal/payroll"
	"github.com/halaostory/halaos/internal/store"
)

// Engine connects the routines to the services they drive. Every
// dependency is optional; a missing one turns its step into a hand-off.
type Engine struct {
	*Service
	Gov        *govfiling.Service
	Calculator *payroll.Calculator
	// Enabled reports whether an edition module ships (nil: all modules).
	Enabled func(module string) bool
}

var _ lifecycle.RoutineHook = (*Engine)(nil)

func (e *Engine) module(m string) bool { return e.Enabled == nil || e.Enabled(m) }

// stepTitles turns lifecycle step keys into run-log lines.
var stepTitles = map[string]string{
	"employee_created":       "Created the employee record",
	"compensation_set":       "Set compensation",
	"time_off_enrolled":      "Enrolled in time-off policies",
	"custom_fields_saved":    "Saved custom fields",
	"onboarding_checklist":   "Created the onboarding checklist",
	"access_provisioning":    "Queued app account provisioning",
	"accounting_sync":        "Queued sync to accounting",
	"offer_letter_sent":      "Sent the offer letter for e-signature",
	"self_onboarding_invite": "Sent the self-onboarding invite",
	"gov_registration":       "Scheduled agency registrations",
	"clearance":              "Opened clearance",
	"offboarding_checklist":  "Created the offboarding checklist",
	"status_separated":       "Marked the employee as separated",
	"access_revocation":      "Scheduled access revocation",
	"final_pay":              "Computed final pay",
	"device_recall":          "Locked and recalled devices",
}

func fromSteps(steps []lifecycle.Step, skip map[string]bool) []Action {
	var out []Action
	for _, st := range steps {
		if skip[st.Key] {
			continue
		}
		title := stepTitles[st.Key]
		if title == "" {
			title = strings.ReplaceAll(st.Key, "_", " ")
		}
		status := ActDone
		switch st.Status {
		case "skipped":
			status = ActSkipped
		case "scheduled":
			status = ActWaiting
		}
		out = append(out, Action{Key: st.Key, Title: title, Status: status, Rationale: st.Detail})
	}
	return out
}

// customSteps turns the routine's plain-language instructions (one per
// line) into hand-offs, so company-specific steps show up in every run.
func customSteps(instructions, subject string) []Action {
	var out []Action
	for _, line := range strings.Split(instructions, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "-*•0123456789.) "))
		if line == "" {
			continue
		}
		out = append(out, Action{Key: "custom", Title: line, Status: ActPerson, Rationale: "Your routine instruction for " + subject})
	}
	return out
}

func employeeName(e store.Employee) string {
	if e.DisplayName != nil && *e.DisplayName != "" {
		return *e.DisplayName
	}
	return strings.TrimSpace(e.FirstName + " " + e.LastName)
}

// AfterHire runs the "New hire" routine once a hire is committed.
func (e *Engine) AfterHire(ctx context.Context, companyID, _ int64, emp store.Employee, steps []lifecycle.Step) {
	set := e.Setting(ctx, companyID, KeyNewHire)
	if !set.Enabled {
		return
	}
	name := employeeName(emp)
	run := Run{
		Trigger:     fmt.Sprintf("%s was hired (starts %s)", name, emp.HireDate.Format("Jan 2, 2006")),
		SubjectType: "employee", SubjectID: emp.ID, SubjectName: name,
	}
	handledGov := false
	var extra []Action

	// Agency registrations (Philippines).
	if comp, err := e.q.GetCompanyByID(ctx, companyID); err == nil && comp.Country == "PHL" && e.Gov != nil && e.module("compliance") {
		handledGov = true
		if set.Autonomy == Recommend {
			extra = append(extra, Action{Key: "gov_registration", Title: "Prepare SSS R-1A, PhilHealth ER2, Pag-IBIG and BIR 1902 registrations", Status: ActProposed,
				Rationale: "New employees must be reported to each agency within 30 days of hire (BIR 1902 within 10 days if they have no TIN).", Link: "/gov-filings"})
		} else {
			act := e.Gov.PrepareNewHireRegistrations(ctx, companyID)
			for _, it := range act.Prepared {
				extra = append(extra, Action{Key: "gov_registration", Title: fmt.Sprintf("Prepared %s (%s)", it.Name, it.Period), Status: ActDone,
					Rationale: "Built from the new hire's details and pre-flight checked.", Link: "/gov-filings"})
			}
			for _, it := range act.Failed {
				extra = append(extra, Action{Key: "gov_registration", Title: fmt.Sprintf("Could not prepare %s", it.Name), Status: ActFailed, Rationale: it.Error, Link: "/gov-filings"})
			}
			if len(act.Prepared) > 0 {
				extra = append(extra, Action{Key: "approve_registrations", Title: "Review and approve the agency registrations", Status: ActPerson,
					Rationale: "Government submissions always wait for a person's approval.", Link: "/gov-filings"})
			} else if len(act.Failed) == 0 {
				extra = append(extra, Action{Key: "gov_registration", Title: "Agency registrations", Status: ActWaiting,
					Rationale: "Already prepared for this month, or waiting for the new hire's IDs from the onboarding portal; the filing autopilot retries daily.", Link: "/gov-filings"})
			}
		}
	}

	// Equipment.
	if e.module("it") {
		if devs, err := e.q.ListAssignedDevicesForEmployee(ctx, store.ListAssignedDevicesForEmployeeParams{AssignedEmployeeID: &emp.ID, CompanyID: companyID}); err == nil && len(devs) == 0 {
			extra = append(extra, Action{Key: "device", Title: "Assign a laptop", Status: ActPerson,
				Rationale: "No device is assigned yet; assign one so it can be enrolled and recalled at exit.", Link: "/devices"})
		}
	}

	// Benefits.
	if e.module("benefits") {
		if plans, err := e.q.ListBenefitPlans(ctx, companyID); err == nil && len(plans) > 0 {
			extra = append(extra, Action{Key: "benefits", Title: "Enroll in benefits (HMO and other plans)", Status: ActPerson,
				Rationale: fmt.Sprintf("%d plan(s) are available; enrollment needs the employee's choices.", len(plans)), Link: "/benefits"})
		}
	}

	// Manager heads-up.
	if emp.ManagerID != nil {
		if mgr, err := e.q.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{ID: *emp.ManagerID, CompanyID: companyID}); err == nil && mgr.UserID != nil {
			title := fmt.Sprintf("Told %s that %s starts on %s", employeeName(mgr), name, emp.HireDate.Format("Jan 2"))
			if set.Autonomy == Recommend {
				extra = append(extra, Action{Key: "manager_notice", Title: fmt.Sprintf("Tell %s that %s starts on %s", employeeName(mgr), name, emp.HireDate.Format("Jan 2")), Status: ActProposed})
			} else if _, err := e.q.CreateNotification(ctx, store.CreateNotificationParams{
				CompanyID: companyID, UserID: *mgr.UserID, Category: "onboarding",
				Title:   "New team member",
				Message: fmt.Sprintf("%s joins your team on %s. Plan their first week and a 1:1.", name, emp.HireDate.Format("Mon, Jan 2")),
			}); err == nil {
				extra = append(extra, Action{Key: "manager_notice", Title: title, Status: ActDone, Rationale: "Managers own the first week."})
			}
		}
	}

	run.Actions = append(fromSteps(steps, map[string]bool{"gov_registration": handledGov}), extra...)
	run.Actions = append(run.Actions, customSteps(set.Instructions, name)...)
	_, _ = e.Record(ctx, companyID, KeyNewHire, set.Autonomy, run)
}

// AfterTerminate runs the "Offboarding" routine once a termination is committed.
func (e *Engine) AfterTerminate(ctx context.Context, companyID, _ int64, sep store.EmployeeSeparation, steps []lifecycle.Step) {
	set := e.Setting(ctx, companyID, KeyOffboarding)
	if !set.Enabled {
		return
	}
	name := fmt.Sprintf("Employee #%d", sep.EmployeeID)
	if emp, err := e.q.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{ID: sep.EmployeeID, CompanyID: companyID}); err == nil {
		name = employeeName(emp)
	}
	run := Run{
		Trigger:     fmt.Sprintf("%s was terminated (%s, last day %s)", name, strings.ReplaceAll(sep.SeparationType, "_", " "), sep.LastWorkingDay.Format("Jan 2, 2006")),
		SubjectType: "employee", SubjectID: sep.EmployeeID, SubjectName: name,
	}
	run.Actions = fromSteps(steps, nil)
	for _, st := range steps {
		if st.Key == "final_pay" && st.Status == "done" {
			run.Actions = append(run.Actions, Action{Key: "approve_final_pay", Title: "Review and approve final pay", Status: ActPerson,
				Rationale: "Final pay is released only after approval (DOLE: within 30 days of separation).", Link: "/final-pay"})
		}
	}
	if sep.Status != "completed" {
		run.Actions = append(run.Actions, Action{Key: "final_pay", Title: "Compute final pay on the last working day", Status: ActWaiting,
			Rationale: "Final pay, device recall and access revocation run when the separation takes effect.", Link: "/final-pay"})
	}
	run.Actions = append(run.Actions, customSteps(set.Instructions, name)...)
	_, _ = e.Record(ctx, companyID, KeyOffboarding, set.Autonomy, run)
}

// CheckPayrollRuns runs the "Pre-payroll check" on calculated runs that have
// not been reviewed yet. It returns the number of runs checked.
func (e *Engine) CheckPayrollRuns(ctx context.Context) int {
	if e.Calculator == nil || !e.module("payroll") {
		return 0
	}
	runs, err := e.q.ListPayrollRunsToCheck(ctx)
	if err != nil {
		e.logger.Error("routines: payroll runs to check", "error", err)
		return 0
	}
	n := 0
	for _, r := range runs {
		set := e.Setting(ctx, r.CompanyID, KeyPrePayrollCheck)
		if !set.Enabled {
			continue
		}
		rep, err := e.Calculator.DetectAnomalies(ctx, r.ID, r.CompanyID)
		if err != nil {
			e.logger.Warn("routines: anomaly detection", "run_id", r.ID, "error", err)
			continue
		}
		_, _ = e.Record(ctx, r.CompanyID, KeyPrePayrollCheck, set.Autonomy, Run{
			Trigger:     fmt.Sprintf("Payroll calculated for %s (pay date %s)", r.CycleName, r.PayDate.Format("Jan 2")),
			SubjectType: "payroll_run", SubjectID: r.ID, SubjectName: r.CycleName,
			Actions: anomalyActions(rep, set.Instructions, r.CycleName),
		})
		n++
	}
	return n
}

func anomalyActions(rep *payroll.AnomalyReport, instructions, cycle string) []Action {
	acts := []Action{{Key: "compare", Title: "Compared " + plural(rep.TotalItems, "payslip", "payslips") + " with past runs", Status: ActDone,
		Rationale: "Checks gross/net variance, contributions, withholding, overtime, work days and duplicates."}}
	minor := 0
	for _, a := range rep.Anomalies {
		if a.Severity == payroll.SeverityCritical || a.Severity == payroll.SeverityHigh {
			who := a.EmployeeName
			if who == "" {
				who = "Payroll"
			}
			acts = append(acts, Action{Key: string(a.Type), Title: who + ": " + a.Description, Status: ActPerson,
				Rationale: fmt.Sprintf("%s severity — check before approving.", a.Severity), Link: "/payroll"})
		} else {
			minor++
		}
	}
	if minor > 0 {
		acts = append(acts, Action{Key: "minor", Title: plural(minor, "minor variance noted", "minor variances noted"), Status: ActDone,
			Rationale: "Within normal range; listed on the run's anomaly report.", Link: "/payroll"})
	}
	if len(acts) == 1 {
		acts = append(acts, Action{Key: "clean", Title: "No variances found — ready for approval", Status: ActDone, Link: "/payroll"})
	}
	return append(acts, customSteps(instructions, cycle)...)
}

// Policy is the filing autopilot's per-company autonomy (govfiling.SetPolicy).
func (e *Engine) Policy(ctx context.Context, companyID int64) string {
	set := e.Setting(ctx, companyID, KeyFilingAutopilot)
	if !set.Enabled {
		return govfiling.AutonomyOff
	}
	return set.Autonomy
}

// RecordAutopilot logs one "Filing autopilot" run per company that had
// something prepared, proposed or failed in this pass.
func (e *Engine) RecordAutopilot(ctx context.Context, res govfiling.AutopilotResult) {
	for companyID, act := range res.Companies {
		if len(act.Prepared)+len(act.Proposed)+len(act.Failed) == 0 {
			continue
		}
		var acts []Action
		for _, it := range act.Prepared {
			acts = append(acts, Action{Key: it.FormCode, Title: fmt.Sprintf("Prepared %s for %s", it.Name, it.Period), Status: ActDone,
				Rationale: "File generated from payroll and pre-flight checked.", Link: "/gov-filings"})
		}
		for _, it := range act.Proposed {
			acts = append(acts, Action{Key: it.FormCode, Title: fmt.Sprintf("Prepare %s for %s", it.Name, it.Period), Status: ActProposed,
				Rationale: "Due soon. Switch the routine to “Ask first” to have it prepared automatically.", Link: "/gov-filings"})
		}
		for _, it := range act.Failed {
			acts = append(acts, Action{Key: it.FormCode, Title: fmt.Sprintf("Could not prepare %s for %s", it.Name, it.Period), Status: ActFailed, Rationale: it.Error, Link: "/gov-filings"})
		}
		if len(act.Prepared) > 0 {
			acts = append(acts, Action{Key: "approve", Title: "Review and approve " + plural(len(act.Prepared), "return", "returns"), Status: ActPerson,
				Rationale: "Nothing is submitted to an agency without approval.", Link: "/gov-filings"})
		}
		set := e.Setting(ctx, companyID, KeyFilingAutopilot)
		_, _ = e.Record(ctx, companyID, KeyFilingAutopilot, set.Autonomy, Run{Trigger: "Scheduled filing check", Actions: acts})
	}
}
