package ai

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/devices"
	"github.com/halaostory/halaos/internal/govfiling"
	"github.com/halaostory/halaos/internal/lifecycle"
	"github.com/halaostory/halaos/internal/selfonboard"
	"github.com/halaostory/halaos/internal/store"
)

// Warp-style lifecycle, compliance and device tools. Write tools go through
// action drafts (human confirmation). Approving a government submission is
// deliberately NOT exposed: it happens on the filing screen where the
// pre-flight review is shown.

func (r *ToolRegistry) lifecycleSvc() *lifecycle.Service {
	svc := lifecycle.NewService(r.queries, r.pool, slog.Default())
	svc.SetOnboarding(selfonboard.NewService(r.queries, slog.Default(), nil, nil, appBaseURL()))
	svc.SetDevices(devices.NewServiceFromKey(r.queries, slog.Default(), os.Getenv("INTEGRATION_ENCRYPTION_KEY")))
	return svc
}

func appBaseURL() string {
	if u := os.Getenv("APP_BASE_URL"); u != "" {
		return u
	}
	return "http://localhost:3001"
}

func strIn(input map[string]any, k string) string {
	v, _ := input[k].(string)
	return strings.TrimSpace(v)
}

func idIn(input map[string]any, k string) int64 {
	f, _ := input[k].(float64)
	return int64(f)
}

func (r *ToolRegistry) toolGetHRInbox(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}
	items, err := r.queries.ListHRInboxItems(ctx, store.ListHRInboxItemsParams{CompanyID: companyID, Lim: 200})
	if err != nil {
		return "", err
	}
	counts := map[string]int{}
	high := 0
	for _, it := range items {
		counts[it.Kind]++
		if it.Priority == "high" {
			high++
		}
	}
	top := items
	if len(top) > 15 {
		top = top[:15]
	}
	return toJSON(map[string]any{"total": len(items), "urgent": high, "counts_by_kind": counts, "top_items": top})
}

func (r *ToolRegistry) toolGetEmployeeLifecycle(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}
	id := idIn(input, "employee_id")
	emp, err := r.queries.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{ID: id, CompanyID: companyID})
	if err != nil {
		return "", fmt.Errorf("employee not found")
	}
	prog, _ := r.queries.GetOnboardingProgress(ctx, store.GetOnboardingProgressParams{EmployeeID: id, CompanyID: companyID})
	out := map[string]any{"employee_id": id, "name": emp.FirstName + " " + emp.LastName, "status": emp.Status, "checklists": prog}
	if sep, err := r.queries.GetLatestEmployeeSeparation(ctx, store.GetLatestEmployeeSeparationParams{EmployeeID: id, CompanyID: companyID}); err == nil {
		out["separation"] = map[string]any{"type": sep.SeparationType, "last_working_day": sep.LastWorkingDay.Format("2006-01-02"), "status": sep.Status}
	}
	if fp, err := r.queries.GetFinalPayStatusForEmployee(ctx, store.GetFinalPayStatusForEmployeeParams{EmployeeID: id, CompanyID: companyID}); err == nil {
		out["final_pay"] = fp
	}
	if inv, err := r.queries.GetLatestOnboardingInvite(ctx, store.GetLatestOnboardingInviteParams{EmployeeID: id, CompanyID: companyID}); err == nil {
		out["self_onboarding"] = inv.Status
	}
	devs, _ := r.queries.ListDevices(ctx, store.ListDevicesParams{CompanyID: companyID, EmployeeID: id})
	out["devices"] = len(devs)
	return toJSON(out)
}

func (r *ToolRegistry) toolHireEmployee(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}
	in := lifecycle.HireInput{
		FirstName: strIn(input, "first_name"), LastName: strIn(input, "last_name"), HireDate: strIn(input, "hire_date"),
		EmploymentType: strIn(input, "employment_type"), SendInvite: input["send_invite"] == true,
	}
	if in.FirstName == "" || in.LastName == "" || in.HireDate == "" {
		return "", fmt.Errorf("first_name, last_name and hire_date are required")
	}
	if e := strIn(input, "email"); e != "" {
		in.Email = &e
	}
	for k, dst := range map[string]**int64{"department_id": &in.DepartmentID, "position_id": &in.PositionID, "manager_id": &in.ManagerID, "offer_template_id": &in.OfferTemplateID} {
		if v := idIn(input, k); v > 0 {
			vv := v
			*dst = &vv
		}
	}
	if s, ok := input["basic_salary"].(float64); ok && s > 0 {
		in.BasicSalary = &s
	}
	if cf, ok := input["custom_fields"].(map[string]any); ok {
		in.CustomFields = map[string]string{}
		for k, v := range cf {
			in.CustomFields[k] = fmt.Sprint(v)
		}
	}
	res, err := r.lifecycleSvc().Hire(ctx, companyID, userID, in)
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{"success": true, "employee_id": res.Employee.ID, "employee_no": res.Employee.EmployeeNo,
		"stage": res.Stage, "steps": res.Steps, "invite_url": res.InviteURL, "signature_url": res.SignatureURL})
}

