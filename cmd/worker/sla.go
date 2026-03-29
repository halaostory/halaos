package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/notification"
	"github.com/halaostory/halaos/internal/store"
)

// checkApprovalSLAs checks pending approvals against SLA configs and sends
// reminders, escalations, or auto-actions as needed. Runs hourly.
func checkApprovalSLAs(ctx context.Context, queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) {
	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("sla: failed to list companies", "error", err)
		return
	}

	totalActions := 0
	for _, company := range companies {
		count := checkSLAsForCompany(ctx, queries, pool, logger, company.ID)
		totalActions += count
	}

	if totalActions > 0 {
		logger.Info("SLA check completed", "total_actions", totalActions)
	}
}

func checkSLAsForCompany(
	ctx context.Context,
	queries *store.Queries,
	pool *pgxpool.Pool,
	logger *slog.Logger,
	companyID int64,
) int {
	actions := 0

	// Check leave request SLAs
	leaveSLA, err := queries.GetSLAConfig(ctx, store.GetSLAConfigParams{
		CompanyID:  companyID,
		EntityType: "leave_request",
	})
	if err == nil && leaveSLA.IsActive {
		pendingLeaves, err := queries.ListPendingLeaveRequestsWithAge(ctx, companyID)
		if err == nil {
			for _, req := range pendingLeaves {
				ageHours := float64(req.AgeHours)
				empName := fmt.Sprintf("%v", req.EmployeeName)
				actions += processSLAForEntity(ctx, queries, pool, logger, companyID,
					"leave_request", req.ID, ageHours, empName, leaveSLA)
			}
		}
	}

	// Check OT request SLAs
	otSLA, err := queries.GetSLAConfig(ctx, store.GetSLAConfigParams{
		CompanyID:  companyID,
		EntityType: "overtime_request",
	})
	if err == nil && otSLA.IsActive {
		pendingOT, err := queries.ListPendingOTRequestsWithAge(ctx, companyID)
		if err == nil {
			for _, req := range pendingOT {
				ageHours := float64(req.AgeHours)
				empName := fmt.Sprintf("%v", req.EmployeeName)
				actions += processSLAForEntity(ctx, queries, pool, logger, companyID,
					"overtime_request", req.ID, ageHours, empName, otSLA)
			}
		}
	}

	return actions
}

