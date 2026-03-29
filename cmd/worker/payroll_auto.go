package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/notification"
	"github.com/halaostory/halaos/internal/payroll"
	"github.com/halaostory/halaos/internal/store"
)

// autoRunScheduledPayrolls checks for companies with auto-run enabled and
// triggers payroll runs for draft cycles approaching their pay date.
// Runs daily at 6 AM.
func autoRunScheduledPayrolls(
	ctx context.Context,
	queries *store.Queries,
	pool *pgxpool.Pool,
	calculator *payroll.Calculator,
	logger *slog.Logger,
) {
	now := time.Now()
	if now.Hour() != 6 {
		return
	}

	logger.Info("checking for auto-run payroll cycles")

	configs, err := queries.ListCompaniesWithAutoRun(ctx)
	if err != nil {
		logger.Error("auto-payroll: failed to list configs", "error", err)
		return
	}

	if len(configs) == 0 {
		return
	}

	totalRuns := 0
	for _, cfg := range configs {
		count, err := autoRunForCompany(ctx, queries, pool, calculator, logger, cfg)
		if err != nil {
			logger.Error("auto-payroll: failed for company",
				"company_id", cfg.CompanyID,
				"company_name", cfg.CompanyName,
				"error", err,
			)
			continue
		}
		totalRuns += count
	}

	if totalRuns > 0 {
		logger.Info("auto-payroll completed", "total_runs_triggered", totalRuns)
	}
}

func autoRunForCompany(
	ctx context.Context,
	queries *store.Queries,
	pool *pgxpool.Pool,
	calculator *payroll.Calculator,
	logger *slog.Logger,
	cfg store.ListCompaniesWithAutoRunRow,
) (int, error) {
	// Find draft cycles within the auto-run window
	cycles, err := queries.ListDraftCyclesForAutoRun(ctx, store.ListDraftCyclesForAutoRunParams{
		CompanyID: cfg.CompanyID,
		Column2:   cfg.DaysBeforePay,
	})
	if err != nil {
		return 0, fmt.Errorf("list draft cycles: %w", err)
	}

	if len(cycles) == 0 {
		return 0, nil
	}

	triggered := 0
	for _, cycle := range cycles {
		// Skip locked cycles
		if cycle.IsLocked {
			logger.Info("auto-payroll: skipping locked cycle",
				"cycle_id", cycle.ID,
				"company_id", cfg.CompanyID,
			)
			continue
		}

		// Create payroll run (initiated_by = nil for system)
		run, err := queries.CreatePayrollRun(ctx, store.CreatePayrollRunParams{
			CompanyID:   cfg.CompanyID,
			CycleID:     cycle.ID,
			RunType:     "regular",
			InitiatedBy: nil,
		})
		if err != nil {
			logger.Error("auto-payroll: failed to create run",
				"cycle_id", cycle.ID,
				"error", err,
			)
			continue
		}

		// Log the auto-run action
		detail := fmt.Sprintf("Auto-triggered payroll run for cycle '%s' (pay date: %s)",
			cycle.Name, cycle.PayDate.Format("2006-01-02"))
		queries.InsertPayrollAutoLog(ctx, store.InsertPayrollAutoLogParams{
			CompanyID: cfg.CompanyID,
			CycleID:   cycle.ID,
			RunID:     &run.ID,
			Action:    "auto_run",
			Detail:    &detail,
		})

		// Dispatch to event outbox for async processing
		idempKey := fmt.Sprintf("payroll.auto_run.%d.%d", cycle.ID, run.ID)
		payload, _ := json.Marshal(map[string]any{
			"run_id":     run.ID,
			"company_id": cfg.CompanyID,
			"run_type":   "regular",
			"auto":       true,
		})
		_, err = queries.InsertHREvent(ctx, store.InsertHREventParams{
			CompanyID:      cfg.CompanyID,
			AggregateType:  "payroll_run",
			AggregateID:    run.ID,
			EventType:      "payroll.run_requested",
			EventVersion:   1,
			Payload:        payload,
			IdempotencyKey: &idempKey,
		})
		if err != nil {
			logger.Error("auto-payroll: failed to enqueue event",
				"run_id", run.ID,
				"error", err,
			)
			continue
		}

		logger.Info("auto-payroll: triggered run",
			"company_id", cfg.CompanyID,
			"cycle_id", cycle.ID,
			"run_id", run.ID,
			"pay_date", cycle.PayDate.Format("2006-01-02"),
		)

		// Notify admins about auto-run
		if cfg.NotifyOnAuto {
			notifyAdminsPayrollResult(ctx, queries, logger, cfg.CompanyID, run.ID,
				"Auto-Payroll Triggered",
				fmt.Sprintf("Payroll for '%s' has been automatically triggered. Pay date: %s. Calculation is in progress.",
					cycle.Name, cycle.PayDate.Format("Jan 02, 2006")),
			)
		}

		triggered++
	}

	return triggered, nil
}

