package ai

import (
	"context"
	"fmt"

	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/store"
)

func (r *ToolRegistry) toolListPendingApprovals(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	// Check user role and company scope
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if user.CompanyID != companyID {
		return "", fmt.Errorf("access denied")
	}
	if user.Role != "admin" && user.Role != "manager" {
		return "", fmt.Errorf("only managers and admins can view pending approvals")
	}

	leaveApprovals, err := r.queries.ListPendingLeaveApprovals(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("list pending leave approvals: %w", err)
	}

	overtimeApprovals, err := r.queries.ListPendingOvertimeApprovals(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("list pending overtime approvals: %w", err)
	}

	type approvalItem struct {
		ID           int64  `json:"id"`
		Type         string `json:"type"`
		EmployeeName string `json:"employee_name"`
		Details      any    `json:"details"`
	}

	items := make([]approvalItem, 0, len(leaveApprovals)+len(overtimeApprovals))

	for _, la := range leaveApprovals {
		items = append(items, approvalItem{
			ID:           la.ID,
			Type:         "leave",
			EmployeeName: fmt.Sprintf("%v", la.EmployeeName),
			Details: map[string]any{
				"leave_type": la.LeaveTypeName,
				"start_date": la.StartDate.Format("2006-01-02"),
				"end_date":   la.EndDate.Format("2006-01-02"),
				"days":       numericToString(la.Days),
			},
		})
	}

	for _, oa := range overtimeApprovals {
		items = append(items, approvalItem{
			ID:           oa.ID,
			Type:         "overtime",
			EmployeeName: fmt.Sprintf("%v", oa.EmployeeName),
			Details: map[string]any{
				"ot_date": oa.OtDate.Format("2006-01-02"),
				"hours":   numericToString(oa.Hours),
				"ot_type": oa.OtType,
			},
		})
	}

	return toJSON(map[string]any{
		"total_pending": len(items),
		"items":         items,
	})
}

func (r *ToolRegistry) toolApproveLeaveRequest(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if user.CompanyID != companyID {
		return "", fmt.Errorf("access denied")
	}
	if user.Role != "admin" && user.Role != "manager" {
		return "", fmt.Errorf("only managers and admins can approve leave requests")
	}

	requestID, ok := input["request_id"].(float64)
	if !ok || requestID <= 0 {
		return "", fmt.Errorf("request_id is required")
	}

	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("approver employee not found: %w", err)
	}

	req, err := r.queries.ApproveLeaveRequest(ctx, store.ApproveLeaveRequestParams{
		ID:         int64(requestID),
		CompanyID:  companyID,
		ApproverID: &emp.ID,
	})
	if err != nil {
		return "", fmt.Errorf("approve leave request: %w", err)
	}

	return toJSON(map[string]any{
		"success":    true,
		"request_id": req.ID,
		"status":     req.Status,
		"message":    "Leave request approved successfully.",
	})
}

func (r *ToolRegistry) toolApproveOvertimeRequest(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if user.CompanyID != companyID {
		return "", fmt.Errorf("access denied")
	}
	if user.Role != "admin" && user.Role != "manager" {
		return "", fmt.Errorf("only managers and admins can approve overtime requests")
	}

	requestID, ok := input["request_id"].(float64)
	if !ok || requestID <= 0 {
		return "", fmt.Errorf("request_id is required")
	}

	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("approver employee not found: %w", err)
	}

	req, err := r.queries.ApproveOvertimeRequest(ctx, store.ApproveOvertimeRequestParams{
		ID:         int64(requestID),
		CompanyID:  companyID,
		ApproverID: &emp.ID,
	})
	if err != nil {
		return "", fmt.Errorf("approve overtime request: %w", err)
	}

	return toJSON(map[string]any{
		"success":    true,
		"request_id": req.ID,
		"status":     req.Status,
		"message":    "Overtime request approved successfully.",
	})
}