func processSLAForEntity(
	ctx context.Context,
	queries *store.Queries,
	pool *pgxpool.Pool,
	logger *slog.Logger,
	companyID int64,
	entityType string,
	entityID int64,
	ageHours float64,
	employeeName string,
	sla store.ApprovalSlaConfig,
) int {
	actions := 0

	// Auto-action (highest priority check)
	if sla.AutoAction != "none" && ageHours >= float64(sla.AutoActionHours) {
		sent, _ := queries.HasSLAEventBeenSent(ctx, store.HasSLAEventBeenSentParams{
			CompanyID:  companyID,
			EntityType: entityType,
			EntityID:   entityID,
			EventType:  "auto_" + sla.AutoAction + "d",
		})
		if !sent {
			eventType := "auto_" + sla.AutoAction + "d"
			if sla.AutoAction == "approve" {
				executeAutoAction(ctx, queries, pool, companyID, entityType, entityID, "approve", logger)
			} else if sla.AutoAction == "reject" {
				executeAutoAction(ctx, queries, pool, companyID, entityType, entityID, "reject", logger)
			}
			queries.InsertSLAEvent(ctx, store.InsertSLAEventParams{
				CompanyID:  companyID,
				EntityType: entityType,
				EntityID:   entityID,
				EventType:  eventType,
			})
			notifyManagers(ctx, queries, logger, companyID,
				fmt.Sprintf("SLA Auto-%s: %s", sla.AutoAction, entityType),
				fmt.Sprintf("%s's %s #%d was auto-%sd after %d hours (SLA exceeded).",
					employeeName, entityType, entityID, sla.AutoAction, sla.AutoActionHours),
				entityType, entityID)
			actions++
			return actions
		}
	}

	// Escalation
	if ageHours >= float64(sla.EscalateAfterHours) {
		sent, _ := queries.HasSLAEventBeenSent(ctx, store.HasSLAEventBeenSentParams{
			CompanyID:  companyID,
			EntityType: entityType,
			EntityID:   entityID,
			EventType:  "escalated",
		})
		if !sent {
			queries.InsertSLAEvent(ctx, store.InsertSLAEventParams{
				CompanyID:  companyID,
				EntityType: entityType,
				EntityID:   entityID,
				EventType:  "escalated",
			})
			notifyAdmins(ctx, queries, logger, companyID,
				fmt.Sprintf("SLA Escalation: %s #%d", entityType, entityID),
				fmt.Sprintf("%s's %s has been pending for %.0f hours. Escalated for immediate attention.",
					employeeName, entityType, ageHours),
				entityType, entityID)
			actions++
		}
	}

	// Second reminder
	if ageHours >= float64(sla.SecondReminderHours) {
		sent, _ := queries.HasSLAEventBeenSent(ctx, store.HasSLAEventBeenSentParams{
			CompanyID:  companyID,
			EntityType: entityType,
			EntityID:   entityID,
			EventType:  "reminder_2",
		})
		if !sent {
			queries.InsertSLAEvent(ctx, store.InsertSLAEventParams{
				CompanyID:  companyID,
				EntityType: entityType,
				EntityID:   entityID,
				EventType:  "reminder_2",
			})
			notifyManagers(ctx, queries, logger, companyID,
				fmt.Sprintf("Urgent: %s pending %.0fh", entityType, ageHours),
				fmt.Sprintf("%s's %s has been pending for %.0f hours. Please review immediately.",
					employeeName, entityType, ageHours),
				entityType, entityID)
			actions++
		}
	}

	// First reminder
	if ageHours >= float64(sla.ReminderAfterHours) {
		sent, _ := queries.HasSLAEventBeenSent(ctx, store.HasSLAEventBeenSentParams{
			CompanyID:  companyID,
			EntityType: entityType,
			EntityID:   entityID,
			EventType:  "reminder_1",
		})
		if !sent {
			queries.InsertSLAEvent(ctx, store.InsertSLAEventParams{
				CompanyID:  companyID,
				EntityType: entityType,
				EntityID:   entityID,
				EventType:  "reminder_1",
			})
			notifyManagers(ctx, queries, logger, companyID,
				fmt.Sprintf("Reminder: %s pending %.0fh", entityType, ageHours),
				fmt.Sprintf("%s's %s has been pending for %.0f hours.",
					employeeName, entityType, ageHours),
				entityType, entityID)
			actions++
		}
	}

	return actions
}

func executeAutoAction(ctx context.Context, queries *store.Queries, pool *pgxpool.Pool, companyID int64, entityType string, entityID int64, action string, logger *slog.Logger) {
	if entityType == "leave_request" && action == "approve" {
		_, err := queries.ApproveLeaveRequest(ctx, store.ApproveLeaveRequestParams{
			ID:        entityID,
			CompanyID: companyID,
		})
		if err != nil {
			logger.Error("sla: auto-approve leave failed", "id", entityID, "error", err)
		}
	} else if entityType == "overtime_request" && action == "approve" {
		_, err := queries.ApproveOvertimeRequest(ctx, store.ApproveOvertimeRequestParams{
			ID:        entityID,
			CompanyID: companyID,
		})
		if err != nil {
			logger.Error("sla: auto-approve OT failed", "id", entityID, "error", err)
		}
	}
	// Reject actions could be added here similarly
}

func notifyManagers(ctx context.Context, queries *store.Queries, logger *slog.Logger, companyID int64, title, msg, entityType string, entityID int64) {
	managers, err := queries.ListManagerUsersByCompany(ctx, companyID)
	if err != nil || len(managers) == 0 {
		admins, err := queries.ListAdminUsersByCompany(ctx, companyID)
		if err != nil || len(admins) == 0 {
			return
		}
		for _, a := range admins {
			notification.Notify(ctx, queries, logger, companyID, a.ID, title, msg, "approval", &entityType, &entityID)
		}
		return
	}
	for _, mgr := range managers {
		notification.Notify(ctx, queries, logger, companyID, mgr.ID, title, msg, "approval", &entityType, &entityID)
	}
}

func notifyAdmins(ctx context.Context, queries *store.Queries, logger *slog.Logger, companyID int64, title, msg, entityType string, entityID int64) {
	admins, err := queries.ListAdminUsersByCompany(ctx, companyID)
	if err != nil || len(admins) == 0 {
		return
	}
	for _, a := range admins {
		notification.Notify(ctx, queries, logger, companyID, a.ID, title, msg, "approval", &entityType, &entityID)
	}
}
