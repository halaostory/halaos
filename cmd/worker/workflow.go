package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/notification"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/internal/workflow"
)

// processAutoApprovals evaluates pending requests against workflow rules
// and auto-approves/rejects matching ones. Runs hourly.
func processAutoApprovals(ctx context.Context, queries *store.Queries, pool *pgxpool.Pool, engine *workflow.Engine, logger *slog.Logger) {
	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("auto-approvals: failed to list companies", "error", err)
		return
	}

	totalProcessed := 0
	for _, company := range companies {
		count := processAutoApprovalsForCompany(ctx, queries, pool, engine, logger, company.ID)
		totalProcessed += count
	}

	if totalProcessed > 0 {
		logger.Info("auto-approvals completed", "total_processed", totalProcessed)
	}
}

func processAutoApprovalsForCompany(
	ctx context.Context,
	queries *store.Queries,
	pool *pgxpool.Pool,
	engine *workflow.Engine,
	logger *slog.Logger,
	companyID int64,
) int {
	processed := 0

	// Process pending leave requests
	leaveReqs, err := queries.ListPendingLeaveRequestsForAutoApproval(ctx, companyID)
	if err != nil {
		logger.Error("auto-approvals: failed to list pending leaves", "company_id", companyID, "error", err)
	} else {
		for _, req := range leaveReqs {
			result, err := engine.EvaluateLeaveRequest(ctx, companyID, req)
			if err != nil {
				logger.Error("auto-approvals: evaluation failed",
					"entity_type", "leave_request",
					"entity_id", req.ID,
					"error", err,
				)
				continue
			}

			if !result.Matched {
				continue
			}

			if result.Action == "auto_approved" {
				if err := executeLeaveAutoApproval(ctx, queries, companyID, req, result, logger); err != nil {
					logger.Error("auto-approvals: execution failed",
						"entity_type", "leave_request",
						"entity_id", req.ID,
						"error", err,
					)
					continue
				}
			}

			// Log execution
			reason := result.Reason
			if _, err := queries.InsertRuleExecution(ctx, store.InsertRuleExecutionParams{
				CompanyID:           companyID,
				RuleID:              result.RuleID,
				EntityType:          "leave_request",
				EntityID:            req.ID,
				Action:              result.Action,
				Reason:              &reason,
				EvaluatedConditions: result.Conditions,
			}); err != nil {
				logger.Error("auto-approvals: failed to log execution", "error", err)
			}

			processed++
		}
	}

	// Process pending OT requests
	otReqs, err := queries.ListPendingOTRequestsForAutoApproval(ctx, companyID)
	if err != nil {
		logger.Error("auto-approvals: failed to list pending OT", "company_id", companyID, "error", err)
	} else {
		for _, req := range otReqs {
			result, err := engine.EvaluateOTRequest(ctx, companyID, req)
			if err != nil {
				logger.Error("auto-approvals: OT evaluation failed",
					"entity_type", "overtime_request",
					"entity_id", req.ID,
					"error", err,
				)
				continue
			}

			if !result.Matched {
				continue
			}

			if result.Action == "auto_approved" {
				if err := executeOTAutoApproval(ctx, queries, companyID, req, result, logger); err != nil {
					logger.Error("auto-approvals: OT execution failed",
						"entity_type", "overtime_request",
						"entity_id", req.ID,
						"error", err,
					)
					continue
				}
			}

			reason := result.Reason
			if _, err := queries.InsertRuleExecution(ctx, store.InsertRuleExecutionParams{
				CompanyID:           companyID,
				RuleID:              result.RuleID,
				EntityType:          "overtime_request",
				EntityID:            req.ID,
				Action:              result.Action,
				Reason:              &reason,
				EvaluatedConditions: result.Conditions,
			}); err != nil {
				logger.Error("auto-approvals: failed to log OT execution", "error", err)
			}

			processed++
		}
	}

	return processed
}

