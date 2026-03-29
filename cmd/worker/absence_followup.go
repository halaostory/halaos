package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/notification"
	"github.com/halaostory/halaos/internal/store"
)

// checkAbsenceFollowup runs at 3 PM to handle employees who received a no-show
// notification at 10 AM but still haven't clocked in or responded. For each such
// employee, it marks them as absent (AWOL) and notifies their manager.
func checkAbsenceFollowup(ctx context.Context, queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) {
	now := time.Now()
	if now.Hour() != 15 {
		return
	}

	logger.Info("checking absence follow-up for unresolved no-shows")

	today := now.Truncate(24 * time.Hour)

	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("absence-followup: failed to list companies", "error", err)
		return
	}

	totalMarked := 0
	for _, company := range companies {
		count, err := processAbsenceFollowup(ctx, queries, pool, logger, company.ID, today)
		if err != nil {
			logger.Error("absence-followup: failed for company",
				"company_id", company.ID,
				"error", err,
			)
			continue
		}
		totalMarked += count
	}

	if totalMarked > 0 {
		logger.Info("absence follow-up completed", "total_marked_absent", totalMarked)
	}
}

func processAbsenceFollowup(
	ctx context.Context,
	queries *store.Queries,
	pool *pgxpool.Pool,
	logger *slog.Logger,
	companyID int64,
	today time.Time,
) (int, error) {
	// Skip holidays
	isHoliday, err := isTodayHoliday(ctx, pool, companyID, today)
	if err != nil {
		return 0, fmt.Errorf("check holiday: %w", err)
	}
	if isHoliday {
		return 0, nil
	}

	// Get employees who were notified of no-show today
	notifiedEmployees, err := getNoShowNotifiedEmployees(ctx, pool, companyID, today)
	if err != nil {
		return 0, fmt.Errorf("get notified employees: %w", err)
	}
	if len(notifiedEmployees) == 0 {
		return 0, nil
	}

	// Check who still hasn't clocked in
	clockedIn, err := getEmployeesClockedIn(ctx, pool, companyID, today)
	if err != nil {
		return 0, fmt.Errorf("get clocked in: %w", err)
	}

	// Check who filed leave since the notification
	onLeave, err := getEmployeesOnLeave(ctx, pool, companyID, today)
	if err != nil {
		return 0, fmt.Errorf("get on leave: %w", err)
	}

	// Also check pending leave (they clicked quick_sick_leave but it's not yet approved)
	pendingLeave, err := getEmployeesWithPendingLeave(ctx, pool, companyID, today)
	if err != nil {
		return 0, fmt.Errorf("get pending leave: %w", err)
	}

	entityType := "attendance"
	markedCount := 0

	// Collect absent employees for manager summary
	absentEmployees := make([]noShowEmployee, 0)

	for _, emp := range notifiedEmployees {
		if clockedIn[emp.ID] || onLeave[emp.ID] || pendingLeave[emp.ID] {
			continue // They resolved it
		}

		// Check dedup — already sent follow-up today?
		sent, err := queries.HasReminderBeenSent(ctx, store.HasReminderBeenSentParams{
			CompanyID:     companyID,
			ReminderType:  "absence_followup",
			EntityType:    &entityType,
			Column4:       emp.ID,
			ScheduledDate: today,
		})
		if err != nil {
			logger.Warn("absence-followup: dedup check failed", "employee_id", emp.ID, "error", err)
			continue
		}
		if sent {
			continue
		}

		// Send final notice to employee
		if emp.UserID != nil {
			todayStr := today.Format("2006-01-02")
			title := "Absent Today — No Clock-in Recorded"
			msg := fmt.Sprintf(
				"Hi %s, you haven't clocked in or filed a leave request today. "+
					"Your attendance has been marked as absent. "+
					"You can still file a leave request to update this record.",
				emp.FirstName,
			)
			actions := []notification.NotificationAction{
				{Label: "File Sick Leave", Action: "quick_sick_leave", Params: map[string]any{"employee_id": emp.ID, "date": todayStr}},
				{Label: "File Vacation Leave", Action: "quick_vacation_leave", Params: map[string]any{"employee_id": emp.ID, "date": todayStr}},
				{Label: "View Attendance", Route: "/attendance"},
			}
			notification.Notify(ctx, queries, logger, companyID, *emp.UserID, title, msg, "ai_reminder", &entityType, &emp.ID, actions)
		}

		// Record dedup
		if _, err := queries.InsertAIReminder(ctx, store.InsertAIReminderParams{
			CompanyID:     companyID,
			UserID:        safeUserID(emp.UserID),
			ReminderType:  "absence_followup",
			EntityType:    &entityType,
			EntityID:      &emp.ID,
			ScheduledDate: today,
		}); err != nil {
			logger.Warn("failed to insert absence_followup reminder", "error", err)
		}

		// Emit absent event for audit
		idempKey := fmt.Sprintf("attendance.absent.%d.%s", emp.ID, today.Format("2006-01-02"))
		payload, _ := json.Marshal(map[string]any{
			"employee_id": emp.ID,
			"date":        today.Format("2006-01-02"),
			"source":      "auto_followup",
		})
		_, _ = queries.InsertHREvent(ctx, store.InsertHREventParams{
			CompanyID:      companyID,
			AggregateType:  "attendance",
			AggregateID:    emp.ID,
			EventType:      "attendance.absent",
			EventVersion:   1,
			Payload:        payload,
			IdempotencyKey: &idempKey,
		})

		absentEmployees = append(absentEmployees, emp)
		markedCount++
	}

	// Notify managers about unresolved absences
	if len(absentEmployees) > 0 {
		notifyManagersAbsent(ctx, queries, logger, companyID, today, absentEmployees, entityType)
	}

	return markedCount, nil
}

