package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/notification"
	"github.com/tonypk/aigonhr/internal/store"
)

// checkNoShows detects employees who haven't clocked in by 10 AM and sends
// notifications to both the employee and their managers. It runs only at the
// 10 AM hour, skips employees on approved leave, and respects company holidays.
// Uses the reminder dedup system to avoid duplicate daily notifications.
func checkNoShows(ctx context.Context, queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) {
	now := time.Now()
	if now.Hour() != 10 {
		return
	}

	logger.Info("checking for no-show employees")

	today := now.Truncate(24 * time.Hour)

	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("no-show: failed to list companies", "error", err)
		return
	}

	totalNotified := 0
	for _, company := range companies {
		count, err := checkNoShowsForCompany(ctx, queries, pool, logger, company.ID, today)
		if err != nil {
			logger.Error("no-show: failed to check company",
				"company_id", company.ID,
				"error", err,
			)
			continue
		}
		totalNotified += count
	}

	if totalNotified > 0 {
		logger.Info("no-show check completed", "total_notified", totalNotified)
	}
}

// checkNoShowsForCompany processes no-show detection for a single company.
// Returns the number of notifications sent.
func checkNoShowsForCompany(
	ctx context.Context,
	queries *store.Queries,
	pool *pgxpool.Pool,
	logger *slog.Logger,
	companyID int64,
	today time.Time,
) (int, error) {
	// Check if today is a company holiday
	isHoliday, err := isTodayHoliday(ctx, pool, companyID, today)
	if err != nil {
		return 0, fmt.Errorf("check holiday: %w", err)
	}
	if isHoliday {
		logger.Info("no-show: skipping company (holiday today)", "company_id", companyID)
		return 0, nil
	}

	employees, err := queries.ListActiveEmployees(ctx, companyID)
	if err != nil {
		return 0, fmt.Errorf("list active employees: %w", err)
	}

	if len(employees) == 0 {
		return 0, nil
	}

	// Get set of employee IDs on approved leave today
	onLeave, err := getEmployeesOnLeave(ctx, pool, companyID, today)
	if err != nil {
		return 0, fmt.Errorf("get employees on leave: %w", err)
	}

	// Get set of employee IDs who have clocked in today
	clockedIn, err := getEmployeesClockedIn(ctx, pool, companyID, today)
	if err != nil {
		return 0, fmt.Errorf("get employees clocked in: %w", err)
	}

	// Find no-show employees
	noShowEmployees := make([]store.Employee, 0)
	for _, emp := range employees {
		// Skip if on approved leave
		if onLeave[emp.ID] {
			continue
		}
		// Skip if already clocked in
		if clockedIn[emp.ID] {
			continue
		}
		noShowEmployees = append(noShowEmployees, emp)
	}

	if len(noShowEmployees) == 0 {
		return 0, nil
	}

	entityType := "attendance"
	notified := 0

	// Notify each no-show employee (if they have a user account)
	for _, emp := range noShowEmployees {
		if emp.UserID == nil {
			continue
		}

		sent, err := queries.HasReminderBeenSent(ctx, store.HasReminderBeenSentParams{
			CompanyID:     companyID,
			ReminderType:  "no_show",
			EntityType:    &entityType,
			Column4:       emp.ID,
			ScheduledDate: today,
		})
		if err != nil {
			logger.Warn("no-show: failed to check reminder status",
				"employee_id", emp.ID,
				"error", err,
			)
			continue
		}
		if sent {
			continue
		}

		title := "You haven't clocked in today"
		msg := fmt.Sprintf(
			"Hi %s, you haven't clocked in today. Are you on leave?",
			emp.FirstName,
		)

		todayStr := today.Format("2006-01-02")
		actions := []notification.NotificationAction{
			{Label: "Request Sick Leave", Action: "quick_sick_leave", Params: map[string]any{"employee_id": emp.ID, "date": todayStr}},
			{Label: "Request Vacation Leave", Action: "quick_vacation_leave", Params: map[string]any{"employee_id": emp.ID, "date": todayStr}},
			{Label: "I'm On My Way", Action: "dismiss", Route: "/attendance"},
		}

		notification.Notify(ctx, queries, logger, companyID, *emp.UserID, title, msg, "ai_reminder", &entityType, &emp.ID, actions)

		if _, err := queries.InsertAIReminder(ctx, store.InsertAIReminderParams{
			CompanyID:     companyID,
			UserID:        *emp.UserID,
			ReminderType:  "no_show",
			EntityType:    &entityType,
			EntityID:      &emp.ID,
			ScheduledDate: today,
		}); err != nil {
			logger.Warn("failed to insert AI reminder", "type", "no_show", "error", err)
		}
		notified++
	}

	// Send summary notification to managers
	if err := notifyManagersNoShow(ctx, queries, logger, companyID, today, noShowEmployees, entityType); err != nil {
		logger.Error("no-show: failed to notify managers",
			"company_id", companyID,
			"error", err,
		)
	} else {
		notified++
	}

	return notified, nil
}