func (r *ToolRegistry) toolRejectLeaveRequest(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if user.CompanyID != companyID {
		return "", fmt.Errorf("access denied")
	}
	if user.Role != "admin" && user.Role != "manager" {
		return "", fmt.Errorf("only managers and admins can reject leave requests")
	}

	requestID, ok := input["request_id"].(float64)
	if !ok || requestID <= 0 {
		return "", fmt.Errorf("request_id is required")
	}

	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("approver employee not found: %w", err)
	}

	var reason *string
	if r, ok := input["reason"].(string); ok && r != "" {
		reason = &r
	}

	req, err := r.queries.RejectLeaveRequest(ctx, store.RejectLeaveRequestParams{
		ID:              int64(requestID),
		CompanyID:       companyID,
		ApproverID:      &emp.ID,
		RejectionReason: reason,
	})
	if err != nil {
		return "", fmt.Errorf("reject leave request: %w", err)
	}

	return toJSON(map[string]any{
		"success":    true,
		"request_id": req.ID,
		"status":     req.Status,
		"message":    "Leave request rejected.",
	})
}

func (r *ToolRegistry) toolRejectOvertimeRequest(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if user.CompanyID != companyID {
		return "", fmt.Errorf("access denied")
	}
	if user.Role != "admin" && user.Role != "manager" {
		return "", fmt.Errorf("only managers and admins can reject overtime requests")
	}

	requestID, ok := input["request_id"].(float64)
	if !ok || requestID <= 0 {
		return "", fmt.Errorf("request_id is required")
	}

	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("approver employee not found: %w", err)
	}

	var reason *string
	if r, ok := input["reason"].(string); ok && r != "" {
		reason = &r
	}

	req, err := r.queries.RejectOvertimeRequest(ctx, store.RejectOvertimeRequestParams{
		ID:              int64(requestID),
		CompanyID:       companyID,
		ApproverID:      &emp.ID,
		RejectionReason: reason,
	})
	if err != nil {
		return "", fmt.Errorf("reject overtime request: %w", err)
	}

	return toJSON(map[string]any{
		"success":    true,
		"request_id": req.ID,
		"status":     req.Status,
		"message":    "Overtime request rejected.",
	})
}

func (r *ToolRegistry) toolGetApprovalContext(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if user.CompanyID != companyID {
		return "", fmt.Errorf("access denied")
	}
	if user.Role != "admin" && user.Role != "manager" {
		return "", fmt.Errorf("only managers and admins can view approval context")
	}

	entityType, _ := input["entity_type"].(string)
	requestID, _ := input["request_id"].(float64)
	if entityType == "" || requestID <= 0 {
		return "", fmt.Errorf("entity_type and request_id are required")
	}

	switch entityType {
	case "leave_request", "leave":
		lr, err := r.queries.GetLeaveRequest(ctx, store.GetLeaveRequestParams{
			ID:        int64(requestID),
			CompanyID: companyID,
		})
		if err != nil {
			return "", fmt.Errorf("leave request not found: %w", err)
		}

		emp, err := r.queries.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{
			ID:        lr.EmployeeID,
			CompanyID: companyID,
		})
		if err != nil {
			return "", fmt.Errorf("employee not found: %w", err)
		}

		balances, err := r.queries.ListLeaveBalances(ctx, store.ListLeaveBalancesParams{
			CompanyID:  companyID,
			EmployeeID: lr.EmployeeID,
			Year:       int32(lr.StartDate.Year()),
		})
		balanceSummary := make([]map[string]any, 0)
		if err == nil {
			for _, b := range balances {
				earned := numericToFloat(b.Earned)
				used := numericToFloat(b.Used)
				carried := numericToFloat(b.Carried)
				adjusted := numericToFloat(b.Adjusted)
				balanceSummary = append(balanceSummary, map[string]any{
					"leave_type": b.LeaveTypeName,
					"remaining":  earned + carried + adjusted - used,
				})
			}
		}

		return toJSON(map[string]any{
			"request": map[string]any{
				"id":         lr.ID,
				"start_date": lr.StartDate.Format("2006-01-02"),
				"end_date":   lr.EndDate.Format("2006-01-02"),
				"days":       numericToString(lr.Days),
				"status":     lr.Status,
				"reason":     lr.Reason,
			},
			"employee": map[string]any{
				"id":        emp.ID,
				"name":      emp.FirstName + " " + emp.LastName,
				"hire_date": emp.HireDate.Format("2006-01-02"),
				"status":    emp.Status,
			},
			"leave_balances": balanceSummary,
		})

	case "overtime_request", "overtime":
		ot, err := r.queries.GetOvertimeRequest(ctx, store.GetOvertimeRequestParams{
			ID:        int64(requestID),
			CompanyID: companyID,
		})
		if err != nil {
			return "", fmt.Errorf("overtime request not found: %w", err)
		}

		emp, err := r.queries.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{
			ID:        ot.EmployeeID,
			CompanyID: companyID,
		})
		if err != nil {
			return "", fmt.Errorf("employee not found: %w", err)
		}

		return toJSON(map[string]any{
			"request": map[string]any{
				"id":      ot.ID,
				"ot_date": ot.OtDate.Format("2006-01-02"),
				"hours":   numericToString(ot.Hours),
				"ot_type": ot.OtType,
				"status":  ot.Status,
				"reason":  ot.Reason,
			},
			"employee": map[string]any{
				"id":        emp.ID,
				"name":      emp.FirstName + " " + emp.LastName,
				"hire_date": emp.HireDate.Format("2006-01-02"),
				"status":    emp.Status,
			},
		})

	default:
		return "", fmt.Errorf("unsupported entity type: %s", entityType)
	}
}

