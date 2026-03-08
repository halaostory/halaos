package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/tonypk/aigonhr/internal/store"
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
