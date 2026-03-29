package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/numericutil"
)

func (r *ToolRegistry) toolGetClearanceStatus(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	clearanceID, ok := input["clearance_id"].(float64)
	if !ok || clearanceID <= 0 {
		return "", fmt.Errorf("clearance_id is required")
	}

	cr, err := r.queries.GetClearanceRequest(ctx, store.GetClearanceRequestParams{
		ID:        int64(clearanceID),
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("clearance request not found: %w", err)
	}

	items, _ := r.queries.ListClearanceItems(ctx, store.ListClearanceItemsParams{
		ClearanceID: int64(clearanceID),
		CompanyID:   companyID,
	})
	statusCounts, _ := r.queries.CountClearanceItemsByStatus(ctx, store.CountClearanceItemsByStatusParams{
		ClearanceID: int64(clearanceID),
		CompanyID:   companyID,
	})

	var totalItems, clearedItems int64
	for _, sc := range statusCounts {
		totalItems += sc.Count
		if sc.Status == "cleared" {
			clearedItems = sc.Count
		}
	}

	type itemResult struct {
		ID         int64  `json:"id"`
		Department string `json:"department"`
		ItemName   string `json:"item_name"`
		Status     string `json:"status"`
		Remarks    string `json:"remarks,omitempty"`
	}
	itemResults := make([]itemResult, len(items))
	for i, it := range items {
		ir := itemResult{
			ID:         it.ID,
			Department: it.Department,
			ItemName:   it.ItemName,
			Status:     it.Status,
		}
		if it.Remarks != nil {
			ir.Remarks = *it.Remarks
		}
		itemResults[i] = ir
	}

	return toJSON(map[string]any{
		"clearance_id":     cr.ID,
		"employee_name":    fmt.Sprintf("%v", cr.EmployeeName),
		"employee_no":      cr.EmployeeNo,
		"status":           cr.Status,
		"resignation_date": cr.ResignationDate.Format("2006-01-02"),
		"last_working_day": cr.LastWorkingDay.Format("2006-01-02"),
		"total_items":      totalItems,
		"cleared_items":    clearedItems,
		"progress":         fmt.Sprintf("%d/%d items cleared", clearedItems, totalItems),
		"items":            itemResults,
	})
}

func (r *ToolRegistry) toolUpdateClearanceItem(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}

	itemID, ok := input["item_id"].(float64)
	if !ok || itemID <= 0 {
		return "", fmt.Errorf("item_id is required")
	}

	status := "cleared"
	if s, ok := input["status"].(string); ok && s != "" {
		status = s
	}

	var remarks *string
	if rm, ok := input["remarks"].(string); ok && rm != "" {
		remarks = &rm
	}

	item, err := r.queries.UpdateClearanceItem(ctx, store.UpdateClearanceItemParams{
		ID:        int64(itemID),
		Status:    status,
		ClearedBy: &userID,
		Remarks:   remarks,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("update clearance item: %w", err)
	}

	return toJSON(map[string]any{
		"success": true,
		"item_id": item.ID,
		"status":  item.Status,
		"message": fmt.Sprintf("Clearance item '%s' marked as %s.", item.ItemName, status),
	})
}

func (r *ToolRegistry) toolQueryFinalPayComponents(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}

	employeeID, ok := input["employee_id"].(float64)
	if !ok || employeeID <= 0 {
		return "", fmt.Errorf("employee_id is required")
	}

	separationDateStr, _ := input["separation_date"].(string)
	if separationDateStr == "" {
		separationDateStr = time.Now().Format("2006-01-02")
	}
	separationDate, err := time.Parse("2006-01-02", separationDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid separation_date format")
	}

	empID := int64(employeeID)

	// 1. Get current salary
	salary, err := r.queries.GetCurrentSalary(ctx, store.GetCurrentSalaryParams{
		CompanyID:     companyID,
		EmployeeID:    empID,
		EffectiveFrom: separationDate,
	})
	if err != nil {
		return "", fmt.Errorf("salary not found for employee: %w", err)
	}

	basicSalary := numericutil.ToFloat(salary.BasicSalary)
	dailyRate := basicSalary / 22

	// 2. Unpaid salary (estimate days from last payroll)
	unpaidDays := 15.0
	if d, ok := input["unpaid_days"].(float64); ok && d >= 0 {
		unpaidDays = d
	}
	unpaidSalary := dailyRate * unpaidDays

	// 3. Leave encashment
	year := int32(separationDate.Year())
	convertibleBalances, _ := r.queries.GetConvertibleLeaveBalances(ctx, store.GetConvertibleLeaveBalancesParams{
		CompanyID:  companyID,
		EmployeeID: empID,
		Year:       year,
	})
	var leaveEncashment float64
	var leaveDays int32
	for _, b := range convertibleBalances {
		leaveEncashment += float64(b.Remaining) * dailyRate
		leaveDays += b.Remaining
	}

	// 4. 13th Month Pay (pro-rated)
	monthsWorked := float64(separationDate.Month())
	thirteenthMonth := basicSalary * monthsWorked / 12

	// 5. Outstanding loans
	activeLoans, _ := r.queries.GetEmployeeActiveLoanSummary(ctx, empID)
	var totalLoanDeductions float64
	for _, l := range activeLoans {
		totalLoanDeductions += numericutil.ToFloat(l.RemainingBalance)
	}

	netFinalPay := unpaidSalary + leaveEncashment + thirteenthMonth - totalLoanDeductions

	return toJSON(map[string]any{
		"employee_id":     empID,
		"separation_date": separationDateStr,
		"basic_salary":    basicSalary,
		"daily_rate":      dailyRate,
		"components": map[string]any{
			"unpaid_salary":      unpaidSalary,
			"unpaid_days":        unpaidDays,
			"leave_encashment":   leaveEncashment,
			"leave_days":         leaveDays,
			"13th_month_prorate": thirteenthMonth,
			"months_worked":      monthsWorked,
		},
		"deductions": map[string]any{
			"outstanding_loans": totalLoanDeductions,
			"active_loan_count": len(activeLoans),
		},
		"net_final_pay": netFinalPay,
	})
}

