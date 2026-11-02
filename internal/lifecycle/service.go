// Package lifecycle orchestrates the employee lifecycle as single actions,
// modelled on Warp's HR product: one "Hire" call creates the employee and
// fans out everything that has to happen (compensation, leave balances,
// custom fields, onboarding checklist, app/device provisioning, accounting
// sync); one "Terminate" call schedules the whole offboarding (clearance,
// offboarding checklist, separation record) and — on the last working day —
// flips the employee to separated and revokes access.
package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/clearance"
	"github.com/halaostory/halaos/internal/devices"
	"github.com/halaostory/halaos/internal/finalpay"
	"github.com/halaostory/halaos/internal/selfonboard"
	"github.com/halaostory/halaos/internal/store"
)

// AccountingEmitter emits employee events to the accounting outbox (optional).
type AccountingEmitter interface {
	EmitEmployeeUpserted(ctx context.Context, companyID, employeeID int64) error
	EmitEmployeeTerminated(ctx context.Context, companyID, employeeID int64, reason string) error
}

// Service implements the lifecycle orchestration.
type Service struct {
	queries    *store.Queries
	pool       *pgxpool.Pool
	logger     *slog.Logger
	accounting AccountingEmitter
	onboarding *selfonboard.Service
	devices    *devices.Service
	now        func() time.Time
}

// SetDevices wires device management so terminations lock and recall devices.
func (s *Service) SetDevices(d *devices.Service) { s.devices = d }

// SetOnboarding wires the self-onboarding portal / e-signature service (optional).
func (s *Service) SetOnboarding(o *selfonboard.Service) { s.onboarding = o }

// NewService builds a lifecycle service.
func NewService(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{queries: queries, pool: pool, logger: logger, now: time.Now}
}

// SetAccountingEmitter wires the accounting outbox (optional).
func (s *Service) SetAccountingEmitter(e AccountingEmitter) { s.accounting = e }

// Step describes one thing the orchestration did, returned to the UI so the
// admin can see exactly what happened ("Warp did 7 things for you").
type Step struct {
	Key    string `json:"key"`
	Status string `json:"status"` // done, skipped, scheduled
	Detail string `json:"detail,omitempty"`
}

// ErrConflict signals a uniqueness / state conflict (maps to HTTP 409).
var ErrConflict = errors.New("conflict")

// ErrNotFound signals a missing employee (maps to HTTP 404).
var ErrNotFound = errors.New("not found")

// ValidationError is a user-facing input problem (maps to HTTP 400).
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

func invalid(format string, a ...any) error { return &ValidationError{Msg: fmt.Sprintf(format, a...)} }

// ---------------------------------------------------------------------------
// Hire
// ---------------------------------------------------------------------------

// HireInput is the single payload for hiring someone.
type HireInput struct {
	EmployeeNo     string            `json:"employee_no"` // optional, auto-generated when empty
	FirstName      string            `json:"first_name" binding:"required"`
	LastName       string            `json:"last_name" binding:"required"`
	MiddleName     *string           `json:"middle_name"`
	DisplayName    *string           `json:"display_name"`
	Email          *string           `json:"email"`
	Phone          *string           `json:"phone"`
	BirthDate      *string           `json:"birth_date"`
	Gender         *string           `json:"gender"`
	Nationality    *string           `json:"nationality"`
	DepartmentID   *int64            `json:"department_id"`
	PositionID     *int64            `json:"position_id"`
	CostCenterID   *int64            `json:"cost_center_id"`
	ManagerID      *int64            `json:"manager_id"`
	HireDate       string            `json:"hire_date" binding:"required"`
	EmploymentType string            `json:"employment_type"`
	BasicSalary    *float64          `json:"basic_salary"`
	StructureID    *int64            `json:"structure_id"`
	CustomFields   map[string]string `json:"custom_fields"`
	// SkipOnboarding disables generating the onboarding checklist.
	SkipOnboarding bool `json:"skip_onboarding"`
	// SendInvite issues a self-onboarding link (new hire enters IDs, bank, etc.).
	SendInvite bool `json:"send_invite"`
	// OfferTemplateID sends this document template (e.g. offer letter) for e-signature.
	OfferTemplateID *int64 `json:"offer_template_id"`
	// EmailLinks e-mails the invite / signing links (requires SMTP); links are always returned.
	EmailLinks bool `json:"email_links"`
}

