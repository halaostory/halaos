package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/tonypk/aigonhr/internal/notification"
	"github.com/tonypk/aigonhr/internal/store"
)

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

		totalSent += sendMilestoneReminders(ctx, queries, logger, company.ID, admins, today)
		totalSent += sendExpiringDocReminders(ctx, queries, logger, company.ID, admins, today)
		totalSent += sendOverdueFilingReminders(ctx, queries, logger, company.ID, admins, today)
		totalSent += sendUpcomingFilingReminders(ctx, queries, logger, company.ID, admins, today)
		totalSent += sendPendingLeaveReminders(ctx, queries, logger, company.ID, managers, today)
		totalSent += sendPendingOTReminders(ctx, queries, logger, company.ID, managers, today)
	}

	if totalSent > 0 {
		logger.Info("proactive reminders sent", "total", totalSent)
	}
}

func sendMilestoneReminders(ctx context.Context, queries *store.Queries, logger *slog.Logger, companyID int64, admins []store.ListAdminUsersByCompanyRow, today time.Time) int {
	milestones, err := queries.ListPendingMilestonesByCompany(ctx, companyID)
	if err != nil || len(milestones) == 0 {
		return 0
	}

	sent := 0
	entityType := "milestone"
	for _, ms := range milestones {
		for _, admin := range admins {
			alreadySent, err := queries.HasReminderBeenSent(ctx, store.HasReminderBeenSentParams{
				CompanyID:     companyID,
				ReminderType:  "contract_milestone",
				EntityType:    &entityType,
				Column4:       ms.ID,
				ScheduledDate: today,
			})
			if err != nil {
				logger.Warn("failed to check reminder status, skipping", "type", "HasReminderBeenSent", "error", err)
				continue
			}
			if alreadySent {
				continue
			}

			title := fmt.Sprintf("Contract Milestone: %s", ms.MilestoneType)
			msg := fmt.Sprintf("Employee #%d has a %s milestone in %d days.",
				ms.EmployeeID, ms.MilestoneType, ms.DaysRemaining)

			actions := []notification.NotificationAction{
				{Label: "View Employee", Route: fmt.Sprintf("/employees/%d", ms.EmployeeID), Action: "view_employee"},
			}
			notification.Notify(ctx, queries, logger, companyID, admin.ID, title, msg, "ai_reminder", &entityType, &ms.ID, actions)

			if _, err := queries.InsertAIReminder(ctx, store.InsertAIReminderParams{
				CompanyID:     companyID,
				UserID:        admin.ID,
				ReminderType:  "contract_milestone",
				EntityType:    &entityType,
				EntityID:      &ms.ID,
				ScheduledDate: today,
			}); err != nil {
				logger.Warn("failed to insert AI reminder", "type", "contract_milestone", "error", err)
			}
			sent++
		}
	}
	return sent
}

func sendExpiringDocReminders(ctx context.Context, queries *store.Queries, logger *slog.Logger, companyID int64, admins []store.ListAdminUsersByCompanyRow, today time.Time) int {
	docs, err := queries.List201ExpiringDocuments(ctx, companyID)
	if err != nil || len(docs) == 0 {
		return 0
	}

	sent := 0
	entityType := "document"
	for _, admin := range admins {
		alreadySent, err := queries.HasReminderBeenSent(ctx, store.HasReminderBeenSentParams{
			CompanyID:     companyID,
			ReminderType:  "expiring_documents",
			EntityType:    &entityType,
			Column4:       0,
			ScheduledDate: today,
		})
		if err != nil {
			logger.Warn("failed to check reminder status, skipping", "type", "HasReminderBeenSent", "error", err)
			continue
		}
		if alreadySent {
			continue
		}

		title := "Expiring Documents Alert"
		msg := fmt.Sprintf("%d employee document(s) are expiring soon. Please review and renew.", len(docs))

		actions := []notification.NotificationAction{
			{Label: "View Documents", Route: "/employees", Action: "view_documents"},
		}
		notification.Notify(ctx, queries, logger, companyID, admin.ID, title, msg, "ai_reminder", &entityType, nil, actions)

		if _, err := queries.InsertAIReminder(ctx, store.InsertAIReminderParams{
			CompanyID:     companyID,
			UserID:        admin.ID,
			ReminderType:  "expiring_documents",
			EntityType:    &entityType,
			ScheduledDate: today,
		}); err != nil {
			logger.Warn("failed to insert AI reminder", "type", "expiring_documents", "error", err)
		}
		sent++
	}
	return sent
}

