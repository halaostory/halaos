package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/tonypk/aigonhr/internal/config"
	"github.com/tonypk/aigonhr/internal/payroll"
	"github.com/tonypk/aigonhr/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.Postgres.DSN())
	if err != nil {
		logger.Error("failed to create db pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
	})
	defer rdb.Close()

	queries := store.New(pool)
	calculator := payroll.NewCalculator(queries, pool, logger)

	logger.Info("worker started")

	// Event outbox processor + agent task executor
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				processEvents(ctx, queries, calculator, pool, logger)
				processAgentTasks(ctx, cfg, queries, pool, logger)
			}
		}
	}()

	// Periodic jobs
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runPeriodicJobs(ctx, queries, pool, rdb, logger)
			}
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	logger.Info("received signal, shutting down", "signal", sig.String())
	cancel()
	time.Sleep(2 * time.Second)
	logger.Info("worker stopped")
}

func processEvents(ctx context.Context, queries *store.Queries, calculator *payroll.Calculator, pool *pgxpool.Pool, logger *slog.Logger) {
	events, err := queries.GetPendingEvents(ctx, 50)
	if err != nil {
		logger.Error("failed to get pending events", "error", err)
		return
	}

	for _, ev := range events {
		if err := dispatchEvent(ctx, queries, calculator, pool, ev, logger); err != nil {
			logger.Error("event dispatch failed",
				"event_id", ev.ID,
				"event_type", ev.EventType,
				"error", err,
			)
			_ = queries.MarkEventFailed(ctx, store.MarkEventFailedParams{
				ID:           ev.ID,
				ErrorMessage: strPtr(err.Error()),
			})
			continue
		}
		_ = queries.MarkEventProcessed(ctx, ev.ID)
	}
}

func dispatchEvent(ctx context.Context, queries *store.Queries, calculator *payroll.Calculator, pool *pgxpool.Pool, ev store.HrEvent, logger *slog.Logger) error {
	logger.Info("processing event", "type", ev.EventType, "aggregate", ev.AggregateType, "id", ev.AggregateID)

	switch ev.EventType {
	case "payroll.run_requested":
		return handlePayrollRunRequested(ctx, queries, calculator, pool, ev, logger)

	case "employee.hired", "employee.terminated", "employee.transferred":
		logger.Info("employee lifecycle event processed", "type", ev.EventType, "employee_id", ev.AggregateID)
		return nil

	case "leave.approved", "leave.cancelled":
		logger.Info("leave event processed", "type", ev.EventType)
		return nil

	case "overtime.approved":
		logger.Info("overtime event processed", "type", ev.EventType)
		return nil

	default:
		logger.Warn("unhandled event type", "type", ev.EventType)
		return nil
	}
}

func runPeriodicJobs(ctx context.Context, queries *store.Queries, pool *pgxpool.Pool, _ *redis.Client, logger *slog.Logger) {
	logger.Info("running periodic jobs")

	// Auto-close open attendance records from previous day
	autoCloseAttendance(ctx, queries, logger)

	// Check for overdue leave accruals (monthly)
	accrueLeaveBalances(ctx, queries, logger)

	// Year-end leave carryover (runs in January)
	processLeaveCarryover(ctx, queries, logger)

	// Monthly free tier token grant (1st of each month)
	grantFreeTokens(ctx, queries, logger)

	// Scan for contract milestones (probation ending, contract expiring, anniversaries)
	scanContractMilestones(ctx, queries, logger)

	// Clean up old milestones
	_ = queries.DeleteOldMilestones(ctx)

	// Mark overdue tax filings
	_ = queries.MarkOverdueFilings(ctx)

	// Mark expired documents (201 file)
	_ = queries.MarkExpiredDocuments(ctx)

	// Detect no-show employees and send notifications (10 AM only)
	checkNoShows(ctx, queries, pool, logger)

	// Calculate flight risk scores (weekly, Monday only)
	calculateFlightRisk(ctx, queries, pool, logger)

	// Calculate team health scores (weekly, Monday only)
	calculateTeamHealth(ctx, queries, pool, logger)

	// Calculate burnout risk scores (weekly, Monday only)
	calculateBurnoutScores(ctx, queries, pool, logger)

	// Generate compliance alerts (daily)
	generateComplianceAlerts(ctx, queries, pool, logger)

	// Auto-regularize probationary employees (daily)
	autoRegularize(ctx, queries, pool, logger)

	// Send proactive AI reminders (notifications)
	sendProactiveReminders(ctx, queries, logger)
}

func autoCloseAttendance(ctx context.Context, queries *store.Queries, logger *slog.Logger) {
	logger.Info("checking for open attendance records to auto-close")

	openRecords, err := queries.ListOpenAttendanceRecords(ctx)
	if err != nil {
		logger.Error("failed to list open attendance records", "error", err)
		return
	}

	if len(openRecords) == 0 {
		return
	}

	closed := 0
	for _, rec := range openRecords {
		if err := queries.AutoCloseAttendance(ctx, rec.ID); err != nil {
			logger.Error("failed to auto-close attendance",
				"id", rec.ID,
				"employee_id", rec.EmployeeID,
				"error", err,
			)
			continue
		}
		closed++
		logger.Info("auto-closed attendance record",
			"id", rec.ID,
			"employee_id", rec.EmployeeID,
			"clock_in", rec.ClockInAt.Time.Format(time.RFC3339),
		)
	}

	logger.Info("auto-close attendance completed", "total_open", len(openRecords), "closed", closed)
}

// grantFreeTokens grants monthly free tokens to eligible companies.
// Runs on the 1st of each month. Grants 1000 tokens to companies that
// haven't received a free grant this month.
func grantFreeTokens(ctx context.Context, queries *store.Queries, logger *slog.Logger) {
	now := time.Now()
	if now.Day() != 1 {
		return
	}

	logger.Info("running monthly free token grant")

	companies, err := queries.ListCompaniesForFreeGrant(ctx)
	if err != nil {
		logger.Error("failed to list companies for free grant", "error", err)
		return
	}

	if len(companies) == 0 {
		logger.Info("no companies eligible for free token grant")
		return
	}

	const freeTokens int64 = 1000
	granted := 0

	for _, companyID := range companies {
		if err := queries.GrantFreeTokens(ctx, store.GrantFreeTokensParams{
			CompanyID: companyID,
			Balance:   freeTokens,
		}); err != nil {
			logger.Error("failed to grant free tokens",
				"company_id", companyID,
				"error", err,
			)
			continue
		}
		granted++
		logger.Info("granted free tokens",
			"company_id", companyID,
			"tokens", freeTokens,
		)
	}

	logger.Info("monthly free token grant completed",
		"eligible", len(companies),
		"granted", granted,
	)
}

func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

func strPtr(s string) *string { return &s }
