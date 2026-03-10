package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/notification"
	"github.com/tonypk/aigonhr/internal/store"
)

// getThresholds reads configurable thresholds from trigger action_config.
// Defaults: auto=0.90, recommend=0.70.
func getThresholds(trigger store.WorkflowTrigger) (autoThreshold, recommendThreshold float64) {
	autoThreshold = 0.90
	recommendThreshold = 0.70

	if len(trigger.ActionConfig) > 0 {
		var cfg struct {
			AutoThreshold      *float64 `json:"auto_threshold"`
			RecommendThreshold *float64 `json:"recommend_threshold"`
		}
		if err := json.Unmarshal(trigger.ActionConfig, &cfg); err == nil {
			if cfg.AutoThreshold != nil {
				autoThreshold = *cfg.AutoThreshold
			}
			if cfg.RecommendThreshold != nil {
				recommendThreshold = *cfg.RecommendThreshold
			}
		}
	}
	return
}

// routeDecision routes a leave request AI decision based on confidence thresholds.
func routeDecision(
	ctx context.Context,
	queries *store.Queries,
	pool *pgxpool.Pool,
	companyID int64,
	decision store.WorkflowDecision,
	output DecisionOutput,
	entityType string,
	entityID int64,
	req store.ListPendingLeaveRequestsForAutoApprovalRow,
	trigger store.WorkflowTrigger,
	logger *slog.Logger,
) error {
	autoThreshold, recommendThreshold := getThresholds(trigger)
	empName := req.FirstName + " " + req.LastName
	days := numericToFloat64(req.Days)

	switch {
	case output.Decision == "escalate" || output.Decision == "request_info":
		return notifyEscalation(ctx, queries, logger, companyID, decision, output, entityType, entityID, empName)

	case output.Confidence >= autoThreshold && output.Decision == "auto_approve":
		// Auto-execute approval
		result := EvaluationResult{
			RuleName: "AI Agent (auto)",
			Action:   "auto_approved",
			Reason:   output.Reasoning,
		}
		if err := executeLeaveAutoApprovalFromDispatcher(ctx, queries, pool, companyID, req, result, logger); err != nil {
			return fmt.Errorf("auto-approve execution: %w", err)
		}
		markExecuted(ctx, queries, decision.ID, "auto_approved")
		return nil

	case output.Confidence >= autoThreshold && output.Decision == "auto_reject":
		reason := output.Reasoning
		_, err := queries.RejectLeaveRequest(ctx, store.RejectLeaveRequestParams{
			ID:              entityID,
			CompanyID:       companyID,
			RejectionReason: &reason,
		})
		if err != nil {
			return fmt.Errorf("auto-reject execution: %w", err)
		}

		// Notify employee
		notifyEmployee(ctx, queries, logger, companyID, req.EmployeeID,
			"Leave Auto-Rejected",
			fmt.Sprintf("Your %s leave (%.0f day(s)) was auto-rejected. Reason: %s",
				req.LeaveTypeName, days, output.Reasoning),
			entityType, entityID,
		)
		markExecuted(ctx, queries, decision.ID, "auto_rejected")
		return nil

	case output.Confidence >= recommendThreshold:
		// Recommend to manager
		return notifyManagerRecommendation(ctx, queries, logger, companyID, decision, output, entityType, entityID, empName, req.LeaveTypeName, days)

	default:
		// Low confidence — escalate
		return notifyEscalation(ctx, queries, logger, companyID, decision, output, entityType, entityID, empName)
	}
}

