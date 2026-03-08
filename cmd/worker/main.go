package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/tonypk/aigonhr/internal/config"
	"github.com/tonypk/aigonhr/internal/notification"
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

	// Event outbox processor
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				processEvents(ctx, queries, calculator, logger)
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
				runPeriodicJobs(ctx, queries, rdb, logger)
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

func processEvents(ctx context.Context, queries *store.Queries, calculator *payroll.Calculator, logger *slog.Logger) {
	events, err := queries.GetPendingEvents(ctx, 50)
	if err != nil {
		logger.Error("failed to get pending events", "error", err)
		return
	}

	for _, ev := range events {
		if err := dispatchEvent(ctx, queries, calculator, ev, logger); err != nil {
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

func dispatchEvent(ctx context.Context, queries *store.Queries, calculator *payroll.Calculator, ev store.HrEvent, logger *slog.Logger) error {
	logger.Info("processing event", "type", ev.EventType, "aggregate", ev.AggregateType, "id", ev.AggregateID)

	switch ev.EventType {
	case "payroll.run_requested":
		return calculator.RunPayroll(ctx, ev.AggregateID, ev.CompanyID)

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

func runPeriodicJobs(ctx context.Context, queries *store.Queries, _ *redis.Client, logger *slog.Logger) {
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

	// Send proactive AI reminders (notifications)
	sendProactiveReminders(ctx, queries, logger)
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

func accrueLeaveBalances(ctx context.Context, queries *store.Queries, logger *slog.Logger) {
	now := time.Now()
	if now.Day() != 1 {
		return // Only run on the 1st of each month
	}

	year := int32(now.Year())
	logger.Info("running monthly leave accrual", "month", fmt.Sprintf("%d-%02d", now.Year(), now.Month()))

	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("failed to list companies", "error", err)
		return
	}

	totalAccrued := 0
	for _, company := range companies {
		count, err := accrueForCompany(ctx, queries, company.ID, year, logger)
		if err != nil {
			logger.Error("failed to accrue leaves for company",
				"company_id", company.ID,
				"error", err,
			)
			continue
		}
		totalAccrued += count
	}

	logger.Info("monthly leave accrual completed",
		"companies", len(companies),
		"balances_accrued", totalAccrued,
	)
}

func accrueForCompany(ctx context.Context, queries *store.Queries, companyID int64, year int32, logger *slog.Logger) (int, error) {
	leaveTypes, err := queries.ListLeaveTypes(ctx, companyID)
	if err != nil {
		return 0, fmt.Errorf("list leave types: %w", err)
	}

	// Filter to monthly accrual types
	monthlyTypes := make([]store.LeaveType, 0)
	for _, lt := range leaveTypes {
		if lt.AccrualType == "monthly" {
			monthlyTypes = append(monthlyTypes, lt)
		}
	}
	if len(monthlyTypes) == 0 {
		return 0, nil
	}

	employees, err := queries.ListActiveEmployees(ctx, companyID)
	if err != nil {
		return 0, fmt.Errorf("list active employees: %w", err)
	}

	count := 0
	for _, emp := range employees {
		for _, lt := range monthlyTypes {
			// Check gender-specific eligibility
			if lt.GenderSpecific != nil && emp.Gender != nil && *lt.GenderSpecific != *emp.Gender {
				continue
			}

			// Calculate cumulative accrual: (default_days / 12) * months_elapsed
			defaultDays := numericToFloat(lt.DefaultDays)
			if defaultDays <= 0 {
				continue
			}
			monthsElapsed := float64(time.Now().Month())
			cumulativeEarned := (defaultDays / 12.0) * monthsElapsed

			// Round to 1 decimal
			cumulativeEarned = math.Round(cumulativeEarned*10) / 10

			var earned pgtype.Numeric
			_ = earned.Scan(fmt.Sprintf("%.1f", cumulativeEarned))

			var carried pgtype.Numeric
			_ = carried.Scan("0")

			_, err := queries.UpsertLeaveBalance(ctx, store.UpsertLeaveBalanceParams{
				CompanyID:   companyID,
				EmployeeID:  emp.ID,
				LeaveTypeID: lt.ID,
				Year:        year,
				Earned:      earned,
				Carried:     carried,
			})
			if err != nil {
				logger.Error("failed to upsert leave balance",
					"company_id", companyID,
					"employee_id", emp.ID,
					"leave_type_id", lt.ID,
					"error", err,
				)
				continue
			}
			count++
		}
	}

	return count, nil
}

func processLeaveCarryover(ctx context.Context, queries *store.Queries, logger *slog.Logger) {
	now := time.Now()
	// Run only in January (carry over from previous year)
	if now.Month() != time.January || now.Day() != 1 {
		return
	}

	prevYear := int32(now.Year() - 1)
	newYear := int32(now.Year())
	logger.Info("processing year-end leave carryover", "from_year", prevYear, "to_year", newYear)

	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("failed to list companies for carryover", "error", err)
		return
	}

	totalCarried := 0
	for _, company := range companies {
		balances, err := queries.ListLeaveBalancesForCarryover(ctx, store.ListLeaveBalancesForCarryoverParams{
			CompanyID: company.ID,
			Year:      prevYear,
		})
		if err != nil {
			logger.Error("failed to list balances for carryover", "company_id", company.ID, "error", err)
			continue
		}

		for _, bal := range balances {
			earned := numericToFloat(bal.Earned)
			used := numericToFloat(bal.Used)
			carried := numericToFloat(bal.Carried)
			adjusted := numericToFloat(bal.Adjusted)
			remaining := earned + carried + adjusted - used

			if remaining <= 0 {
				continue
			}

			// Apply max carryover cap
			maxCarry := numericToFloat(bal.MaxCarryover)
			if maxCarry <= 0 {
				maxCarry = 5 // Default 5 days
			}
			carryAmount := math.Min(remaining, maxCarry)
			carryAmount = math.Round(carryAmount*10) / 10

			var carriedNum pgtype.Numeric
			_ = carriedNum.Scan(fmt.Sprintf("%.1f", carryAmount))
			var zeroNum pgtype.Numeric
			_ = zeroNum.Scan("0")

			_, err := queries.UpsertLeaveBalance(ctx, store.UpsertLeaveBalanceParams{
				CompanyID:   company.ID,
				EmployeeID:  bal.EmployeeID,
				LeaveTypeID: bal.LeaveTypeID,
				Year:        newYear,
				Earned:      zeroNum,
				Carried:     carriedNum,
			})
			if err != nil {
				logger.Error("failed to create carryover balance",
					"employee_id", bal.EmployeeID,
					"leave_type_id", bal.LeaveTypeID,
					"error", err,
				)
				continue
			}

			forfeited := remaining - carryAmount
			logger.Info("leave carryover processed",
				"employee", fmt.Sprintf("%s %s (%s)", bal.FirstName, bal.LastName, bal.EmployeeNo),
				"leave_type", bal.LeaveTypeName,
				"remaining", remaining,
				"carried", carryAmount,
				"forfeited", forfeited,
			)
			totalCarried++
		}
	}

	logger.Info("leave carryover completed", "balances_carried", totalCarried)
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

func scanContractMilestones(ctx context.Context, queries *store.Queries, logger *slog.Logger) {
	logger.Info("scanning contract milestones")

	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("failed to list companies for milestone scan", "error", err)
		return
	}

	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	totalCreated := 0

	for _, company := range companies {
		// 1. Probation ending (Philippine law: 6 months max)
		probationary, err := queries.ListProbationaryEmployees(ctx, company.ID)
		if err != nil {
			logger.Error("failed to list probationary employees", "company_id", company.ID, "error", err)
			continue
		}
		for _, emp := range probationary {
			// Probation ends 6 months after hire
			probationEnd := emp.HireDate.AddDate(0, 6, 0)
			daysRemaining := int(probationEnd.Sub(today).Hours() / 24)

			// Alert if within 30 days
			if daysRemaining <= 30 && daysRemaining >= -7 {
				_, err := queries.UpsertContractMilestone(ctx, store.UpsertContractMilestoneParams{
					CompanyID:     company.ID,
					EmployeeID:    emp.ID,
					MilestoneType: "probation_ending",
					MilestoneDate: probationEnd,
					DaysRemaining: int32(daysRemaining),
				})
				if err != nil {
					logger.Error("failed to upsert probation milestone", "employee_id", emp.ID, "error", err)
					continue
				}
				totalCreated++
			}
		}

		// 2. Contract expiring
		contractual, err := queries.ListContractualEmployees(ctx, company.ID)
		if err != nil {
			logger.Error("failed to list contractual employees", "company_id", company.ID, "error", err)
			continue
		}
		for _, emp := range contractual {
			if !emp.ContractEndDate.Valid {
				continue
			}
			contractEnd := emp.ContractEndDate.Time
			daysRemaining := int(contractEnd.Sub(today).Hours() / 24)

			// Alert if within 60 days
			if daysRemaining <= 60 && daysRemaining >= -7 {
				_, err := queries.UpsertContractMilestone(ctx, store.UpsertContractMilestoneParams{
					CompanyID:     company.ID,
					EmployeeID:    emp.ID,
					MilestoneType: "contract_expiring",
					MilestoneDate: contractEnd,
					DaysRemaining: int32(daysRemaining),
				})
				if err != nil {
					logger.Error("failed to upsert contract milestone", "employee_id", emp.ID, "error", err)
					continue
				}
				totalCreated++
			}
		}

		// 3. Upcoming work anniversaries (this month)
		month := int32(now.Month())
		anniversaries, err := queries.ListUpcomingAnniversaries(ctx, store.ListUpcomingAnniversariesParams{
			CompanyID: company.ID,
			Column2:   month,
			Column3:   1,
			Column4:   31,
		})
		if err != nil {
			logger.Error("failed to list anniversaries", "company_id", company.ID, "error", err)
			continue
		}
		for _, emp := range anniversaries {
			yearsOfService := now.Year() - emp.HireDate.Year()
			if yearsOfService <= 0 {
				continue
			}
			// Only milestone years: 1, 3, 5, 10, 15, 20, 25, 30
			if !isAnniversaryMilestone(yearsOfService) {
				continue
			}
			anniversaryDate := emp.HireDate.AddDate(yearsOfService, 0, 0)
			daysRemaining := int(anniversaryDate.Sub(today).Hours() / 24)

			_, err := queries.UpsertContractMilestone(ctx, store.UpsertContractMilestoneParams{
				CompanyID:     company.ID,
				EmployeeID:    emp.ID,
				MilestoneType: "anniversary",
				MilestoneDate: anniversaryDate,
				DaysRemaining: int32(daysRemaining),
			})
			if err != nil {
				logger.Error("failed to upsert anniversary milestone", "employee_id", emp.ID, "error", err)
				continue
			}
			totalCreated++
		}
	}

	logger.Info("contract milestone scan completed", "milestones_created_or_updated", totalCreated)
}

func isAnniversaryMilestone(years int) bool {
	switch years {
	case 1, 3, 5, 10, 15, 20, 25, 30:
		return true
	default:
		return false
	}
}

// sendProactiveReminders checks for important HR items and sends notifications.
// Runs hourly but uses HasReminderBeenSent to avoid duplicate daily notifications.
func sendProactiveReminders(ctx context.Context, queries *store.Queries, logger *slog.Logger) {
	today := time.Now().Truncate(24 * time.Hour)

	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("reminders: failed to list companies", "error", err)
		return
	}

	totalSent := 0
	for _, company := range companies {
		// Get admin users for this company
		admins, err := queries.ListAdminUsersByCompany(ctx, company.ID)
		if err != nil || len(admins) == 0 {
			continue
		}

		// Get manager users (fallback to admins if no managers found)
		managers, err := queries.ListManagerUsersByCompany(ctx, company.ID)
		if err != nil || len(managers) == 0 {
			// Convert admins to manager rows for type compatibility
			managers = make([]store.ListManagerUsersByCompanyRow, len(admins))
			for i, a := range admins {
				managers[i] = store.ListManagerUsersByCompanyRow{ID: a.ID, CompanyID: a.CompanyID}
			}
		}

		// 1. Contract milestones (notify admins)
		milestones, err := queries.ListPendingMilestonesByCompany(ctx, company.ID)
		if err == nil && len(milestones) > 0 {
			for _, ms := range milestones {
				entityType := "milestone"
				for _, admin := range admins {
					sent, err := queries.HasReminderBeenSent(ctx, store.HasReminderBeenSentParams{
						CompanyID:     company.ID,
						ReminderType:  "contract_milestone",
						EntityType:    &entityType,
						Column4:       ms.ID,
						ScheduledDate: today,
					})
					if err != nil {
						logger.Warn("failed to check reminder status, skipping", "type", "HasReminderBeenSent", "error", err)
						continue
					}
					if sent {
						continue
					}

					title := fmt.Sprintf("Contract Milestone: %s", ms.MilestoneType)
					msg := fmt.Sprintf("Employee #%d has a %s milestone in %d days.",
						ms.EmployeeID, ms.MilestoneType, ms.DaysRemaining)

					notification.Notify(ctx, queries, logger, company.ID, admin.ID, title, msg, "ai_reminder", &entityType, &ms.ID)

					_, _ = queries.InsertAIReminder(ctx, store.InsertAIReminderParams{
						CompanyID:     company.ID,
						UserID:        admin.ID,
						ReminderType:  "contract_milestone",
						EntityType:    &entityType,
						EntityID:      &ms.ID,
						ScheduledDate: today,
					})
					totalSent++
				}
			}
		}

		// 2. Expiring documents (notify admins)
		docs, err := queries.List201ExpiringDocuments(ctx, company.ID)
		if err == nil && len(docs) > 0 {
			entityType := "document"
			for _, admin := range admins {
				sent, err := queries.HasReminderBeenSent(ctx, store.HasReminderBeenSentParams{
					CompanyID:     company.ID,
					ReminderType:  "expiring_documents",
					EntityType:    &entityType,
					Column4:       0,
					ScheduledDate: today,
				})
				if err != nil {
					logger.Warn("failed to check reminder status, skipping", "type", "HasReminderBeenSent", "error", err)
					continue
				}
				if sent {
					continue
				}

				title := "Expiring Documents Alert"
				msg := fmt.Sprintf("%d employee document(s) are expiring soon. Please review and renew.", len(docs))

				notification.Notify(ctx, queries, logger, company.ID, admin.ID, title, msg, "ai_reminder", &entityType, nil)

				_, _ = queries.InsertAIReminder(ctx, store.InsertAIReminderParams{
					CompanyID:     company.ID,
					UserID:        admin.ID,
					ReminderType:  "expiring_documents",
					EntityType:    &entityType,
					ScheduledDate: today,
				})
				totalSent++
			}
		}

		// 3. Overdue tax filings (notify admins)
		overdue, err := queries.ListOverdueFilings(ctx, company.ID)
		if err == nil && len(overdue) > 0 {
			entityType := "tax_filing"
			for _, admin := range admins {
				sent, err := queries.HasReminderBeenSent(ctx, store.HasReminderBeenSentParams{
					CompanyID:     company.ID,
					ReminderType:  "overdue_filings",
					EntityType:    &entityType,
					Column4:       0,
					ScheduledDate: today,
				})
				if err != nil {
					logger.Warn("failed to check reminder status, skipping", "type", "HasReminderBeenSent", "error", err)
					continue
				}
				if sent {
					continue
				}

				title := "Overdue Government Filings"
				msg := fmt.Sprintf("%d government filing(s) are overdue. Please submit immediately.", len(overdue))

				notification.Notify(ctx, queries, logger, company.ID, admin.ID, title, msg, "ai_reminder", &entityType, nil)

				_, _ = queries.InsertAIReminder(ctx, store.InsertAIReminderParams{
					CompanyID:     company.ID,
					UserID:        admin.ID,
					ReminderType:  "overdue_filings",
					EntityType:    &entityType,
					ScheduledDate: today,
				})
				totalSent++
			}
		}

		// 4. Upcoming tax filings (notify admins)
		upcoming, err := queries.ListUpcomingFilings(ctx, company.ID)
		if err == nil && len(upcoming) > 0 {
			entityType := "tax_filing"
			for _, admin := range admins {
				sent, err := queries.HasReminderBeenSent(ctx, store.HasReminderBeenSentParams{
					CompanyID:     company.ID,
					ReminderType:  "upcoming_filings",
					EntityType:    &entityType,
					Column4:       0,
					ScheduledDate: today,
				})
				if err != nil {
					logger.Warn("failed to check reminder status, skipping", "type", "HasReminderBeenSent", "error", err)
					continue
				}
				if sent {
					continue
				}

				title := "Upcoming Government Filings"
				msg := fmt.Sprintf("%d government filing(s) are due soon. Please prepare and submit.", len(upcoming))

				notification.Notify(ctx, queries, logger, company.ID, admin.ID, title, msg, "ai_reminder", &entityType, nil)

				_, _ = queries.InsertAIReminder(ctx, store.InsertAIReminderParams{
					CompanyID:     company.ID,
					UserID:        admin.ID,
					ReminderType:  "upcoming_filings",
					EntityType:    &entityType,
					ScheduledDate: today,
				})
				totalSent++
			}
		}

		// 5. Pending leave approvals (notify managers)
		pendingLeaves, err := queries.ListPendingLeaveApprovals(ctx, company.ID)
		if err == nil && len(pendingLeaves) > 0 {
			entityType := "leave_request"
			for _, mgr := range managers {
				sent, err := queries.HasReminderBeenSent(ctx, store.HasReminderBeenSentParams{
					CompanyID:     company.ID,
					ReminderType:  "pending_approvals",
					EntityType:    &entityType,
					Column4:       0,
					ScheduledDate: today,
				})
				if err != nil {
					logger.Warn("failed to check reminder status, skipping", "type", "HasReminderBeenSent", "error", err)
					continue
				}
				if sent {
					continue
				}

				title := "Pending Leave Approvals"
				msg := fmt.Sprintf("%d leave request(s) are waiting for your approval.", len(pendingLeaves))

				notification.Notify(ctx, queries, logger, company.ID, mgr.ID, title, msg, "ai_reminder", &entityType, nil)

				_, _ = queries.InsertAIReminder(ctx, store.InsertAIReminderParams{
					CompanyID:     company.ID,
					UserID:        mgr.ID,
					ReminderType:  "pending_approvals",
					EntityType:    &entityType,
					ScheduledDate: today,
				})
				totalSent++
			}
		}

		// 6. Pending overtime approvals (notify managers)
		pendingOT, err := queries.ListPendingOvertimeApprovals(ctx, company.ID)
		if err == nil && len(pendingOT) > 0 {
			entityType := "overtime_request"
			for _, mgr := range managers {
				sent, err := queries.HasReminderBeenSent(ctx, store.HasReminderBeenSentParams{
					CompanyID:     company.ID,
					ReminderType:  "pending_ot_approvals",
					EntityType:    &entityType,
					Column4:       0,
					ScheduledDate: today,
				})
				if err != nil {
					logger.Warn("failed to check reminder status, skipping", "type", "HasReminderBeenSent", "error", err)
					continue
				}
				if sent {
					continue
				}

				title := "Pending Overtime Approvals"
				msg := fmt.Sprintf("%d overtime request(s) are waiting for your approval.", len(pendingOT))

				notification.Notify(ctx, queries, logger, company.ID, mgr.ID, title, msg, "ai_reminder", &entityType, nil)

				_, _ = queries.InsertAIReminder(ctx, store.InsertAIReminderParams{
					CompanyID:     company.ID,
					UserID:        mgr.ID,
					ReminderType:  "pending_ot_approvals",
					EntityType:    &entityType,
					ScheduledDate: today,
				})
				totalSent++
			}
		}
	}

	if totalSent > 0 {
		logger.Info("proactive reminders sent", "total", totalSent)
	}
}
