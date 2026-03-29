package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/store"
)

func (r *ToolRegistry) toolQueryLeaveBalance(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	year := int32(time.Now().Year())
	if y, ok := input["year"].(float64); ok {
		year = int32(y)
	}

	// Get the employee linked to this user
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found for user: %w", err)
	}

	employeeID := emp.ID
	if eid, ok := input["employee_id"].(float64); ok {
		employeeID = int64(eid)
	}

	balances, err := r.queries.ListLeaveBalances(ctx, store.ListLeaveBalancesParams{
		CompanyID:  companyID,
		EmployeeID: employeeID,
		Year:       year,
	})
	if err != nil {
		return "", fmt.Errorf("query leave balances: %w", err)
	}

	return toJSON(balances)
}

func (r *ToolRegistry) toolListLeaveTypes(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	types, err := r.queries.ListLeaveTypes(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("list leave types: %w", err)
	}

	type leaveTypeResult struct {
		ID          int64  `json:"id"`
		Code        string `json:"code"`
		Name        string `json:"name"`
		IsPaid      bool   `json:"is_paid"`
		DefaultDays string `json:"default_days"`
	}

	results := make([]leaveTypeResult, len(types))
	for i, lt := range types {
		results[i] = leaveTypeResult{
			ID:          lt.ID,
			Code:        lt.Code,
			Name:        lt.Name,
			IsPaid:      lt.IsPaid,
			DefaultDays: numericToString(lt.DefaultDays),
		}
	}

	return toJSON(results)
}

func (r *ToolRegistry) toolCreateLeaveRequest(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	leaveTypeID, ok := input["leave_type_id"].(float64)
	if !ok || leaveTypeID <= 0 {
		return "", fmt.Errorf("leave_type_id is required")
	}

	startDateStr, _ := input["start_date"].(string)
	endDateStr, _ := input["end_date"].(string)
	daysFloat, _ := input["days"].(float64)

	if startDateStr == "" || endDateStr == "" || daysFloat <= 0 {
		return "", fmt.Errorf("start_date, end_date, and days are required")
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid start_date format, use YYYY-MM-DD")
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid end_date format, use YYYY-MM-DD")
	}

	if endDate.Before(startDate) {
		return "", fmt.Errorf("end_date must not be before start_date")
	}

	var days pgtype.Numeric
	_ = days.Scan(fmt.Sprintf("%.1f", daysFloat))

	var reason *string
	if r, ok := input["reason"].(string); ok && r != "" {
		reason = &r
	}

	req, err := r.queries.CreateLeaveRequest(ctx, store.CreateLeaveRequestParams{
		CompanyID:   companyID,
		EmployeeID:  emp.ID,
		LeaveTypeID: int64(leaveTypeID),
		StartDate:   startDate,
		EndDate:     endDate,
		Days:        days,
		Reason:      reason,
	})
	if err != nil {
		return "", fmt.Errorf("create leave request: %w", err)
	}

	return toJSON(map[string]any{
		"success":    true,
		"request_id": req.ID,
		"status":     req.Status,
		"start_date": startDateStr,
		"end_date":   endDateStr,
		"days":       daysFloat,
		"message":    "Leave request submitted successfully. It is now pending approval.",
	})
}

// leaveDefs returns tool definitions for leave-related tools.
func leaveDefs() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "query_leave_balance",
			Description: "Query the leave balance for the current user or a specific employee. Returns earned, used, carried, and remaining days per leave type for the current year.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_id": map[string]any{"type": "integer", "description": "Optional employee ID. Omit to query the current user's balance."},
					"year":        map[string]any{"type": "integer", "description": "Year to query. Defaults to current year."},
				},
			}),
		},
		{
			Name:        "list_leave_types",
			Description: "List all available leave types for the company (e.g., Vacation Leave, Sick Leave, Maternity Leave). Returns leave type IDs needed for create_leave_request.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "create_leave_request",
			Description: "Submit a leave request for the current user. You MUST call list_leave_types first to get the correct leave_type_id. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"leave_type_id": map[string]any{"type": "integer", "description": "Leave type ID (from list_leave_types)."},
					"start_date":    map[string]any{"type": "string", "description": "Start date in YYYY-MM-DD format."},
					"end_date":      map[string]any{"type": "string", "description": "End date in YYYY-MM-DD format."},
					"days":          map[string]any{"type": "number", "description": "Number of leave days (e.g., 1, 0.5, 2)."},
					"reason":        map[string]any{"type": "string", "description": "Optional reason for the leave request."},
				},
				"required": []string{"leave_type_id", "start_date", "end_date", "days"},
			}),
		},
		{
			Name:        "create_overtime_request",
			Description: "Submit an overtime request for the current user. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ot_date":  map[string]any{"type": "string", "description": "Overtime date in YYYY-MM-DD format."},
					"start_at": map[string]any{"type": "string", "description": "OT start time in HH:MM format (24h)."},
					"end_at":   map[string]any{"type": "string", "description": "OT end time in HH:MM format (24h)."},
					"hours":    map[string]any{"type": "number", "description": "Total OT hours (e.g., 2, 1.5)."},
					"ot_type":  map[string]any{"type": "string", "description": "OT type: regular, rest_day, holiday, special_holiday. Default: regular."},
					"reason":   map[string]any{"type": "string", "description": "Optional reason for overtime."},
				},
				"required": []string{"ot_date", "start_at", "end_at", "hours"},
			}),
		},
	}
}

// registerLeaveTools registers leave-related tool executors.
func (r *ToolRegistry) registerLeaveTools() {
	r.tools["query_leave_balance"] = r.toolQueryLeaveBalance
	r.tools["list_leave_types"] = r.toolListLeaveTypes
	r.tools["create_leave_request"] = r.toolCreateLeaveRequest
	r.tools["create_overtime_request"] = r.toolCreateOvertimeRequest
}
