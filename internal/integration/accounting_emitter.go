package integration

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/tonypk/aigonhr/internal/store"
)

// AccountingEmitter builds payroll events and enqueues them for delivery.
type AccountingEmitter struct {
	outbox  *AccountingOutbox
	builder *PayrollEventBuilder
	queries *store.Queries
	logger  *slog.Logger
}

// NewAccountingEmitter creates a new emitter.
func NewAccountingEmitter(queries *store.Queries, logger *slog.Logger) *AccountingEmitter {
	return &AccountingEmitter{
		outbox:  NewAccountingOutbox(queries, logger),
		builder: NewPayrollEventBuilder(queries),
		queries: queries,
		logger:  logger,
	}
}

// EmitPayrollApproved builds and enqueues the payroll.run.completed event.
func (e *AccountingEmitter) EmitPayrollApproved(ctx context.Context, companyID, cycleID int64) error {
	// Check if accounting link exists
	_, err := e.queries.GetActiveAccountingLink(ctx, companyID)
	if err != nil {
		// No accounting link configured — skip silently
		return nil
	}

	// Find the latest completed run for this cycle
	runID, err := e.queries.GetLatestCompletedRunForCycle(ctx, store.GetLatestCompletedRunForCycleParams{
		CycleID:   cycleID,
		CompanyID: companyID,
	})
	if err != nil {
		return fmt.Errorf("no completed run for cycle %d: %w", cycleID, err)
	}

	event, err := e.builder.BuildPayrollRunCompleted(ctx, companyID, cycleID, runID)
	if err != nil {
		return fmt.Errorf("build event: %w", err)
	}

	if err := e.outbox.Enqueue(ctx, companyID, EventPayrollRunCompleted, "payroll_run", runID, event); err != nil {
		return fmt.Errorf("enqueue event: %w", err)
	}

	e.logger.Info("payroll accounting event emitted",
		"company_id", companyID,
		"cycle_id", cycleID,
		"run_id", runID,
		"head_count", event.Totals.HeadCount,
	)
	return nil
}

// EmitEmployeeUpserted enqueues an employee.upserted event for accounting sync.
func (e *AccountingEmitter) EmitEmployeeUpserted(ctx context.Context, companyID, employeeID int64) error {
	_, err := e.queries.GetActiveAccountingLink(ctx, companyID)
	if err != nil {
		return nil // no accounting link — skip
	}

	emp, err := e.queries.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{
		ID: employeeID, CompanyID: companyID,
	})
	if err != nil {
		return fmt.Errorf("get employee %d: %w", employeeID, err)
	}

	company, err := e.queries.GetCompanyByID(ctx, companyID)
	if err != nil {
		return fmt.Errorf("get company %d: %w", companyID, err)
	}

	jurisdiction := company.Country
	if jurisdiction == "" {
		jurisdiction = "PH"
	}

	var deptName, posTitle string
	if emp.DepartmentID != nil {
		if dept, err := e.queries.GetDepartmentByID(ctx, store.GetDepartmentByIDParams{
			ID: *emp.DepartmentID, CompanyID: companyID,
		}); err == nil {
			deptName = dept.Name
		}
	}
	if emp.PositionID != nil {
		if pos, err := e.queries.GetPositionByID(ctx, store.GetPositionByIDParams{
			ID: *emp.PositionID, CompanyID: companyID,
		}); err == nil {
			posTitle = pos.Title
		}
	}

	// Get profile for TIN/SSS/PhilHealth/PagIBIG
	var tin, sss, philhealth, pagibig string
	profile, err := e.queries.GetEmployeeProfile(ctx, store.GetEmployeeProfileParams{
		EmployeeID: employeeID, CompanyID: companyID,
	})
	if err == nil {
		tin = derefStr(profile.Tin)
		sss = derefStr(profile.SssNo)
		philhealth = derefStr(profile.PhilhealthNo)
		pagibig = derefStr(profile.PagibigNo)
	}

	deptID := int64(0)
	if emp.DepartmentID != nil {
		deptID = *emp.DepartmentID
	}

	event := EmployeeUpsertedEvent{
		EventID:        uuid.New().String(),
		EventType:      EventEmployeeUpserted,
		EventVersion:   1,
		OccurredAt:     time.Now().UTC(),
		HRCompanyID:    companyID,
		Jurisdiction:   jurisdiction,
		EmployeeID:     employeeID,
		EmployeeNo:     emp.EmployeeNo,
		FirstName:      emp.FirstName,
		LastName:       emp.LastName,
		Email:          derefStr(emp.Email),
		TIN:            tin,
		SSS:            sss,
		PhilHealth:     philhealth,
		PagIBIG:        pagibig,
		DepartmentID:   deptID,
		DepartmentName: deptName,
		PositionTitle:  posTitle,
		Status:         emp.Status,
	}

	if err := e.outbox.Enqueue(ctx, companyID, EventEmployeeUpserted, "employee", employeeID, event); err != nil {
		return fmt.Errorf("enqueue employee.upserted: %w", err)
	}

	e.logger.Info("employee.upserted event emitted",
		"company_id", companyID,
		"employee_id", employeeID,
	)
	return nil
}

// EmitEmployeeTerminated enqueues an employee.terminated event.
func (e *AccountingEmitter) EmitEmployeeTerminated(ctx context.Context, companyID, employeeID int64, reason string) error {
	_, err := e.queries.GetActiveAccountingLink(ctx, companyID)
	if err != nil {
		return nil
	}

	emp, err := e.queries.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{
		ID: employeeID, CompanyID: companyID,
	})
	if err != nil {
		return fmt.Errorf("get employee %d: %w", employeeID, err)
	}

	termDate := time.Now().Format("2006-01-02")
	if emp.SeparationDate.Valid {
		termDate = emp.SeparationDate.Time.Format("2006-01-02")
	}

	event := EmployeeTerminatedEvent{
		EventID:         uuid.New().String(),
		EventType:       EventEmployeeTerminated,
		EventVersion:    1,
		OccurredAt:      time.Now().UTC(),
		HRCompanyID:     companyID,
		EmployeeID:      employeeID,
		EmployeeNo:      emp.EmployeeNo,
		TerminationDate: termDate,
		Reason:          reason,
	}

	if err := e.outbox.Enqueue(ctx, companyID, EventEmployeeTerminated, "employee", employeeID, event); err != nil {
		return fmt.Errorf("enqueue employee.terminated: %w", err)
	}

	e.logger.Info("employee.terminated event emitted",
		"company_id", companyID,
		"employee_id", employeeID,
	)
	return nil
}

// EmitPayrollReversed enqueues a payroll.run.reversed event.
func (e *AccountingEmitter) EmitPayrollReversed(ctx context.Context, companyID, payrollRunID int64, originalPayDate, reason string) error {
	_, err := e.queries.GetActiveAccountingLink(ctx, companyID)
	if err != nil {
		return nil
	}

	event := PayrollRunReversedEvent{
		EventID:         uuid.New().String(),
		EventType:       EventPayrollRunReversed,
		EventVersion:    1,
		OccurredAt:      time.Now().UTC(),
		HRCompanyID:     companyID,
		PayrollRunID:    payrollRunID,
		OriginalPayDate: originalPayDate,
		ReversalReason:  reason,
	}

	if err := e.outbox.Enqueue(ctx, companyID, EventPayrollRunReversed, "payroll_run", payrollRunID, event); err != nil {
		return fmt.Errorf("enqueue payroll.run.reversed: %w", err)
	}

	e.logger.Info("payroll.run.reversed event emitted",
		"company_id", companyID,
		"payroll_run_id", payrollRunID,
	)
	return nil
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