// HireResult is returned by Hire.
type HireResult struct {
	Employee     store.Employee `json:"employee"`
	Stage        string         `json:"stage"`
	Steps        []Step         `json:"steps"`
	InviteURL    string         `json:"invite_url,omitempty"`
	SignatureURL string         `json:"signature_url,omitempty"`
}

// Hire creates an employee and runs every downstream step in one transaction.
func (s *Service) Hire(ctx context.Context, companyID, userID int64, in HireInput) (*HireResult, error) {
	hireDate, err := time.Parse(dateLayout, in.HireDate)
	if err != nil {
		return nil, invalid("hire_date must be YYYY-MM-DD")
	}
	var birthDate pgtype.Date
	if in.BirthDate != nil && *in.BirthDate != "" {
		bd, err := time.Parse(dateLayout, *in.BirthDate)
		if err != nil {
			return nil, invalid("birth_date must be YYYY-MM-DD")
		}
		birthDate = pgtype.Date{Time: bd, Valid: true}
	}
	if in.BasicSalary != nil && *in.BasicSalary < 0 {
		return nil, invalid("basic_salary must be positive")
	}
	empType := in.EmploymentType
	if empType == "" {
		empType = "regular"
	}

	// Validate custom fields up front (before opening the tx).
	specs, fieldIDs, err := s.fieldSpecs(ctx, companyID)
	if err != nil {
		return nil, err
	}
	customValues, err := ValidateFieldValues(specs, in.CustomFields, true)
	if err != nil {
		return nil, invalid("%s", err.Error())
	}

	empNo := in.EmployeeNo
	if empNo == "" {
		next, err := s.queries.NextEmployeeNumber(ctx, companyID)
		if err != nil {
			return nil, fmt.Errorf("next employee number: %w", err)
		}
		empNo = FormatEmployeeNo(next)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	q := s.queries.WithTx(tx)

	var steps []Step

	emp, err := q.CreateEmployee(ctx, store.CreateEmployeeParams{
		CompanyID:      companyID,
		EmployeeNo:     empNo,
		FirstName:      in.FirstName,
		LastName:       in.LastName,
		MiddleName:     in.MiddleName,
		DisplayName:    in.DisplayName,
		Email:          in.Email,
		Phone:          in.Phone,
		BirthDate:      birthDate,
		Gender:         in.Gender,
		Nationality:    in.Nationality,
		DepartmentID:   in.DepartmentID,
		PositionID:     in.PositionID,
		CostCenterID:   in.CostCenterID,
		ManagerID:      in.ManagerID,
		HireDate:       hireDate,
		EmploymentType: empType,
	})
	if err != nil {
		s.logger.Error("lifecycle: create employee failed", "error", err)
		return nil, fmt.Errorf("%w: employee number %s already exists", ErrConflict, empNo)
	}
	steps = append(steps, Step{Key: "employee_created", Status: "done", Detail: empNo})

	if _, err := q.CreateEmploymentHistory(ctx, store.CreateEmploymentHistoryParams{
		CompanyID:      companyID,
		EmployeeID:     emp.ID,
		ActionType:     "hire",
		EffectiveDate:  hireDate,
		ToDepartmentID: in.DepartmentID,
		ToPositionID:   in.PositionID,
		CreatedBy:      &userID,
	}); err != nil {
		return nil, fmt.Errorf("employment history: %w", err)
	}

	// Compensation
	if in.BasicSalary != nil {
		var amt pgtype.Numeric
		if err := amt.Scan(fmt.Sprintf("%.2f", *in.BasicSalary)); err != nil {
			return nil, invalid("invalid basic_salary")
		}
		if _, err := q.CreateEmployeeSalary(ctx, store.CreateEmployeeSalaryParams{
			CompanyID:     companyID,
			EmployeeID:    emp.ID,
			StructureID:   in.StructureID,
			BasicSalary:   amt,
			EffectiveFrom: hireDate,
			CreatedBy:     &userID,
		}); err != nil {
			return nil, fmt.Errorf("assign salary: %w", err)
		}
		steps = append(steps, Step{Key: "compensation_set", Status: "done"})
	} else {
		steps = append(steps, Step{Key: "compensation_set", Status: "skipped", Detail: "no salary provided"})
	}

	// Leave balances (time off policies)
	n, err := initLeaveBalances(ctx, q, companyID, emp.ID, hireDate, s.now())
	if err != nil {
		return nil, err
	}
	steps = append(steps, Step{Key: "time_off_enrolled", Status: "done", Detail: fmt.Sprintf("%d leave types", n)})

	// Custom fields
	for k, v := range customValues {
		if v == "" {
			continue
		}
		if err := q.UpsertCustomFieldValue(ctx, store.UpsertCustomFieldValueParams{
			CompanyID:  companyID,
			EmployeeID: emp.ID,
			FieldID:    fieldIDs[k],
			Value:      v,
			UpdatedBy:  &userID,
		}); err != nil {
			return nil, fmt.Errorf("custom field %s: %w", k, err)
		}
	}
	if len(customValues) > 0 {
		steps = append(steps, Step{Key: "custom_fields_saved", Status: "done", Detail: fmt.Sprintf("%d fields", len(customValues))})
	}

	// Onboarding checklist
	var onboardingTasks int
	if in.SkipOnboarding {
		steps = append(steps, Step{Key: "onboarding_checklist", Status: "skipped"})
	} else {
		cnt, err := createChecklist(ctx, q, companyID, emp.ID, "onboarding", hireDate)
		if err != nil {
			return nil, err
		}
		onboardingTasks = cnt
		steps = append(steps, Step{Key: "onboarding_checklist", Status: "done", Detail: fmt.Sprintf("%d tasks", cnt)})
	}

	// Provisioning (apps / devices) via outbox → worker → Integration Hub
	if err := emitEvent(ctx, q, companyID, emp.ID, userID, "employee.hired", employeePayload(emp)); err != nil {
		return nil, err
	}
	steps = append(steps, Step{Key: "access_provisioning", Status: "scheduled", Detail: "employee.hired"})

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if s.accounting != nil {
		go func() {
			if err := s.accounting.EmitEmployeeUpserted(context.Background(), companyID, emp.ID); err != nil {
				s.logger.Error("lifecycle: accounting upsert failed", "employee_id", emp.ID, "error", err)
			}
		}()
		steps = append(steps, Step{Key: "accounting_sync", Status: "scheduled"})
	}

	res := &HireResult{Employee: emp, Steps: steps}
	s.afterHireDocuments(ctx, companyID, userID, emp.ID, in, res)

	res.Stage = ComputeStage(StageInput{
		Status:              emp.Status,
		HireDate:            hireDate,
		OnboardingRemaining: int64(onboardingTasks),
		Today:               s.now(),
	})
	return res, nil
}

// ---------------------------------------------------------------------------
// Terminate
// ---------------------------------------------------------------------------

// TerminateInput is the single payload for offboarding someone.
type TerminateInput struct {
	SeparationType string  `json:"separation_type" binding:"required"`
	Reason         *string `json:"reason"`
	NoticeDate     string  `json:"notice_date"` // defaults to today
	LastWorkingDay string  `json:"last_working_day" binding:"required"`
	RehireEligible *bool   `json:"rehire_eligible"` // default true
	RevokeAccess   *bool   `json:"revoke_access"`   // default true
	SkipClearance  bool    `json:"skip_clearance"`
	SkipChecklist  bool    `json:"skip_offboarding"`
}

// TerminateResult is returned by Terminate.
type TerminateResult struct {
	Separation store.EmployeeSeparation `json:"separation"`
	Effective  bool                     `json:"effective"`
	Stage      string                   `json:"stage"`
	Steps      []Step                   `json:"steps"`
}

// Terminate schedules (or immediately applies) an employee's separation.
func (s *Service) Terminate(ctx context.Context, companyID, userID, employeeID int64, in TerminateInput) (*TerminateResult, error) {
	if !ValidSeparationType(in.SeparationType) {
		return nil, invalid("invalid separation_type")
	}
	lwd, err := time.Parse(dateLayout, in.LastWorkingDay)
	if err != nil {
		return nil, invalid("last_working_day must be YYYY-MM-DD")
	}
	now := s.now()
	notice := dateOnly(now)
	if in.NoticeDate != "" {
		if notice, err = time.Parse(dateLayout, in.NoticeDate); err != nil {
			return nil, invalid("notice_date must be YYYY-MM-DD")
		}
	}
	if lwd.Before(notice) {
		return nil, invalid("last_working_day cannot be before notice_date")
	}
	rehire := in.RehireEligible == nil || *in.RehireEligible
	revoke := in.RevokeAccess == nil || *in.RevokeAccess

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	q := s.queries.WithTx(tx)

	emp, err := q.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{ID: employeeID, CompanyID: companyID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get employee: %w", err)
	}
	if emp.Status == "separated" || emp.Status == "terminated" {
		return nil, fmt.Errorf("%w: employee is already separated", ErrConflict)
	}
	if _, err := q.GetActiveSeparation(ctx, store.GetActiveSeparationParams{EmployeeID: employeeID, CompanyID: companyID}); err == nil {
		return nil, fmt.Errorf("%w: a separation is already scheduled for this employee", ErrConflict)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("check active separation: %w", err)
	}

	var steps []Step

	var clearanceID *int64
	if in.SkipClearance {
		steps = append(steps, Step{Key: "clearance", Status: "skipped"})
	} else {
		cr, err := q.CreateClearanceRequest(ctx, store.CreateClearanceRequestParams{
			CompanyID:       companyID,
			EmployeeID:      employeeID,
			ResignationDate: notice,
			LastWorkingDay:  lwd,
			Reason:          in.Reason,
			SubmittedBy:     userID,
		})
		if err != nil {
			return nil, fmt.Errorf("create clearance: %w", err)
		}
		clearanceID = &cr.ID
		cnt, err := createClearanceItems(ctx, q, companyID, cr.ID)
		if err != nil {
			return nil, err
		}
		steps = append(steps, Step{Key: "clearance", Status: "done", Detail: fmt.Sprintf("%d items", cnt)})
	}

	if in.SkipChecklist {
		steps = append(steps, Step{Key: "offboarding_checklist", Status: "skipped"})
	} else {
		cnt, err := createChecklist(ctx, q, companyID, employeeID, "offboarding", notice)
		if err != nil {
			return nil, err
		}
		steps = append(steps, Step{Key: "offboarding_checklist", Status: "done", Detail: fmt.Sprintf("%d tasks", cnt)})
	}

	sep, err := q.CreateEmployeeSeparation(ctx, store.CreateEmployeeSeparationParams{
		CompanyID:      companyID,
		EmployeeID:     employeeID,
		SeparationType: in.SeparationType,
		Reason:         in.Reason,
		NoticeDate:     notice,
		LastWorkingDay: lwd,
		RehireEligible: rehire,
		ClearanceID:    clearanceID,
		RevokeAccess:   revoke,
		CreatedBy:      &userID,
	})
	if err != nil {
		return nil, fmt.Errorf("create separation: %w", err)
	}

	effective := SeparationIsEffective(lwd, now)
	if effective {
		if err := effectuate(ctx, q, sep, emp, &userID); err != nil {
			return nil, err
		}
		steps = append(steps, Step{Key: "status_separated", Status: "done"})
		if revoke {
			steps = append(steps, Step{Key: "access_revocation", Status: "scheduled", Detail: "employee.terminated"})
		}

	} else {
		steps = append(steps, Step{Key: "status_separated", Status: "scheduled", Detail: in.LastWorkingDay})
		if revoke {
			steps = append(steps, Step{Key: "access_revocation", Status: "scheduled", Detail: in.LastWorkingDay})
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if effective {
		s.emitAccountingTerminated(companyID, sep)
		steps = append(steps, s.autoFinalPay(ctx, sep))
		if st, ok := s.deviceRecall(ctx, sep); ok {
			steps = append(steps, st)
		}
	}

	stage := StageOffboarding
	if effective {
		stage = StageSeparated
		sep.Status = "completed"
	}
	return &TerminateResult{Separation: sep, Effective: effective, Stage: stage, Steps: steps}, nil
}

// CancelSeparation cancels a scheduled (not yet effective) separation.
func (s *Service) CancelSeparation(ctx context.Context, companyID, employeeID int64) (*store.EmployeeSeparation, error) {
	sep, err := s.queries.GetActiveSeparation(ctx, store.GetActiveSeparationParams{EmployeeID: employeeID, CompanyID: companyID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	out, err := s.queries.CancelEmployeeSeparation(ctx, store.CancelEmployeeSeparationParams{ID: sep.ID, CompanyID: companyID})
	if err != nil {
		return nil, err
	}
	if _, err := s.queries.SkipOpenChecklistTasks(ctx, store.SkipOpenChecklistTasksParams{
		EmployeeID: employeeID, CompanyID: companyID, WorkflowType: "offboarding",
		Note: "Closed automatically: separation cancelled",
	}); err != nil {
		s.logger.Warn("lifecycle: failed to close offboarding checklist", "employee_id", employeeID, "error", err)
	}
	if sep.ClearanceID != nil {
		if _, err := s.queries.UpdateClearanceStatus(ctx, store.UpdateClearanceStatusParams{
			ID: *sep.ClearanceID, CompanyID: companyID, Status: "cancelled",
		}); err != nil {
			s.logger.Warn("lifecycle: failed to cancel clearance", "clearance_id", *sep.ClearanceID, "error", err)
		}
	}
	return &out, nil
}

// ProcessDueSeparations applies every scheduled separation whose last working
// day has arrived. Called periodically by the worker. Returns how many were applied.
func (s *Service) ProcessDueSeparations(ctx context.Context) int {
	due, err := s.queries.ListDueSeparations(ctx)
	if err != nil {
		s.logger.Error("lifecycle: list due separations", "error", err)
		return 0
	}
	applied := 0
	for _, sep := range due {
		if err := s.applyScheduled(ctx, sep); err != nil {
			s.logger.Error("lifecycle: apply separation failed", "separation_id", sep.ID, "error", err)
			continue
		}
		applied++
		s.emitAccountingTerminated(sep.CompanyID, sep)
		if st := s.autoFinalPay(ctx, sep); st.Status != "done" {
			s.logger.Warn("lifecycle: final pay not computed", "employee_id", sep.EmployeeID, "detail", st.Detail)
		}
		s.deviceRecall(ctx, sep)
	}
	return applied
}

func (s *Service) applyScheduled(ctx context.Context, sep store.EmployeeSeparation) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	q := s.queries.WithTx(tx)
	emp, err := q.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{ID: sep.EmployeeID, CompanyID: sep.CompanyID})
	if err != nil {
		return err
	}
	if err := effectuate(ctx, q, sep, emp, sep.CreatedBy); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) emitAccountingTerminated(companyID int64, sep store.EmployeeSeparation) {
	if s.accounting == nil {
		return
	}
	reason := sep.SeparationType
	if sep.Reason != nil && *sep.Reason != "" {
		reason = *sep.Reason
	}
	go func() {
		if err := s.accounting.EmitEmployeeTerminated(context.Background(), companyID, sep.EmployeeID, reason); err != nil {
			s.logger.Error("lifecycle: accounting terminated failed", "employee_id", sep.EmployeeID, "error", err)
		}
	}()
}

// effectuate marks the employee separated, records history and (optionally)
// enqueues the employee.terminated event that drives access revocation.
func effectuate(ctx context.Context, q *store.Queries, sep store.EmployeeSeparation, emp store.Employee, actor *int64) error {
	if _, err := q.MarkEmployeeSeparated(ctx, store.MarkEmployeeSeparatedParams{
		SeparationDate: pgtype.Date{Time: sep.LastWorkingDay, Valid: true},
		ID:             sep.EmployeeID,
		CompanyID:      sep.CompanyID,
	}); err != nil {
		return fmt.Errorf("mark separated: %w", err)
	}
	remarks := sep.SeparationType
	if sep.Reason != nil && *sep.Reason != "" {
		remarks = sep.SeparationType + ": " + *sep.Reason
	}
	if _, err := q.CreateEmploymentHistory(ctx, store.CreateEmploymentHistoryParams{
		CompanyID:     sep.CompanyID,
		EmployeeID:    sep.EmployeeID,
		ActionType:    "separated",
		EffectiveDate: sep.LastWorkingDay,
		Remarks:       &remarks,
		CreatedBy:     actor,
	}); err != nil {
		return fmt.Errorf("employment history: %w", err)
	}
	if err := q.CompleteEmployeeSeparation(ctx, sep.ID); err != nil {
		return fmt.Errorf("complete separation: %w", err)
	}
	if _, err := q.SkipOpenChecklistTasks(ctx, store.SkipOpenChecklistTasksParams{
		EmployeeID: sep.EmployeeID, CompanyID: sep.CompanyID, WorkflowType: "onboarding",
		Note: "Closed automatically: employee separated",
	}); err != nil {
		return fmt.Errorf("close onboarding checklist: %w", err)
	}
	if sep.RevokeAccess {
		payload := employeePayload(emp)
		payload["separation_type"] = sep.SeparationType
		payload["last_working_day"] = sep.LastWorkingDay.Format(dateLayout)
		var uid int64
		if actor != nil {
			uid = *actor
		}
		if err := emitEvent(ctx, q, sep.CompanyID, sep.EmployeeID, uid, "employee.terminated", payload); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// helpers shared by hire / terminate
// ---------------------------------------------------------------------------

func (s *Service) fieldSpecs(ctx context.Context, companyID int64) ([]FieldSpec, map[string]int64, error) {
	fields, err := s.queries.ListCustomFields(ctx, store.ListCustomFieldsParams{CompanyID: companyID, IncludeInactive: false})
	if err != nil {
		return nil, nil, fmt.Errorf("list custom fields: %w", err)
	}
	specs := make([]FieldSpec, 0, len(fields))
	ids := make(map[string]int64, len(fields))
	for _, f := range fields {
		specs = append(specs, FieldSpec{Key: f.FieldKey, Label: f.Label, Type: f.FieldType, Options: f.Options, IsRequired: f.IsRequired})
		ids[f.FieldKey] = f.ID
	}
	return specs, ids, nil
}

func initLeaveBalances(ctx context.Context, q *store.Queries, companyID, employeeID int64, hireDate, now time.Time) (int, error) {
	year := int32(hireDate.Year())
	if y := int32(now.Year()); y > year {
		year = y
	}
	leaveTypes, err := q.ListLeaveTypes(ctx, companyID)
	if err != nil {
		return 0, fmt.Errorf("list leave types: %w", err)
	}
	var zero pgtype.Numeric
	_ = zero.Scan("0")
	for _, lt := range leaveTypes {
		if _, err := q.UpsertLeaveBalance(ctx, store.UpsertLeaveBalanceParams{
			CompanyID:   companyID,
			EmployeeID:  employeeID,
			LeaveTypeID: lt.ID,
			Year:        year,
			Earned:      lt.DefaultDays,
			Carried:     zero,
		}); err != nil {
			return 0, fmt.Errorf("init leave balance: %w", err)
		}
	}
	return len(leaveTypes), nil
}

func createChecklist(ctx context.Context, q *store.Queries, companyID, employeeID int64, workflow string, ref time.Time) (int, error) {
	templates, err := q.ListOnboardingTemplates(ctx, companyID)
	if err != nil {
		return 0, fmt.Errorf("list %s templates: %w", workflow, err)
	}
	n := 0
	for _, t := range templates {
		if t.WorkflowType != workflow || (t.CompanyID != companyID && t.CompanyID != 0) {
			continue
		}
		due := pgtype.Date{}
		if t.DueDays != 0 {
			due = pgtype.Date{Time: ref.AddDate(0, 0, int(t.DueDays)), Valid: true}
		}
		tmplID := t.ID
		if _, err := q.CreateOnboardingTask(ctx, store.CreateOnboardingTaskParams{
			CompanyID:    companyID,
			EmployeeID:   employeeID,
			TemplateID:   &tmplID,
			WorkflowType: workflow,
			Title:        t.Title,
			Description:  t.Description,
			IsRequired:   t.IsRequired,
			AssigneeRole: t.AssigneeRole,
			DueDate:      due,
			SortOrder:    t.SortOrder,
		}); err != nil {
			return 0, fmt.Errorf("create %s task: %w", workflow, err)
		}
		n++
	}
	return n, nil
}

func createClearanceItems(ctx context.Context, q *store.Queries, companyID, clearanceID int64) (int, error) {
	templates, err := q.ListClearanceTemplates(ctx, companyID)
	if err != nil {
		return 0, fmt.Errorf("list clearance templates: %w", err)
	}
	type item struct{ dept, name string }
	var items []item
	for _, t := range templates {
		items = append(items, item{t.Department, t.ItemName})
	}
	if len(items) == 0 {
		for _, d := range clearance.DefaultItems() {
			items = append(items, item{d.Department, d.ItemName})
		}
	}
	for _, it := range items {
		if _, err := q.CreateClearanceItem(ctx, store.CreateClearanceItemParams{
			ClearanceID: clearanceID,
			Department:  it.dept,
			ItemName:    it.name,
		}); err != nil {
			return 0, fmt.Errorf("create clearance item: %w", err)
		}
	}
	return len(items), nil
}

func employeePayload(emp store.Employee) map[string]any {
	p := map[string]any{
		"first_name":      emp.FirstName,
		"last_name":       emp.LastName,
		"employee_no":     emp.EmployeeNo,
		"employment_type": emp.EmploymentType,
		"department_id":   emp.DepartmentID,
		"position_id":     emp.PositionID,
		"manager_id":      emp.ManagerID,
		"hire_date":       emp.HireDate.Format(dateLayout),
	}
	if emp.Email != nil {
		p["email"] = *emp.Email
	} else {
		p["email"] = ""
	}
	return p
}

func emitEvent(ctx context.Context, q *store.Queries, companyID, employeeID, userID int64, eventType string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%s:%d:%d", eventType, employeeID, time.Now().UnixNano())
	var actor *int64
	if userID != 0 {
		actor = &userID
	}
	if _, err := q.InsertHREvent(ctx, store.InsertHREventParams{
		CompanyID:      companyID,
		AggregateType:  "employee",
		AggregateID:    employeeID,
		EventType:      eventType,
		EventVersion:   1,
		Payload:        body,
		ActorUserID:    actor,
		IdempotencyKey: &key,
	}); err != nil {
		return fmt.Errorf("emit %s: %w", eventType, err)
	}
	return nil
}

// afterHireDocuments sends the offer letter for e-signature and the
// self-onboarding invite. Failures are reported as steps, not errors: the
// employee already exists and HR can resend from the profile.
func (s *Service) afterHireDocuments(ctx context.Context, companyID, userID, employeeID int64, in HireInput, res *HireResult) {
	if s.onboarding == nil {
		return
	}
	if in.OfferTemplateID != nil {
		out, err := s.onboarding.CreateRequest(ctx, companyID, userID, selfonboard.RequestInput{
			EmployeeID: employeeID, TemplateID: in.OfferTemplateID, SendEmail: in.EmailLinks,
		})
		if err != nil {
			res.Steps = append(res.Steps, Step{Key: "offer_letter_sent", Status: "skipped", Detail: err.Error()})
		} else {
			res.SignatureURL = out.Link.URL
			res.Steps = append(res.Steps, Step{Key: "offer_letter_sent", Status: "done", Detail: out.Request.Title})
		}
	}
	if in.SendInvite {
		email := ""
		if in.Email != nil {
			email = *in.Email
		}
		l, err := s.onboarding.CreateInvite(ctx, companyID, userID, employeeID, email, in.EmailLinks)
		if err != nil {
			res.Steps = append(res.Steps, Step{Key: "self_onboarding_invite", Status: "skipped", Detail: err.Error()})
			return
		}
		res.InviteURL = l.URL
		detail := "link created"
		if l.Emailed {
			detail = "e-mailed to " + email
		}
		res.Steps = append(res.Steps, Step{Key: "self_onboarding_invite", Status: "done", Detail: detail})
	}
}

// autoFinalPay computes the final pay for HR review once a separation is
// effective (Warp closes out final payroll as part of termination).
func (s *Service) autoFinalPay(ctx context.Context, sep store.EmployeeSeparation) Step {
	fp, res, err := finalpay.ComputeAndStore(ctx, s.queries, sep.CompanyID, sep.EmployeeID, sep.LastWorkingDay, sep.SeparationType)
	if err != nil {
		return Step{Key: "final_pay", Status: "skipped", Detail: err.Error()}
	}
	if fp == nil {
		return Step{Key: "final_pay", Status: "done", Detail: "already computed"}
	}
	return Step{Key: "final_pay", Status: "done", Detail: fmt.Sprintf("computed %.2f — awaiting approval", res.Total)}
}

// deviceRecall locks the leaver's devices and requests their return.
func (s *Service) deviceRecall(ctx context.Context, sep store.EmployeeSeparation) (Step, bool) {
	if s.devices == nil || !sep.RevokeAccess {
		return Step{}, false
	}
	n, err := s.devices.OnTermination(ctx, sep.CompanyID, sep.EmployeeID, sep.CreatedBy)
	if err != nil {
		s.logger.Error("lifecycle: device recall", "employee_id", sep.EmployeeID, "error", err)
		return Step{Key: "device_recall", Status: "skipped", Detail: err.Error()}, true
	}
	if n == 0 {
		return Step{}, false
	}
	return Step{Key: "device_recall", Status: "scheduled", Detail: fmt.Sprintf("%d device(s): lock + return", n)}, true
}