// notifyManagersNoShow sends a summary notification to all managers about
// employees who haven't clocked in by 10 AM.
func notifyManagersNoShow(
	ctx context.Context,
	queries *store.Queries,
	logger *slog.Logger,
	companyID int64,
	today time.Time,
	noShowEmployees []store.Employee,
	entityType string,
) error {
	if len(noShowEmployees) == 0 {
		return nil
	}

	// Get managers (fallback to admins)
	managers, err := queries.ListManagerUsersByCompany(ctx, companyID)
	if err != nil || len(managers) == 0 {
		admins, err := queries.ListAdminUsersByCompany(ctx, companyID)
		if err != nil || len(admins) == 0 {
			return fmt.Errorf("no managers or admins found for company %d", companyID)
		}
		managers = make([]store.ListManagerUsersByCompanyRow, len(admins))
		for i, a := range admins {
			managers[i] = store.ListManagerUsersByCompanyRow{ID: a.ID, CompanyID: a.CompanyID}
		}
	}

	title := "No-Show Attendance Alert"
	msg := fmt.Sprintf(
		"%d employee(s) haven't clocked in by 10 AM today. Please review.",
		len(noShowEmployees),
	)

	actions := []notification.NotificationAction{
		{Label: "View Attendance", Route: "/attendance", Action: "view_attendance"},
	}

	for _, mgr := range managers {
		sent, err := queries.HasReminderBeenSent(ctx, store.HasReminderBeenSentParams{
			CompanyID:     companyID,
			ReminderType:  "no_show_summary",
			EntityType:    &entityType,
			Column4:       0,
			ScheduledDate: today,
		})
		if err != nil {
			logger.Warn("no-show: failed to check manager reminder status",
				"manager_id", mgr.ID,
				"error", err,
			)
			continue
		}
		if sent {
			continue
		}

		notification.Notify(ctx, queries, logger, companyID, mgr.ID, title, msg, "ai_reminder", &entityType, nil, actions)

		if _, err := queries.InsertAIReminder(ctx, store.InsertAIReminderParams{
			CompanyID:     companyID,
			UserID:        mgr.ID,
			ReminderType:  "no_show_summary",
			EntityType:    &entityType,
			ScheduledDate: today,
		}); err != nil {
			logger.Warn("failed to insert AI reminder", "type", "no_show_summary", "error", err)
		}
	}

	return nil
}

// isTodayHoliday checks if the given date is a company holiday.
func isTodayHoliday(ctx context.Context, pool *pgxpool.Pool, companyID int64, date time.Time) (bool, error) {
	var count int64
	err := pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM holidays WHERE company_id = $1 AND date = $2",
		companyID, date,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// getEmployeesOnLeave returns a set of employee IDs that have approved leave
// covering the given date.
func getEmployeesOnLeave(ctx context.Context, pool *pgxpool.Pool, companyID int64, date time.Time) (map[int64]bool, error) {
	rows, err := pool.Query(ctx,
		`SELECT employee_id FROM leave_requests
		 WHERE company_id = $1 AND status = 'approved'
		   AND start_date <= $2 AND end_date >= $2`,
		companyID, date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]bool)
	for rows.Next() {
		var empID int64
		if err := rows.Scan(&empID); err != nil {
			return nil, err
		}
		result[empID] = true
	}
	return result, rows.Err()
}

// getEmployeesClockedIn returns a set of employee IDs that have attendance
// records for the given date.
func getEmployeesClockedIn(ctx context.Context, pool *pgxpool.Pool, companyID int64, date time.Time) (map[int64]bool, error) {
	rows, err := pool.Query(ctx,
		`SELECT DISTINCT employee_id FROM attendance_logs
		 WHERE company_id = $1 AND clock_in_at::date = $2`,
		companyID, date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]bool)
	for rows.Next() {
		var empID int64
		if err := rows.Scan(&empID); err != nil {
			return nil, err
		}
		result[empID] = true
	}
	return result, rows.Err()
}
