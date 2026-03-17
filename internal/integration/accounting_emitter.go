package integration

import (
	"context"
	"fmt"
	"log/slog"

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