func executeLeaveAutoApproval(
	ctx context.Context,
	queries *store.Queries,
	companyID int64,
	req store.ListPendingLeaveRequestsForAutoApprovalRow,
	result workflow.EvaluationResult,
	logger *slog.Logger,
) error {
	// Approve the leave request (no approver_id for auto-approval)
	_, err := queries.ApproveLeaveRequest(ctx, store.ApproveLeaveRequestParams{
		ID:        req.ID,
		CompanyID: companyID,
	})
	if err != nil {
		return fmt.Errorf("approve leave: %w", err)
	}

	// Deduct leave balance
	days := numericToFloat(req.Days)
	year := req.StartDate.Year()
	var deductNumeric pgtype.Numeric
	_ = deductNumeric.Scan(fmt.Sprintf("%f", days))
	_ = queries.DeductLeaveBalance(ctx, store.DeductLeaveBalanceParams{
		CompanyID:   companyID,
		EmployeeID:  req.EmployeeID,
		LeaveTypeID: req.LeaveTypeID,
		Year:        int32(year),
		Used:        deductNumeric,
	})

	// Notify employee
	entityType := "leave_request"
	empName := req.FirstName + " " + req.LastName
	title := "Leave Auto-Approved"
	msg := fmt.Sprintf("Your %s leave (%.0f day(s)) was auto-approved by rule: %s",
		req.LeaveTypeName, days, result.RuleName)

	// Find user_id for the employee
	employees, _ := queries.ListActiveEmployees(ctx, companyID)
	for _, emp := range employees {
		if emp.ID == req.EmployeeID && emp.UserID != nil {
			notification.Notify(ctx, queries, logger, companyID, *emp.UserID,
				title, msg, "approval", &entityType, &req.ID)
			break
		}
	}

	// Notify managers
	managers, _ := queries.ListManagerUsersByCompany(ctx, companyID)
	for _, mgr := range managers {
		mgrTitle := "Leave Auto-Approved"
		mgrMsg := fmt.Sprintf("%s's %s leave (%.0f day(s)) was auto-approved by rule: %s",
			empName, req.LeaveTypeName, days, result.RuleName)

		actionsJSON := []notification.NotificationAction{
			{Label: "View Leave", Route: "/leaves", Action: "view_leave"},
		}
		notification.Notify(ctx, queries, logger, companyID, mgr.ID,
			mgrTitle, mgrMsg, "approval", &entityType, &req.ID, actionsJSON)
	}

	logger.Info("leave auto-approved",
		"leave_request_id", req.ID,
		"employee", empName,
		"rule", result.RuleName,
	)

	return nil
}

func executeOTAutoApproval(
	ctx context.Context,
	queries *store.Queries,
	companyID int64,
	req store.ListPendingOTRequestsForAutoApprovalRow,
	result workflow.EvaluationResult,
	logger *slog.Logger,
) error {
	_, err := queries.ApproveOvertimeRequest(ctx, store.ApproveOvertimeRequestParams{
		ID:        req.ID,
		CompanyID: companyID,
	})
	if err != nil {
		return fmt.Errorf("approve OT: %w", err)
	}

	entityType := "overtime_request"
	empName := req.FirstName + " " + req.LastName
	hours := numericToFloat(req.Hours)

	employees, _ := queries.ListActiveEmployees(ctx, companyID)
	for _, emp := range employees {
		if emp.ID == req.EmployeeID && emp.UserID != nil {
			title := "Overtime Auto-Approved"
			msg := fmt.Sprintf("Your OT request (%.1f hours) was auto-approved by rule: %s",
				hours, result.RuleName)
			notification.Notify(ctx, queries, logger, companyID, *emp.UserID,
				title, msg, "approval", &entityType, &req.ID)
			break
		}
	}

	managers, _ := queries.ListManagerUsersByCompany(ctx, companyID)
	for _, mgr := range managers {
		title := "Overtime Auto-Approved"
		msg := fmt.Sprintf("%s's OT (%.1f hours) was auto-approved by rule: %s",
			empName, hours, result.RuleName)
		notification.Notify(ctx, queries, logger, companyID, mgr.ID,
			title, msg, "approval", &entityType, &req.ID)
	}

	logger.Info("OT auto-approved",
		"ot_request_id", req.ID,
		"employee", empName,
		"rule", result.RuleName,
	)

	return nil
}