func (r *ToolRegistry) toolCreateFinalPay(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}

	employeeID, ok := input["employee_id"].(float64)
	if !ok || employeeID <= 0 {
		return "", fmt.Errorf("employee_id is required")
	}

	amount, ok := input["amount"].(float64)
	if !ok || amount <= 0 {
		return "", fmt.Errorf("amount is required")
	}

	// Create a final_pay payroll cycle
	var notes *string
	if n, ok := input["notes"].(string); ok && n != "" {
		notes = &n
	}

	notesStr := "Final pay created via AI assistant"
	if notes != nil {
		notesStr = *notes
	}

	var cycleID int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO payroll_cycles (company_id, name, cycle_type, start_date, end_date, pay_date, status, notes, created_by)
		VALUES ($1, $2, 'final_pay', $3, $4, $5, 'draft', $6, $7)
		RETURNING id
	`, companyID,
		fmt.Sprintf("Final Pay - Employee %d", int64(employeeID)),
		time.Now(), time.Now(), time.Now(),
		notesStr, userID).Scan(&cycleID)
	if err != nil {
		return "", fmt.Errorf("create final pay cycle: %w", err)
	}

	return toJSON(map[string]any{
		"success":  true,
		"cycle_id": cycleID,
		"amount":   amount,
		"message":  "Final pay record created as draft. Please review and approve via the Payroll page.",
	})
}

func (r *ToolRegistry) toolCompleteClearance(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}

	clearanceID, ok := input["clearance_id"].(float64)
	if !ok || clearanceID <= 0 {
		return "", fmt.Errorf("clearance_id is required")
	}

	// Check all items are cleared
	statusCounts, err := r.queries.CountClearanceItemsByStatus(ctx, store.CountClearanceItemsByStatusParams{
		ClearanceID: int64(clearanceID),
		CompanyID:   companyID,
	})
	if err != nil {
		return "", fmt.Errorf("check clearance items: %w", err)
	}

	for _, sc := range statusCounts {
		if sc.Status == "pending" && sc.Count > 0 {
			return toJSON(map[string]any{
				"success": false,
				"message": fmt.Sprintf("Cannot complete: %d items still pending.", sc.Count),
			})
		}
	}

	cr, err := r.queries.UpdateClearanceStatus(ctx, store.UpdateClearanceStatusParams{
		ID:        int64(clearanceID),
		CompanyID: companyID,
		Status:    "completed",
	})
	if err != nil {
		return "", fmt.Errorf("complete clearance: %w", err)
	}

	return toJSON(map[string]any{
		"success":      true,
		"clearance_id": cr.ID,
		"status":       cr.Status,
		"message":      "Clearance completed successfully. Employee is now fully separated.",
	})
}

// clearanceDefs returns tool definitions for clearance-related tools.
func clearanceDefs() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "get_clearance_status",
			Description: "Get clearance/offboarding progress for an employee: items completed, pending departments, and overall status. Admin/Manager only.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"clearance_id": map[string]any{"type": "integer", "description": "Clearance request ID."},
				},
				"required": []string{"clearance_id"},
			}),
		},
		{
			Name:        "update_clearance_item",
			Description: "Mark a clearance item as cleared or flagged. Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"item_id": map[string]any{"type": "integer", "description": "Clearance item ID."},
					"status":  map[string]any{"type": "string", "description": "New status: cleared, flagged. Default: cleared."},
					"remarks": map[string]any{"type": "string", "description": "Optional remarks."},
				},
				"required": []string{"item_id"},
			}),
		},
		{
			Name:        "query_final_pay_components",
			Description: "Calculate final pay components for a separating employee: unpaid salary, leave encashment, pro-rated 13th month, minus outstanding loans. Admin only.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_id":     map[string]any{"type": "integer", "description": "Employee ID."},
					"separation_date": map[string]any{"type": "string", "description": "Separation date in YYYY-MM-DD. Default: today."},
					"unpaid_days":     map[string]any{"type": "number", "description": "Working days since last payroll. Default: 15."},
				},
				"required": []string{"employee_id"},
			}),
		},
		{
			Name:        "create_final_pay",
			Description: "Create a final pay payroll record for a separated employee. Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_id": map[string]any{"type": "integer", "description": "Employee ID."},
					"amount":      map[string]any{"type": "number", "description": "Total final pay amount in PHP."},
					"notes":       map[string]any{"type": "string", "description": "Optional notes about the final pay."},
				},
				"required": []string{"employee_id", "amount"},
			}),
		},
		{
			Name:        "complete_clearance",
			Description: "Complete the clearance process for an employee. Requires all items to be cleared first. Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"clearance_id": map[string]any{"type": "integer", "description": "Clearance request ID."},
				},
				"required": []string{"clearance_id"},
			}),
		},
	}
}

// registerClearanceTools registers clearance-related tool executors.
func (r *ToolRegistry) registerClearanceTools() {
	r.tools["get_clearance_status"] = r.toolGetClearanceStatus
	r.tools["update_clearance_item"] = r.toolUpdateClearanceItem
	r.tools["query_final_pay_components"] = r.toolQueryFinalPayComponents
	r.tools["create_final_pay"] = r.toolCreateFinalPay
	r.tools["complete_clearance"] = r.toolCompleteClearance
}