func (r *ToolRegistry) toolTerminateEmployee(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}
	in := lifecycle.TerminateInput{SeparationType: strIn(input, "separation_type"), LastWorkingDay: strIn(input, "last_working_day"), NoticeDate: strIn(input, "notice_date")}
	if reason := strIn(input, "reason"); reason != "" {
		in.Reason = &reason
	}
	if v, ok := input["rehire_eligible"].(bool); ok {
		in.RehireEligible = &v
	}
	res, err := r.lifecycleSvc().Terminate(ctx, companyID, userID, idIn(input, "employee_id"), in)
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{"success": true, "effective_now": res.Effective, "stage": res.Stage, "steps": res.Steps})
}

func (r *ToolRegistry) toolSendOnboardingInvite(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}
	svc := selfonboard.NewService(r.queries, slog.Default(), nil, nil, appBaseURL())
	l, err := svc.CreateInvite(ctx, companyID, userID, idIn(input, "employee_id"), strIn(input, "email"), false)
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{"success": true, "url": l.URL, "expires_at": l.ExpiresAt, "note": "Share this link with the new hire; it is shown only once."})
}

func (r *ToolRegistry) toolSendDocumentForSignature(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}
	req := selfonboard.RequestInput{EmployeeID: idIn(input, "employee_id"), Title: strIn(input, "title"), Body: strIn(input, "body")}
	if t := idIn(input, "template_id"); t > 0 {
		req.TemplateID = &t
	}
	svc := selfonboard.NewService(r.queries, slog.Default(), nil, nil, appBaseURL())
	out, err := svc.CreateRequest(ctx, companyID, userID, req)
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{"success": true, "request_id": out.Request.ID, "title": out.Request.Title, "url": out.Link.URL})
}

func (r *ToolRegistry) govSvc() *govfiling.Service {
	return govfiling.NewService(r.queries, r.pool, slog.Default(), govfiling.Config{FileDir: os.Getenv("GOV_FILES_DIR")})
}

func (r *ToolRegistry) toolListGovFilings(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}
	subs, err := r.queries.ListGovSubmissions(ctx, store.ListGovSubmissionsParams{CompanyID: companyID, Status: strIn(input, "status"), Limit: 30})
	if err != nil {
		return "", err
	}
	type row struct {
		ID        int64  `json:"id"`
		Form      string `json:"form"`
		Period    string `json:"period"`
		Channel   string `json:"channel"`
		Status    string `json:"status"`
		Reference string `json:"reference_no,omitempty"`
		Error     string `json:"error,omitempty"`
	}
	out := []row{}
	for _, s := range subs {
		rr := row{ID: s.ID, Form: s.Agency + " " + s.FormCode, Period: s.Period, Channel: s.Channel, Status: s.Status}
		if s.ReferenceNo != nil {
			rr.Reference = *s.ReferenceNo
		}
		if s.ErrorMessage != nil {
			rr.Error = *s.ErrorMessage
		}
		out = append(out, rr)
	}
	upcoming, _ := r.queries.ListUpcomingFilings(ctx, companyID)
	comp, _ := r.queries.GetCompanyByID(ctx, companyID)
	forms := []string{}
	for _, f := range govfiling.Forms(comp.Country) {
		forms = append(forms, f.Code+" ("+f.Name+")")
	}
	return toJSON(map[string]any{"submissions": out, "upcoming_due": upcoming, "supported_forms": forms})
}

func (r *ToolRegistry) toolPrepareGovFiling(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}
	p, err := r.govSvc().Prepare(ctx, companyID, &userID, govfiling.PrepareInput{FormCode: strIn(input, "form_code"), Period: strIn(input, "period"), Channel: strIn(input, "channel")})
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success": true, "submission_id": p.Submission.ID, "status": p.Submission.Status, "channel": p.Submission.Channel,
		"blocking": p.Preflight.Blocking, "issues": p.Preflight.Issues, "ai_summary": p.Preflight.AISummary,
		"next_step": "Review and approve it on the Government Filings page; HalaOS submits only after that approval.",
	})
}

func (r *ToolRegistry) toolLockDevice(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}
	action := strIn(input, "action")
	if action == "" {
		action = "lock"
	}
	if action != "lock" && action != "return_request" {
		return "", fmt.Errorf("the assistant can only lock devices or request their return; wipes must be requested on the Devices page")
	}
	svc := devices.NewServiceFromKey(r.queries, slog.Default(), os.Getenv("INTEGRATION_ENCRYPTION_KEY"))
	a, err := svc.RequestAction(ctx, companyID, idIn(input, "device_id"), action, "ai", &userID, false)
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{"success": true, "action_id": a.ID, "status": a.Status})
}