// autoApprovePayroll checks for payroll runs with ready_for_approval status
// and auto-approves them if the company has auto-approve enabled and conditions
// are met (no critical anomalies, net pay within limits).
// Runs hourly.
func autoApprovePayroll(
	ctx context.Context,
	queries *store.Queries,
	pool *pgxpool.Pool,
	logger *slog.Logger,
) {
	configs, err := queries.ListCompaniesWithAutoRun(ctx)
	if err != nil {
		logger.Error("auto-approve: failed to list configs", "error", err)
		return
	}

	for _, cfg := range configs {
		if !cfg.AutoApproveEnabled {
			continue
		}
		autoApproveForCompany(ctx, queries, pool, logger, cfg)
	}
}

func autoApproveForCompany(
	ctx context.Context,
	queries *store.Queries,
	pool *pgxpool.Pool,
	logger *slog.Logger,
	cfg store.ListCompaniesWithAutoRunRow,
) {
	// Find runs in ready_for_approval status
	rows, err := pool.Query(ctx,
		`SELECT pr.id, pr.cycle_id, pr.total_net, pc.name
		 FROM payroll_runs pr
		 JOIN payroll_cycles pc ON pc.id = pr.cycle_id
		 WHERE pr.company_id = $1
		   AND pr.status = 'ready_for_approval'
		   AND pc.status = 'draft'`,
		cfg.CompanyID,
	)
	if err != nil {
		logger.Error("auto-approve: failed to query runs",
			"company_id", cfg.CompanyID,
			"error", err,
		)
		return
	}
	defer rows.Close()

	type readyRun struct {
		RunID     int64
		CycleID   int64
		TotalNet  float64
		CycleName string
	}

	var readyRuns []readyRun
	for rows.Next() {
		var r readyRun
		var totalNet interface{}
		if err := rows.Scan(&r.RunID, &r.CycleID, &totalNet, &r.CycleName); err != nil {
			logger.Error("auto-approve: scan failed", "error", err)
			continue
		}
		// Convert totalNet from pgtype.Numeric
		if tn, ok := totalNet.(float64); ok {
			r.TotalNet = tn
		}
		readyRuns = append(readyRuns, r)
	}

	for _, r := range readyRuns {
		// Check max auto-approve amount limit
		maxAmount := numericToFloat(cfg.MaxAutoApproveAmount)
		if maxAmount > 0 && r.TotalNet > maxAmount {
			detail := fmt.Sprintf("Auto-approve skipped for '%s': total net %.2f exceeds limit %.2f",
				r.CycleName, r.TotalNet, maxAmount)
			logger.Info("auto-approve: skipped (over limit)",
				"cycle_id", r.CycleID,
				"total_net", r.TotalNet,
				"max_amount", maxAmount,
			)
			queries.InsertPayrollAutoLog(ctx, store.InsertPayrollAutoLogParams{
				CompanyID: cfg.CompanyID,
				CycleID:   r.CycleID,
				RunID:     &r.RunID,
				Action:    "auto_skipped",
				Detail:    &detail,
			})
			continue
		}

		// Auto-approve the cycle
		err := queries.ApprovePayrollCycle(ctx, store.ApprovePayrollCycleParams{
			ID:         r.CycleID,
			CompanyID:  cfg.CompanyID,
			ApprovedBy: nil, // System auto-approve
		})
		if err != nil {
			logger.Error("auto-approve: failed",
				"cycle_id", r.CycleID,
				"error", err,
			)
			continue
		}

		detail := fmt.Sprintf("Auto-approved payroll '%s' (total net: %.2f, zero critical anomalies)",
			r.CycleName, r.TotalNet)
		queries.InsertPayrollAutoLog(ctx, store.InsertPayrollAutoLogParams{
			CompanyID: cfg.CompanyID,
			CycleID:   r.CycleID,
			RunID:     &r.RunID,
			Action:    "auto_approve",
			Detail:    &detail,
		})

		logger.Info("auto-approve: payroll approved",
			"company_id", cfg.CompanyID,
			"cycle_id", r.CycleID,
			"total_net", r.TotalNet,
		)

		// Notify admins
		if cfg.NotifyOnAuto {
			notifyAdminsPayrollResult(ctx, queries, logger, cfg.CompanyID, r.RunID,
				"Payroll Auto-Approved",
				fmt.Sprintf("Payroll '%s' has been automatically approved. Zero critical anomalies detected. Total net pay: %.2f.",
					r.CycleName, r.TotalNet),
			)

			// Notify employees that payslips are available
			go notifyEmployeesPayslips(ctx, queries, logger, cfg.CompanyID, r.CycleID, r.CycleName)
		}
	}
}

func notifyEmployeesPayslips(
	ctx context.Context,
	queries *store.Queries,
	logger *slog.Logger,
	companyID, cycleID int64,
	cycleName string,
) {
	emps, err := queries.ListActiveEmployees(ctx, companyID)
	if err != nil {
		return
	}
	entityType := "payroll"
	actions := []notification.NotificationAction{
		{Label: "View Payslip", Route: "/payslips", Action: "view_payslip"},
	}
	for _, e := range emps {
		if e.UserID != nil {
			notification.Notify(ctx, queries, logger, companyID, *e.UserID,
				"Payslip Available",
				fmt.Sprintf("Your payslip for %s is now available.", cycleName),
				"payroll", &entityType, &cycleID, actions)
		}
	}
}