// approvalDefs returns tool definitions for approval-related tools.
func approvalDefs() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "list_pending_approvals",
			Description: "List all pending leave and overtime requests awaiting approval. Manager/Admin only. Returns request IDs needed for approve_leave_request and approve_overtime_request.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "approve_leave_request",
			Description: "Approve a pending leave request. Manager/Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"request_id": map[string]any{"type": "integer", "description": "Leave request ID (from list_pending_approvals)."},
				},
				"required": []string{"request_id"},
			}),
		},
		{
			Name:        "approve_overtime_request",
			Description: "Approve a pending overtime request. Manager/Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"request_id": map[string]any{"type": "integer", "description": "Overtime request ID (from list_pending_approvals)."},
				},
				"required": []string{"request_id"},
			}),
		},
		{
			Name:        "reject_leave_request",
			Description: "Reject a pending leave request with a reason. Manager/Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"request_id": map[string]any{"type": "integer", "description": "Leave request ID (from list_pending_approvals)."},
					"reason":     map[string]any{"type": "string", "description": "Reason for rejecting the leave request."},
				},
				"required": []string{"request_id"},
			}),
		},
		{
			Name:        "reject_overtime_request",
			Description: "Reject a pending overtime request with a reason. Manager/Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"request_id": map[string]any{"type": "integer", "description": "Overtime request ID (from list_pending_approvals)."},
					"reason":     map[string]any{"type": "string", "description": "Reason for rejecting the overtime request."},
				},
				"required": []string{"request_id"},
			}),
		},
		{
			Name:        "get_approval_context",
			Description: "Get detailed context for a pending approval request, including employee info, leave balance, and team conflicts. Manager/Admin only.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"entity_type": map[string]any{"type": "string", "description": "Type: leave_request or overtime_request", "enum": []string{"leave_request", "overtime_request"}},
					"request_id":  map[string]any{"type": "integer", "description": "The request ID to get context for."},
				},
				"required": []string{"entity_type", "request_id"},
			}),
		},
	}
}

// registerApprovalTools registers approval-related tool executors.
func (r *ToolRegistry) registerApprovalTools() {
	r.tools["list_pending_approvals"] = r.toolListPendingApprovals
	r.tools["approve_leave_request"] = r.toolApproveLeaveRequest
	r.tools["approve_overtime_request"] = r.toolApproveOvertimeRequest
	r.tools["reject_leave_request"] = r.toolRejectLeaveRequest
	r.tools["reject_overtime_request"] = r.toolRejectOvertimeRequest
	r.tools["get_approval_context"] = r.toolGetApprovalContext
}
