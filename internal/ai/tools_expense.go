package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/tonypk/aigonhr/internal/store"
)

func (r *ToolRegistry) toolListExpenseCategories(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	categories, err := r.queries.ListActiveExpenseCategories(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("list expense categories: %w", err)
	}

	type categoryResult struct {
		ID              int64  `json:"id"`
		Name            string `json:"name"`
		Description     string `json:"description,omitempty"`
		MaxAmount       string `json:"max_amount,omitempty"`
		RequiresReceipt bool   `json:"requires_receipt"`
	}

	results := make([]categoryResult, len(categories))
	for i, c := range categories {
		desc := ""
		if c.Description != nil {
			desc = *c.Description
		}
		results[i] = categoryResult{
			ID:              c.ID,
			Name:            c.Name,
			Description:     desc,
			MaxAmount:       numericToString(c.MaxAmount),
			RequiresReceipt: c.RequiresReceipt,
		}
	}

	return toJSON(results)
}

func (r *ToolRegistry) toolCreateExpenseClaim(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	categoryID, ok := input["category_id"].(float64)
	if !ok || categoryID <= 0 {
		return "", fmt.Errorf("category_id is required")
	}

	description, _ := input["description"].(string)
	if description == "" {
		return "", fmt.Errorf("description is required")
	}

	amountFloat, ok := input["amount"].(float64)
	if !ok || amountFloat <= 0 {
		return "", fmt.Errorf("amount must be greater than 0")
	}

	expenseDateStr, _ := input["expense_date"].(string)
	if expenseDateStr == "" {
		return "", fmt.Errorf("expense_date is required")
	}
	expenseDate, err := time.Parse("2006-01-02", expenseDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid expense_date format, use YYYY-MM-DD")
	}

	var amount pgtype.Numeric
	_ = amount.Scan(fmt.Sprintf("%.2f", amountFloat))

	// Generate claim number
	nextNum, err := r.queries.NextExpenseClaimNumber(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("generate claim number: %w", err)
	}
	claimNumber := fmt.Sprintf("EXP-%05d", nextNum)

	var notes *string
	if n, ok := input["notes"].(string); ok && n != "" {
		notes = &n
	}

	claim, err := r.queries.CreateExpenseClaim(ctx, store.CreateExpenseClaimParams{
		CompanyID:   companyID,
		EmployeeID:  emp.ID,
		ClaimNumber: claimNumber,
		CategoryID:  int64(categoryID),
		Description: description,
		Amount:      amount,
		Currency:    "PHP",
		ExpenseDate: expenseDate,
		Status:      "submitted",
		Notes:       notes,
	})
	if err != nil {
		return "", fmt.Errorf("create expense claim: %w", err)
	}

	return toJSON(map[string]any{
		"success":      true,
		"claim_id":     claim.ID,
		"claim_number": claim.ClaimNumber,
		"status":       claim.Status,
		"amount":       amountFloat,
		"message":      fmt.Sprintf("Expense claim %s submitted successfully for ₱%.2f.", claimNumber, amountFloat),
	})
}

func (r *ToolRegistry) toolCreateOvertimeRequest(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	otDateStr, _ := input["ot_date"].(string)
	startAtStr, _ := input["start_at"].(string)
	endAtStr, _ := input["end_at"].(string)
	hoursFloat, _ := input["hours"].(float64)

	if otDateStr == "" || startAtStr == "" || endAtStr == "" || hoursFloat <= 0 {
		return "", fmt.Errorf("ot_date, start_at, end_at, and hours are required")
	}

	otDate, err := time.Parse("2006-01-02", otDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid ot_date format, use YYYY-MM-DD")
	}

	startAt, err := time.Parse("2006-01-02 15:04", otDateStr+" "+startAtStr)
	if err != nil {
		return "", fmt.Errorf("invalid start_at format, use HH:MM")
	}
	endAt, err := time.Parse("2006-01-02 15:04", otDateStr+" "+endAtStr)
	if err != nil {
		return "", fmt.Errorf("invalid end_at format, use HH:MM")
	}

	if !endAt.After(startAt) {
		return "", fmt.Errorf("end_at must be after start_at")
	}

	var hours pgtype.Numeric
	_ = hours.Scan(fmt.Sprintf("%.1f", hoursFloat))

	otType := "regular"
	if t, ok := input["ot_type"].(string); ok && t != "" {
		otType = t
	}

	var reason *string
	if r, ok := input["reason"].(string); ok && r != "" {
		reason = &r
	}

	req, err := r.queries.CreateOvertimeRequest(ctx, store.CreateOvertimeRequestParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
		OtDate:     otDate,
		StartAt:    startAt,
		EndAt:      endAt,
		Hours:      hours,
		OtType:     otType,
		Reason:     reason,
	})
	if err != nil {
		return "", fmt.Errorf("create overtime request: %w", err)
	}

	return toJSON(map[string]any{
		"success":    true,
		"request_id": req.ID,
		"status":     req.Status,
		"ot_date":    otDateStr,
		"hours":      hoursFloat,
		"ot_type":    otType,
		"message":    "Overtime request submitted successfully. It is now pending approval.",
	})
}
