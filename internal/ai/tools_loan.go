package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/numericutil"
)

func (r *ToolRegistry) toolQueryMyLoans(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	loans, err := r.queries.ListMyLoans(ctx, emp.ID)
	if err != nil {
		return "", fmt.Errorf("list loans: %w", err)
	}

	type loanResult struct {
		ID                  int64  `json:"id"`
		LoanType            string `json:"loan_type"`
		PrincipalAmount     string `json:"principal_amount"`
		RemainingBalance    string `json:"remaining_balance"`
		MonthlyAmortization string `json:"monthly_amortization"`
		Status              string `json:"status"`
		TermMonths          int32  `json:"term_months"`
	}
	results := make([]loanResult, len(loans))
	for i, l := range loans {
		results[i] = loanResult{
			ID:                  l.ID,
			LoanType:            l.LoanTypeName,
			PrincipalAmount:     numericToString(l.PrincipalAmount),
			RemainingBalance:    numericToString(l.RemainingBalance),
			MonthlyAmortization: numericToString(l.MonthlyAmortization),
			Status:              l.Status,
			TermMonths:          l.TermMonths,
		}
	}

	if len(results) == 0 {
		return toJSON(map[string]any{"message": "You have no loans on record.", "loans": []any{}})
	}
	return toJSON(map[string]any{"total": len(results), "loans": results})
}

func (r *ToolRegistry) toolListPendingLoans(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT l.id, e.first_name || ' ' || e.last_name AS employee_name, e.employee_no,
		       lt.name AS loan_type, l.principal_amount, l.term_months, l.created_at
		FROM loans l
		JOIN employees e ON e.id = l.employee_id
		JOIN loan_types lt ON lt.id = l.loan_type_id
		WHERE l.company_id = $1 AND l.status = 'pending'
		ORDER BY l.created_at
	`, companyID)
	if err != nil {
		return "", fmt.Errorf("list pending loans: %w", err)
	}
	defer rows.Close()

	type pendingLoan struct {
		ID           int64  `json:"id"`
		EmployeeName string `json:"employee_name"`
		EmployeeNo   string `json:"employee_no"`
		LoanType     string `json:"loan_type"`
		Amount       string `json:"amount"`
		TermMonths   int32  `json:"term_months"`
		RequestedAt  string `json:"requested_at"`
	}
	var results []pendingLoan
	for rows.Next() {
		var p pendingLoan
		var amount pgtype.Numeric
		var createdAt time.Time
		if err := rows.Scan(&p.ID, &p.EmployeeName, &p.EmployeeNo, &p.LoanType, &amount, &p.TermMonths, &createdAt); err != nil {
			continue
		}
		p.Amount = numericToString(amount)
		p.RequestedAt = createdAt.Format("2006-01-02")
		results = append(results, p)
	}
	if results == nil {
		results = []pendingLoan{}
	}
	return toJSON(map[string]any{"total": len(results), "pending_loans": results})
}

func (r *ToolRegistry) toolApproveLoan(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	loanID, ok := input["loan_id"].(float64)
	if !ok || loanID <= 0 {
		return "", fmt.Errorf("loan_id is required")
	}

	loan, err := r.queries.ApproveLoan(ctx, store.ApproveLoanParams{
		ID:         int64(loanID),
		CompanyID:  companyID,
		ApprovedBy: &userID,
	})
	if err != nil {
		return "", fmt.Errorf("approve loan: %w", err)
	}

	return toJSON(map[string]any{
		"success": true,
		"loan_id": loan.ID,
		"status":  loan.Status,
		"message": "Loan approved successfully.",
	})
}

func (r *ToolRegistry) toolRejectLoan(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	loanID, ok := input["loan_id"].(float64)
	if !ok || loanID <= 0 {
		return "", fmt.Errorf("loan_id is required")
	}

	loan, err := r.queries.CancelLoan(ctx, store.CancelLoanParams{
		ID:        int64(loanID),
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("reject loan: %w", err)
	}

	return toJSON(map[string]any{
		"success": true,
		"loan_id": loan.ID,
		"status":  loan.Status,
		"message": "Loan rejected/cancelled successfully.",
	})
}

func (r *ToolRegistry) toolQueryLoanEligibility(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	employeeID := emp.ID
	if eid, ok := input["employee_id"].(float64); ok && eid > 0 {
		if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
			return "", err
		}
		employeeID = int64(eid)
	}

	// Get current salary
	salary, err := r.queries.GetCurrentSalary(ctx, store.GetCurrentSalaryParams{
		CompanyID:     companyID,
		EmployeeID:    employeeID,
		EffectiveFrom: time.Now(),
	})
	if err != nil {
		return toJSON(map[string]any{"eligible": false, "reason": "No salary record found."})
	}

	basicSalary := numericutil.ToFloat(salary.BasicSalary)
	maxLoanAmount := basicSalary * 3

	// Get existing active loans
	activeLoans, err := r.queries.GetEmployeeActiveLoanSummary(ctx, employeeID)
	if err != nil {
		activeLoans = nil
	}

	var totalOutstanding float64
	for _, l := range activeLoans {
		totalOutstanding += numericutil.ToFloat(l.RemainingBalance)
	}

	availableAmount := maxLoanAmount - totalOutstanding
	if availableAmount < 0 {
		availableAmount = 0
	}

	// Get available loan types
	loanTypes, err := r.queries.ListLoanTypes(ctx, companyID)
	if err != nil {
		loanTypes = nil
	}

	type loanTypeInfo struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Code string `json:"code"`
	}
	types := make([]loanTypeInfo, len(loanTypes))
	for i, lt := range loanTypes {
		types[i] = loanTypeInfo{ID: lt.ID, Name: lt.Name, Code: lt.Code}
	}

	return toJSON(map[string]any{
		"eligible":           availableAmount > 0,
		"basic_salary":       basicSalary,
		"max_loan_amount":    maxLoanAmount,
		"existing_loan_debt": totalOutstanding,
		"available_amount":   availableAmount,
		"active_loan_count":  len(activeLoans),
		"available_types":    types,
	})
}