func sendOverdueFilingReminders(ctx context.Context, queries *store.Queries, logger *slog.Logger, companyID int64, admins []store.ListAdminUsersByCompanyRow, today time.Time) int {
	overdue, err := queries.ListOverdueFilings(ctx, companyID)
	if err != nil || len(overdue) == 0 {
		return 0
	}

	sent := 0
	entityType := "tax_filing"
	for _, admin := range admins {
		alreadySent, err := queries.HasReminderBeenSent(ctx, store.HasReminderBeenSentParams{
			CompanyID:     companyID,
			ReminderType:  "overdue_filings",
			EntityType:    &entityType,
			Column4:       0,
			ScheduledDate: today,
		})
		if err != nil {
			logger.Warn("failed to check reminder status, skipping", "type", "HasReminderBeenSent", "error", err)
			continue
		}
		if alreadySent {
			continue
		}

		title := "Overdue Government Filings"
		msg := fmt.Sprintf("%d government filing(s) are overdue. Please submit immediately.", len(overdue))

		notification.Notify(ctx, queries, logger, companyID, admin.ID, title, msg, "ai_reminder", &entityType, nil)

		if _, err := queries.InsertAIReminder(ctx, store.InsertAIReminderParams{
			CompanyID:     companyID,
			UserID:        admin.ID,
			ReminderType:  "overdue_filings",
			EntityType:    &entityType,
			ScheduledDate: today,
		}); err != nil {
			logger.Warn("failed to insert AI reminder", "type", "overdue_filings", "error", err)
		}
		sent++
	}
	return sent
}

func sendUpcomingFilingReminders(ctx context.Context, queries *store.Queries, logger *slog.Logger, companyID int64, admins []store.ListAdminUsersByCompanyRow, today time.Time) int {
	upcoming, err := queries.ListUpcomingFilings(ctx, companyID)
	if err != nil || len(upcoming) == 0 {
		return 0
	}

	sent := 0
	entityType := "tax_filing"
	for _, admin := range admins {
		alreadySent, err := queries.HasReminderBeenSent(ctx, store.HasReminderBeenSentParams{
			CompanyID:     companyID,
			ReminderType:  "upcoming_filings",
			EntityType:    &entityType,
			Column4:       0,
			ScheduledDate: today,
		})
		if err != nil {
			logger.Warn("failed to check reminder status, skipping", "type", "HasReminderBeenSent", "error", err)
			continue
		}
		if alreadySent {
			continue
		}

		title := "Upcoming Government Filings"
		msg := fmt.Sprintf("%d government filing(s) are due soon. Please prepare and submit.", len(upcoming))

		notification.Notify(ctx, queries, logger, companyID, admin.ID, title, msg, "ai_reminder", &entityType, nil)

		if _, err := queries.InsertAIReminder(ctx, store.InsertAIReminderParams{
			CompanyID:     companyID,
			UserID:        admin.ID,
			ReminderType:  "upcoming_filings",
			EntityType:    &entityType,
			ScheduledDate: today,
		}); err != nil {
			logger.Warn("failed to insert AI reminder", "type", "upcoming_filings", "error", err)
		}
		sent++
	}
	return sent
}