// routeOTDecision routes an OT request AI decision.
func routeOTDecision(
	ctx context.Context,
	queries *store.Queries,
	companyID int64,
	decision store.WorkflowDecision,
	output DecisionOutput,
	req store.ListPendingOTRequestsForAutoApprovalRow,
	trigger store.WorkflowTrigger,
	logger *slog.Logger,
) error {
	autoThreshold, recommendThreshold := getThresholds(trigger)
	empName := req.FirstName + " " + req.LastName
	hours := numericToFloat64(req.Hours)
	entityType := "overtime_request"

	switch {
	case output.Decision == "escalate" || output.Decision == "request_info":
		return notifyEscalation(ctx, queries, logger, companyID, decision, output, entityType, req.ID, empName)

	case output.Confidence >= autoThreshold && output.Decision == "auto_approve":
		_, err := queries.ApproveOvertimeRequest(ctx, store.ApproveOvertimeRequestParams{
			ID:        req.ID,
			CompanyID: companyID,
		})
		if err != nil {
			return fmt.Errorf("auto-approve OT: %w", err)
		}

		notifyEmployee(ctx, queries, logger, companyID, req.EmployeeID,
			"OT Auto-Approved",
			fmt.Sprintf("Your OT request (%.1f hours) was auto-approved by AI. Reason: %s",
				hours, output.Reasoning),
			entityType, req.ID,
		)
		markExecuted(ctx, queries, decision.ID, "auto_approved")
		return nil

	case output.Confidence >= autoThreshold && output.Decision == "auto_reject":
		reason := output.Reasoning
		_, err := queries.RejectOvertimeRequest(ctx, store.RejectOvertimeRequestParams{
			ID:              req.ID,
			CompanyID:       companyID,
			RejectionReason: &reason,
		})
		if err != nil {
			return fmt.Errorf("auto-reject OT: %w", err)
		}

		notifyEmployee(ctx, queries, logger, companyID, req.EmployeeID,
			"OT Auto-Rejected",
			fmt.Sprintf("Your OT request (%.1f hours) was auto-rejected. Reason: %s",
				hours, output.Reasoning),
			entityType, req.ID,
		)
		markExecuted(ctx, queries, decision.ID, "auto_rejected")
		return nil

	case output.Confidence >= recommendThreshold:
		return notifyManagerRecommendation(ctx, queries, logger, companyID, decision, output,
			entityType, req.ID, empName, "OT", hours)

	default:
		return notifyEscalation(ctx, queries, logger, companyID, decision, output, entityType, req.ID, empName)
	}
}

func notifyManagerRecommendation(
	ctx context.Context,
	queries *store.Queries,
	logger *slog.Logger,
	companyID int64,
	decision store.WorkflowDecision,
	output DecisionOutput,
	entityType string,
	entityID int64,
	empName, typeName string,
	amount float64,
) error {
	actionLabel := "Approve"
	if output.Decision == "recommend_reject" {
		actionLabel = "Reject"
	}

	title := fmt.Sprintf("AI Recommends %s", actionLabel)
	msg := fmt.Sprintf("%s's %s (%.1f) — AI recommends %s (%.0f%% confidence). Reason: %s",
		empName, typeName, amount, actionLabel, output.Confidence*100, output.Reasoning)

	actions := []notification.NotificationAction{
		{
			Label:  "Quick Approve",
			Action: "quick_approve",
			Params: map[string]any{
				"entity_type": entityType,
				"entity_id":   entityID,
				"decision_id": decision.ID,
			},
		},
		{
			Label: "Review",
			Route: fmt.Sprintf("/workflow-decisions?entity_type=%s&entity_id=%d", entityType, entityID),
		},
	}

	managers, _ := queries.ListManagerUsersByCompany(ctx, companyID)
	for _, mgr := range managers {
		notification.Notify(ctx, queries, logger, companyID, mgr.ID,
			title, msg, "approval", &entityType, &entityID, actions)
	}
	return nil
}

func notifyEscalation(
	ctx context.Context,
	queries *store.Queries,
	logger *slog.Logger,
	companyID int64,
	decision store.WorkflowDecision,
	output DecisionOutput,
	entityType string,
	entityID int64,
	empName string,
) error {
	title := "Request Needs Review"
	msg := fmt.Sprintf("%s's %s request requires manual review. AI confidence: %.0f%%. Reason: %s",
		empName, entityType, output.Confidence*100, output.Reasoning)

	actions := []notification.NotificationAction{
		{
			Label: "Review Request",
			Route: fmt.Sprintf("/workflow-decisions?entity_type=%s&entity_id=%d", entityType, entityID),
		},
	}

	admins, _ := queries.ListAdminUsersByCompany(ctx, companyID)
	for _, admin := range admins {
		notification.Notify(ctx, queries, logger, companyID, admin.ID,
			title, msg, "approval", &entityType, &entityID, actions)
	}
	return nil
}