func lifecycleDefs() []provider.ToolDefinition {
	obj := func(props map[string]any, req ...string) map[string]any {
		return jsonSchema(map[string]any{"type": "object", "properties": props, "required": req})
	}
	str := func(d string) map[string]any { return map[string]any{"type": "string", "description": d} }
	num := func(d string) map[string]any { return map[string]any{"type": "integer", "description": d} }
	return []provider.ToolDefinition{
		{Name: "get_hr_inbox", Description: "Everything waiting on HR right now across modules (approvals, onboarding/offboarding tasks, expiring documents, compliance alerts, government filings needing approval or a CAPTCHA/OTP, device actions, final pay to review). Use it to answer 'what needs my attention?'. Admin only.",
			Parameters: obj(map[string]any{})},
		{Name: "get_employee_lifecycle", Description: "Lifecycle status of one employee: onboarding/offboarding checklist progress, self-onboarding portal status, separation, final pay and devices.",
			Parameters: obj(map[string]any{"employee_id": num("Employee ID")}, "employee_id")},
		{Name: "hire_employee", Description: "Hire someone in one step: creates the employee, salary, leave balances, onboarding checklist, app provisioning and accounting sync; optionally sends the self-onboarding link and an offer letter for e-signature. Requires user confirmation.",
			Parameters: obj(map[string]any{
				"first_name": str("First name"), "last_name": str("Last name"), "email": str("Work or personal e-mail"),
				"hire_date": str("Start date YYYY-MM-DD"), "employment_type": str("regular, probationary, contractual, part_time"),
				"department_id": num("Department ID"), "position_id": num("Position ID"), "manager_id": num("Manager employee ID"),
				"basic_salary":      map[string]any{"type": "number", "description": "Monthly basic salary"},
				"custom_fields":     map[string]any{"type": "object", "description": "Company custom field values keyed by field key"},
				"send_invite":       map[string]any{"type": "boolean", "description": "Create a self-onboarding link for the new hire"},
				"offer_template_id": num("Document template to send for e-signature (e.g. offer letter)"),
			}, "first_name", "last_name", "hire_date")},
		{Name: "terminate_employee", Description: "Offboard an employee in one step: clearance + offboarding checklist now; on the last working day the employee is separated, app access revoked, devices locked and recalled, and final pay computed for review. Requires user confirmation.",
			Parameters: obj(map[string]any{
				"employee_id":      num("Employee ID"),
				"separation_type":  map[string]any{"type": "string", "enum": []string{"resignation", "termination", "end_of_contract", "retirement", "layoff", "death", "other"}},
				"last_working_day": str("YYYY-MM-DD"), "notice_date": str("YYYY-MM-DD (default today)"), "reason": str("Reason"),
				"rehire_eligible": map[string]any{"type": "boolean"},
			}, "employee_id", "separation_type", "last_working_day")},
		{Name: "send_onboarding_invite", Description: "Create a self-onboarding link where a new hire enters personal details, government IDs, bank account and signs documents.",
			Parameters: obj(map[string]any{"employee_id": num("Employee ID"), "email": str("Override e-mail")}, "employee_id")},
		{Name: "send_document_for_signature", Description: "Send a document (from a template, or title + body text) to an employee for electronic signature. Returns the signing link.",
			Parameters: obj(map[string]any{"employee_id": num("Employee ID"), "template_id": num("Document template ID"), "title": str("Title if no template"), "body": str("Body text if no template")}, "employee_id")},
		{Name: "list_gov_filings", Description: "Government filings: prepared/submitted returns with status and reference numbers, upcoming due dates, and the forms HalaOS can file for this company.",
			Parameters: obj(map[string]any{"status": str("Optional status filter: draft, ready, approved, running, needs_human, submitted, failed")})},
		{Name: "prepare_gov_filing", Description: "Prepare a statutory return (e.g. bir_1601c, sss_r3, philhealth_rf1, pagibig_mcrf, bir_1604c, irs_941) for a period: builds the file from payroll and runs the pre-flight review. Submission still needs approval on the Government Filings page.",
			Parameters: obj(map[string]any{"form_code": str("Form code"), "period": str("YYYY-MM (monthly), YYYY-Qn (quarterly) or YYYY (annual)"), "channel": str("Optional: browser, email, api, manual")}, "form_code", "period")},
		{Name: "lock_device", Description: "Lock a company device through MDM, or request its return. Wipes are not available to the assistant.",
			Parameters: obj(map[string]any{"device_id": num("Device ID"), "action": map[string]any{"type": "string", "enum": []string{"lock", "return_request"}}}, "device_id")},
	}
}

func (r *ToolRegistry) registerLifecycleTools() {
	r.tools["get_hr_inbox"] = r.toolGetHRInbox
	r.tools["get_employee_lifecycle"] = r.toolGetEmployeeLifecycle
	r.tools["hire_employee"] = r.toolHireEmployee
	r.tools["terminate_employee"] = r.toolTerminateEmployee
	r.tools["send_onboarding_invite"] = r.toolSendOnboardingInvite
	r.tools["send_document_for_signature"] = r.toolSendDocumentForSignature
	r.tools["list_gov_filings"] = r.toolListGovFilings
	r.tools["prepare_gov_filing"] = r.toolPrepareGovFiling
	r.tools["lock_device"] = r.toolLockDevice
}