func sendPendingLeaveReminders(ctx context.Context, queries *store.Queries, logger *slog.Logger, companyID int64, managers []store.ListManagerUsersByCompanyRow, today time.Time) int {
	pendingLeaves, err := queries.ListPendingLeaveApprovals(ctx, companyID)
	if err != nil || len(pendingLeaves) == 0 {
		return 0
	}

	sent := 0
	entityType := "leave_request"
	for _, mgr := range managers {
		alreadySent, err := queries.HasReminderBeenSent(ctx, store.HasReminderBeenSentParams{
			CompanyID:     companyID,
			ReminderType:  "pending_approvals",
			EntityType:    &entityType,
			Column4:       0,
			ScheduledDate: today,
		})
		if err != nil {
			logger.Warn("failed to check reminder status, skipping", "type", "HasReminderBeenSent", "error", err)
			continue
		}
		if alreadySent {
			continue
		}

		title := "Pending Leave Approvals"
		msg := fmt.Sprintf("%d leave request(s) are waiting for your approval.", len(pendingLeaves))

		actions := []notification.NotificationAction{
			{Label: "Review All", Route: "/approvals", Action: "review_leave_approvals"},
		}
		// Add quick-approve for up to 3 individual pending leaves
		for i, lr := range pendingLeaves {
			if i >= 3 {
				break
			}
			empName := fmt.Sprintf("%v", lr.EmployeeName)
			actions = append(actions, notification.NotificationAction{
				Label:  fmt.Sprintf("Approve %s", empName),
				Action: "quick_approve",
				Params: map[string]any{"entity_type": "leave_request", "entity_id": lr.ID},
			})
		}
		notification.Notify(ctx, queries, logger, companyID, mgr.ID, title, msg, "ai_reminder", &entityType, nil, actions)

		if _, err := queries.InsertAIReminder(ctx, store.InsertAIReminderParams{
			CompanyID:     companyID,
			UserID:        mgr.ID,
			ReminderType:  "pending_approvals",
			EntityType:    &entityType,
			ScheduledDate: today,
		}); err != nil {
			logger.Warn("failed to insert AI reminder", "type", "pending_approvals", "error", err)
		}
		sent++
	}
	return sent
}

func sendPendingOTReminders(ctx context.Context, queries *store.Queries, logger *slog.Logger, companyID int64, managers []store.ListManagerUsersByCompanyRow, today time.Time) int {
	pendingOT, err := queries.ListPendingOvertimeApprovals(ctx, companyID)
	if err != nil || len(pendingOT) == 0 {
		return 0
	}

	sent := 0
	entityType := "overtime_request"
	for _, mgr := range managers {
		alreadySent, err := queries.HasReminderBeenSent(ctx, store.HasReminderBeenSentParams{
			CompanyID:     companyID,
			ReminderType:  "pending_ot_approvals",
			EntityType:    &entityType,
			Column4:       0,
			ScheduledDate: today,
		})
		if err != nil {
			logger.Warn("failed to check reminder status, skipping", "type", "HasReminderBeenSent", "error", err)
			continue
		}
		if alreadySent {
			continue
		}

		title := "Pending Overtime Approvals"
		msg := fmt.Sprintf("%d overtime request(s) are waiting for your approval.", len(pendingOT))

		actions := []notification.NotificationAction{
			{Label: "Review", Route: "/approvals", Action: "review_ot_approvals"},
		}
		// Add quick-approve for up to 3 individual pending OT requests
		for i, ot := range pendingOT {
			if i >= 3 {
				break
			}
			empName := fmt.Sprintf("%v", ot.EmployeeName)
			actions = append(actions, notification.NotificationAction{
				Label:  fmt.Sprintf("Approve %s", empName),
				Action: "quick_approve",
				Params: map[string]any{"entity_type": "overtime_request", "entity_id": ot.ID},
			})
		}
		notification.Notify(ctx, queries, logger, companyID, mgr.ID, title, msg, "ai_reminder", &entityType, nil, actions)

		if _, err := queries.InsertAIReminder(ctx, store.InsertAIReminderParams{
			CompanyID:     companyID,
			UserID:        mgr.ID,
			ReminderType:  "pending_ot_approvals",
			EntityType:    &entityType,
			ScheduledDate: today,
		}); err != nil {
			logger.Warn("failed to insert AI reminder", "type", "pending_ot_approvals", "error", err)
		}
		sent++
	}
	return sent
}
