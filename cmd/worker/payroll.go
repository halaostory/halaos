package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/notification"
	"github.com/tonypk/aigonhr/internal/payroll"
	"github.com/tonypk/aigonhr/internal/store"
)

// handlePayrollRunRequested runs payroll calculation and then performs automatic
// anomaly detection. If no critical anomalies are found, it updates the payroll
// run status to 'ready_for_approval'. Otherwise, it keeps 'completed' status.
// In both cases, HR admins are notified with the result.
func handlePayrollRunRequested(ctx context.Context, queries *store.Queries, calculator *payroll.Calculator, pool *pgxpool.Pool, ev store.HrEvent, logger *slog.Logger) error {
	runID := ev.AggregateID
	companyID := ev.CompanyID

	// Step 1: Run payroll calculation
	if err := calculator.RunPayroll(ctx, runID, companyID); err != nil {
		return err
	}

	// Step 2: Run anomaly detection as automatic pre-check
	logger.Info("running automatic anomaly pre-check", "run_id", runID, "company_id", companyID)

	report, err := calculator.DetectAnomalies(ctx, runID, companyID)
	if err != nil {
		// Anomaly detection failure should not fail the entire payroll run.
		// Log the error and proceed with default 'completed' status.
		logger.Error("anomaly detection failed, payroll remains completed",
			"run_id", runID,
			"error", err,
		)
		notifyAdminsPayrollResult(ctx, queries, logger, companyID, runID,
			"Payroll Completed",
			"Payroll computation completed. Anomaly pre-check could not be performed automatically. Please review manually.",
		)
		return nil
	}

	criticalCount := report.Summary.Critical
	totalAnomalies := len(report.Anomalies)

	logger.Info("anomaly pre-check completed",
		"run_id", runID,
		"total_anomalies", totalAnomalies,
		"critical", criticalCount,
		"high", report.Summary.High,
		"medium", report.Summary.Medium,
		"low", report.Summary.Low,
	)

	// Step 3: Update status and notify based on results
	if criticalCount == 0 {
		// No critical anomalies: promote to ready_for_approval
		_, err := pool.Exec(ctx,
			"UPDATE payroll_runs SET status = 'ready_for_approval' WHERE id = $1",
			runID,
		)
		if err != nil {
			logger.Error("failed to update payroll run to ready_for_approval",
				"run_id", runID,
				"error", err,
			)
			// Continue to notify even if status update fails
		} else {
			logger.Info("payroll run promoted to ready_for_approval", "run_id", runID)
		}

		msg := "Payroll ready for approval, no anomalies detected."
		if totalAnomalies > 0 {
			msg = fmt.Sprintf("Payroll ready for approval. %d non-critical anomaly(ies) noted — please review at your convenience.", totalAnomalies)
		}
		notifyAdminsPayrollResult(ctx, queries, logger, companyID, runID,
			"Payroll Ready for Approval",
			msg,
		)
	} else {
		// Critical anomalies found: keep 'completed' status, alert HR
		notifyAdminsPayrollResult(ctx, queries, logger, companyID, runID,
			"Payroll Requires Review",
			fmt.Sprintf("Payroll completed with %d anomaly(ies) (%d critical). Please review before approving.",
				totalAnomalies, criticalCount),
		)
	}

	return nil
}

// notifyAdminsPayrollResult sends a payroll notification to all admin users
// for the given company with an action to review the payroll page.
func notifyAdminsPayrollResult(ctx context.Context, queries *store.Queries, logger *slog.Logger, companyID, runID int64, title, msg string) {
	admins, err := queries.ListAdminUsersByCompany(ctx, companyID)
	if err != nil {
		logger.Error("failed to list admins for payroll notification",
			"company_id", companyID,
			"error", err,
		)
		return
	}

	entityType := "payroll_run"
	actions := []notification.NotificationAction{
		{Label: "Review Payroll", Route: "/payroll", Action: "review_payroll"},
	}

	for _, admin := range admins {
		notification.Notify(ctx, queries, logger, companyID, admin.ID,
			title, msg, "payroll", &entityType, &runID, actions,
		)
	}
}
