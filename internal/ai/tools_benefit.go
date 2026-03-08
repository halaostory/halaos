package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/numericutil"
)

func (r *ToolRegistry) toolQueryMyBenefits(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	enrollments, err := r.queries.ListMyEnrollments(ctx, store.ListMyEnrollmentsParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
	})
	if err != nil {
		return "", fmt.Errorf("list enrollments: %w", err)
	}

	type enrollmentResult struct {
		ID            int64  `json:"id"`
		PlanName      string `json:"plan_name"`
		Category      string `json:"category"`
		Status        string `json:"status"`
		EmployerShare string `json:"employer_share"`
		EmployeeShare string `json:"employee_share"`
	}
	results := make([]enrollmentResult, len(enrollments))
	for i, e := range enrollments {
		results[i] = enrollmentResult{
			ID:            e.ID,
			PlanName:      e.PlanName,
			Category:      e.PlanCategory,
			Status:        e.Status,
			EmployerShare: numericToString(e.EmployerShare),
			EmployeeShare: numericToString(e.EmployeeShare),
		}
	}

	// Also get pending claims
	claims, _ := r.queries.ListBenefitClaims(ctx, store.ListBenefitClaimsParams{
		CompanyID:  companyID,
		Status:     "pending",
		EmployeeID: emp.ID,
		Lim:        10,
		Off:        0,
	})

	return toJSON(map[string]any{
		"enrollments":    results,
		"pending_claims": len(claims),
	})
}

func (r *ToolRegistry) toolListPendingBenefitClaims(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}

	claims, err := r.queries.ListBenefitClaims(ctx, store.ListBenefitClaimsParams{
		CompanyID:  companyID,
		Status:     "pending",
		EmployeeID: int64(0),
		Lim:        50,
		Off:        0,
	})
	if err != nil {
		return "", fmt.Errorf("list benefit claims: %w", err)
	}

	type claimResult struct {
		ID           int64  `json:"id"`
		EmployeeName string `json:"employee_name"`
		PlanName     string `json:"plan_name"`
		Amount       string `json:"amount"`
		ClaimDate    string `json:"claim_date"`
		Description  string `json:"description"`
	}
	results := make([]claimResult, len(claims))
	for i, c := range claims {
		results[i] = claimResult{
			ID:           c.ID,
			EmployeeName: c.FirstName + " " + c.LastName,
			PlanName:     c.PlanName,
			Amount:       numericToString(c.Amount),
			ClaimDate:    c.ClaimDate.Format("2006-01-02"),
			Description:  c.Description,
		}
	}

	return toJSON(map[string]any{"total": len(results), "pending_claims": results})
}

func (r *ToolRegistry) toolApproveBenefitClaim(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}

	claimID, ok := input["claim_id"].(float64)
	if !ok || claimID <= 0 {
		return "", fmt.Errorf("claim_id is required")
	}

	claim, err := r.queries.ApproveBenefitClaim(ctx, store.ApproveBenefitClaimParams{
		ID:         int64(claimID),
		CompanyID:  companyID,
		ApprovedBy: &userID,
	})
	if err != nil {
		return "", fmt.Errorf("approve benefit claim: %w", err)
	}

	return toJSON(map[string]any{
		"success":  true,
		"claim_id": claim.ID,
		"status":   claim.Status,
		"message":  "Benefit claim approved successfully.",
	})
}

func (r *ToolRegistry) toolRejectBenefitClaim(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}

	claimID, ok := input["claim_id"].(float64)
	if !ok || claimID <= 0 {
		return "", fmt.Errorf("claim_id is required")
	}

	var reason *string
	if r, ok := input["reason"].(string); ok && r != "" {
		reason = &r
	}

	claim, err := r.queries.RejectBenefitClaim(ctx, store.RejectBenefitClaimParams{
		ID:              int64(claimID),
		CompanyID:       companyID,
		RejectionReason: reason,
	})
	if err != nil {
		return "", fmt.Errorf("reject benefit claim: %w", err)
	}

	return toJSON(map[string]any{
		"success":  true,
		"claim_id": claim.ID,
		"status":   claim.Status,
		"message":  "Benefit claim rejected.",
	})
}

func (r *ToolRegistry) toolQueryEncashmentEligibility(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	year := int32(time.Now().Year())
	balances, err := r.queries.GetConvertibleLeaveBalances(ctx, store.GetConvertibleLeaveBalancesParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
		Year:       year,
	})
	if err != nil {
		return "", fmt.Errorf("get convertible balances: %w", err)
	}

	// Get daily rate from salary
	salary, salErr := r.queries.GetCurrentSalary(ctx, store.GetCurrentSalaryParams{
		CompanyID:     companyID,
		EmployeeID:    emp.ID,
		EffectiveFrom: time.Now(),
	})

	var dailyRate float64
	if salErr == nil {
		dailyRate = numericutil.ToFloat(salary.BasicSalary) / 22 // approx working days
	}

	type encashableLeave struct {
		LeaveType      string  `json:"leave_type"`
		RemainingDays  int32   `json:"remaining_days"`
		EstimatedValue float64 `json:"estimated_value"`
	}
	var results []encashableLeave
	var totalDays int32
	var totalValue float64
	for _, b := range balances {
		est := float64(b.Remaining) * dailyRate
		results = append(results, encashableLeave{
			LeaveType:      b.LeaveTypeName,
			RemainingDays:  b.Remaining,
			EstimatedValue: est,
		})
		totalDays += b.Remaining
		totalValue += est
	}
	if results == nil {
		results = []encashableLeave{}
	}

	return toJSON(map[string]any{
		"year":             year,
		"daily_rate":       dailyRate,
		"convertible":      results,
		"total_days":       totalDays,
		"total_est_amount": totalValue,
	})
}

func (r *ToolRegistry) toolApproveLeaveEncashment(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}

	encashmentID, ok := input["encashment_id"].(float64)
	if !ok || encashmentID <= 0 {
		return "", fmt.Errorf("encashment_id is required")
	}

	enc, err := r.queries.ApproveLeaveEncashment(ctx, store.ApproveLeaveEncashmentParams{
		ID:         int64(encashmentID),
		CompanyID:  companyID,
		ApprovedBy: &userID,
	})
	if err != nil {
		return "", fmt.Errorf("approve leave encashment: %w", err)
	}

	return toJSON(map[string]any{
		"success":       true,
		"encashment_id": enc.ID,
		"status":        enc.Status,
		"message":       "Leave encashment approved successfully.",
	})
}