func notifyEmployee(
	ctx context.Context,
	queries *store.Queries,
	logger *slog.Logger,
	companyID int64,
	employeeID int64,
	title, msg string,
	entityType string,
	entityID int64,
) {
	employees, _ := queries.ListActiveEmployees(ctx, companyID)
	for _, emp := range employees {
		if emp.ID == employeeID && emp.UserID != nil {
			notification.Notify(ctx, queries, logger, companyID, *emp.UserID,
				title, msg, "approval", &entityType, &entityID)
			break
		}
	}
}

func markExecuted(ctx context.Context, queries *store.Queries, decisionID int64, result string) {
	resultJSON, _ := json.Marshal(map[string]string{"action": result})
	_ = queries.MarkDecisionExecuted(ctx, store.MarkDecisionExecutedParams{
		ID:              decisionID,
		ExecutionResult: resultJSON,
	})
}

// executeLeaveAutoApprovalFromDispatcher handles leave auto-approval logic
// from the event-driven dispatcher (reuses same logic as hourly worker).
func executeLeaveAutoApprovalFromDispatcher(
	ctx context.Context,
	queries *store.Queries,
	pool *pgxpool.Pool,
	companyID int64,
	req store.ListPendingLeaveRequestsForAutoApprovalRow,
	result EvaluationResult,
	logger *slog.Logger,
) error {
	_, err := queries.ApproveLeaveRequest(ctx, store.ApproveLeaveRequestParams{
		ID:        req.ID,
		CompanyID: companyID,
	})
	if err != nil {
		return fmt.Errorf("approve leave: %w", err)
	}

	// Deduct balance
	days := numericToFloat64(req.Days)
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

	entityType := "leave_request"
	empName := req.FirstName + " " + req.LastName
	title := "Leave Auto-Approved"
	msg := fmt.Sprintf("Your %s leave (%.0f day(s)) was auto-approved: %s",
		req.LeaveTypeName, days, result.RuleName)

	// Notify employee
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
		mgrMsg := fmt.Sprintf("%s's %s leave (%.0f day(s)) was auto-approved: %s",
			empName, req.LeaveTypeName, days, result.RuleName)
		notification.Notify(ctx, queries, logger, companyID, mgr.ID,
			"Leave Auto-Approved", mgrMsg, "approval", &entityType, &req.ID)
	}

	logger.Info("leave auto-approved (dispatcher)",
		"leave_request_id", req.ID,
		"employee", empName,
		"source", result.RuleName,
	)
	return nil
}

// executeOTAutoApprovalFromDispatcher handles OT auto-approval from dispatcher.
func executeOTAutoApprovalFromDispatcher(
	ctx context.Context,
	queries *store.Queries,
	companyID int64,
	req store.ListPendingOTRequestsForAutoApprovalRow,
	result EvaluationResult,
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
	hours := numericToFloat64(req.Hours)

	employees, _ := queries.ListActiveEmployees(ctx, companyID)
	for _, emp := range employees {
		if emp.ID == req.EmployeeID && emp.UserID != nil {
			notification.Notify(ctx, queries, logger, companyID, *emp.UserID,
				"OT Auto-Approved",
				fmt.Sprintf("Your OT (%.1f hours) was auto-approved: %s", hours, result.RuleName),
				"approval", &entityType, &req.ID)
			break
		}
	}

	managers, _ := queries.ListManagerUsersByCompany(ctx, companyID)
	for _, mgr := range managers {
		notification.Notify(ctx, queries, logger, companyID, mgr.ID,
			"OT Auto-Approved",
			fmt.Sprintf("%s's OT (%.1f hours) was auto-approved: %s", empName, hours, result.RuleName),
			"approval", &entityType, &req.ID)
	}

	logger.Info("OT auto-approved (dispatcher)",
		"ot_request_id", req.ID,
		"employee", empName,
		"source", result.RuleName,
	)
	return nil
}