// notifyManagersAbsent sends a summary to managers about employees marked absent.
func notifyManagersAbsent(
	ctx context.Context,
	queries *store.Queries,
	logger *slog.Logger,
	companyID int64,
	today time.Time,
	absentEmployees []noShowEmployee,
	entityType string,
) {
	managers, err := queries.ListManagerUsersByCompany(ctx, companyID)
	if err != nil || len(managers) == 0 {
		admins, err := queries.ListAdminUsersByCompany(ctx, companyID)
		if err != nil || len(admins) == 0 {
			return
		}
		managers = make([]store.ListManagerUsersByCompanyRow, len(admins))
		for i, a := range admins {
			managers[i] = store.ListManagerUsersByCompanyRow{ID: a.ID, CompanyID: a.CompanyID}
		}
	}

	// Build names list
	names := make([]string, 0, len(absentEmployees))
	for _, emp := range absentEmployees {
		names = append(names, fmt.Sprintf("%s %s", emp.FirstName, emp.LastName))
	}

	title := "Absent Employees — No Response"
	msg := fmt.Sprintf(
		"%d employee(s) were notified at 10 AM but haven't clocked in or filed leave by 3 PM: %s",
		len(absentEmployees),
		truncateNames(names, 5),
	)

	actions := []notification.NotificationAction{
		{Label: "View Attendance", Route: "/attendance/records"},
	}

	for _, mgr := range managers {
		sent, _ := queries.HasReminderBeenSent(ctx, store.HasReminderBeenSentParams{
			CompanyID:     companyID,
			ReminderType:  "absence_summary",
			EntityType:    &entityType,
			Column4:       0,
			ScheduledDate: today,
		})
		if sent {
			continue
		}

		notification.Notify(ctx, queries, logger, companyID, mgr.ID, title, msg, "ai_reminder", &entityType, nil, actions)

		if _, err := queries.InsertAIReminder(ctx, store.InsertAIReminderParams{
			CompanyID:     companyID,
			UserID:        mgr.ID,
			ReminderType:  "absence_summary",
			EntityType:    &entityType,
			ScheduledDate: today,
		}); err != nil {
			logger.Warn("failed to insert absence_summary reminder", "error", err)
		}
	}
}

// noShowEmployee is a lightweight struct for absence follow-up processing.
type noShowEmployee struct {
	ID        int64
	FirstName string
	LastName  string
	UserID    *int64
}

// getNoShowNotifiedEmployees returns employees who received a no-show notification today.
func getNoShowNotifiedEmployees(ctx context.Context, pool *pgxpool.Pool, companyID int64, today time.Time) ([]noShowEmployee, error) {
	rows, err := pool.Query(ctx,
		`SELECT e.id, e.first_name, e.last_name, e.user_id
		 FROM ai_reminders ar
		 JOIN employees e ON e.id = ar.entity_id AND e.company_id = ar.company_id
		 WHERE ar.company_id = $1
		   AND ar.reminder_type = 'no_show'
		   AND ar.scheduled_date = $2
		   AND ar.entity_type = 'attendance'
		   AND e.status = 'active'`,
		companyID, today,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []noShowEmployee
	for rows.Next() {
		var emp noShowEmployee
		if err := rows.Scan(&emp.ID, &emp.FirstName, &emp.LastName, &emp.UserID); err != nil {
			return nil, err
		}
		result = append(result, emp)
	}
	return result, rows.Err()
}

// getEmployeesWithPendingLeave returns employee IDs who have a pending leave request for today.
func getEmployeesWithPendingLeave(ctx context.Context, pool *pgxpool.Pool, companyID int64, date time.Time) (map[int64]bool, error) {
	rows, err := pool.Query(ctx,
		`SELECT employee_id FROM leave_requests
		 WHERE company_id = $1 AND status = 'pending'
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

// safeUserID returns the user ID or 0 if nil.
func safeUserID(uid *int64) int64 {
	if uid == nil {
		return 0
	}
	return *uid
}

// truncateNames formats a list of names, showing at most n with a "+X more" suffix.
func truncateNames(names []string, n int) string {
	if len(names) <= n {
		result := ""
		for i, name := range names {
			if i > 0 {
				result += ", "
			}
			result += name
		}
		return result
	}
	result := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			result += ", "
		}
		result += names[i]
	}
	return fmt.Sprintf("%s +%d more", result, len(names)-n)
}
