package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/numericutil"
)

// =====================================================
// Phase 1: Loan + Benefit + Leave Encashment Tools
// =====================================================

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

// =====================================================
// Phase 2: Performance + Training Tools
// =====================================================

func (r *ToolRegistry) toolListReviewCycles(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	cycles, err := r.queries.ListReviewCycles(ctx, store.ListReviewCyclesParams{
		CompanyID: companyID,
		Limit:     20,
		Offset:    0,
	})
	if err != nil {
		return "", fmt.Errorf("list review cycles: %w", err)
	}

	type cycleResult struct {
		ID             int64  `json:"id"`
		Name           string `json:"name"`
		CycleType      string `json:"cycle_type"`
		PeriodStart    string `json:"period_start"`
		PeriodEnd      string `json:"period_end"`
		ReviewDeadline string `json:"review_deadline"`
		Status         string `json:"status"`
	}
	results := make([]cycleResult, len(cycles))
	for i, c := range cycles {
		cr := cycleResult{
			ID:          c.ID,
			Name:        c.Name,
			CycleType:   c.CycleType,
			PeriodStart: c.PeriodStart.Format("2006-01-02"),
			PeriodEnd:   c.PeriodEnd.Format("2006-01-02"),
			Status:      c.Status,
		}
		if c.ReviewDeadline.Valid {
			cr.ReviewDeadline = c.ReviewDeadline.Time.Format("2006-01-02")
		}
		results[i] = cr
	}
	return toJSON(map[string]any{"total": len(results), "cycles": results})
}

func (r *ToolRegistry) toolGetMyPerformanceReview(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	reviews, err := r.queries.ListMyReviews(ctx, emp.ID)
	if err != nil {
		return "", fmt.Errorf("list reviews: %w", err)
	}

	type reviewResult struct {
		ID          int64  `json:"id"`
		CycleName   string `json:"cycle_name"`
		CycleType   string `json:"cycle_type"`
		Status      string `json:"status"`
		SelfRating  *int32 `json:"self_rating,omitempty"`
		FinalRating *int32 `json:"final_rating,omitempty"`
		PeriodStart string `json:"period_start"`
		PeriodEnd   string `json:"period_end"`
	}
	results := make([]reviewResult, len(reviews))
	for i, rv := range reviews {
		results[i] = reviewResult{
			ID:          rv.ID,
			CycleName:   rv.CycleName,
			CycleType:   rv.CycleType,
			Status:      rv.Status,
			SelfRating:  rv.SelfRating,
			FinalRating: rv.FinalRating,
			PeriodStart: rv.PeriodStart.Format("2006-01-02"),
			PeriodEnd:   rv.PeriodEnd.Format("2006-01-02"),
		}
	}

	return toJSON(map[string]any{"total": len(results), "reviews": results})
}

func (r *ToolRegistry) toolCreateGoal(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	title, _ := input["title"].(string)
	if title == "" {
		return "", fmt.Errorf("title is required")
	}

	category := "individual"
	if c, ok := input["category"].(string); ok && c != "" {
		category = c
	}

	var description *string
	if d, ok := input["description"].(string); ok && d != "" {
		description = &d
	}

	var weight pgtype.Numeric
	if w, ok := input["weight"].(float64); ok && w > 0 {
		_ = weight.Scan(fmt.Sprintf("%.0f", w))
	}

	var targetValue *string
	if tv, ok := input["target_value"].(string); ok && tv != "" {
		targetValue = &tv
	}

	var dueDate pgtype.Date
	if dd, ok := input["due_date"].(string); ok && dd != "" {
		t, err := time.Parse("2006-01-02", dd)
		if err == nil {
			dueDate = pgtype.Date{Time: t, Valid: true}
		}
	}

	var cycleID *int64
	if cid, ok := input["review_cycle_id"].(float64); ok && cid > 0 {
		id := int64(cid)
		cycleID = &id
	}

	goal, err := r.queries.CreateGoal(ctx, store.CreateGoalParams{
		CompanyID:     companyID,
		EmployeeID:    emp.ID,
		ReviewCycleID: cycleID,
		Title:         title,
		Description:   description,
		Category:      category,
		Weight:        weight,
		TargetValue:   targetValue,
		DueDate:       dueDate,
	})
	if err != nil {
		return "", fmt.Errorf("create goal: %w", err)
	}

	return toJSON(map[string]any{
		"success": true,
		"goal_id": goal.ID,
		"title":   goal.Title,
		"message": fmt.Sprintf("Goal '%s' created successfully.", title),
	})
}

func (r *ToolRegistry) toolSubmitSelfReview(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	reviewID, ok := input["review_id"].(float64)
	if !ok || reviewID <= 0 {
		return "", fmt.Errorf("review_id is required")
	}

	ratingFloat, ok := input["rating"].(float64)
	if !ok || ratingFloat < 1 || ratingFloat > 5 {
		return "", fmt.Errorf("rating is required (1-5)")
	}
	rating := int32(ratingFloat)

	var comments *string
	if c, ok := input["comments"].(string); ok && c != "" {
		comments = &c
	}

	review, err := r.queries.SubmitSelfReview(ctx, store.SubmitSelfReviewParams{
		ID:           int64(reviewID),
		CompanyID:    companyID,
		SelfRating:   &rating,
		SelfComments: comments,
	})
	if err != nil {
		return "", fmt.Errorf("submit self review: %w", err)
	}

	return toJSON(map[string]any{
		"success":   true,
		"review_id": review.ID,
		"status":    review.Status,
		"message":   "Self-review submitted successfully. It is now pending manager review.",
	})
}

func (r *ToolRegistry) toolSubmitManagerReview(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	reviewID, ok := input["review_id"].(float64)
	if !ok || reviewID <= 0 {
		return "", fmt.Errorf("review_id is required")
	}

	ratingFloat, ok := input["rating"].(float64)
	if !ok || ratingFloat < 1 || ratingFloat > 5 {
		return "", fmt.Errorf("rating is required (1-5)")
	}
	rating := int32(ratingFloat)

	var comments, strengths, improvements, finalComments *string
	if c, ok := input["comments"].(string); ok && c != "" {
		comments = &c
	}
	if s, ok := input["strengths"].(string); ok && s != "" {
		strengths = &s
	}
	if im, ok := input["improvements"].(string); ok && im != "" {
		improvements = &im
	}
	if fc, ok := input["final_comments"].(string); ok && fc != "" {
		finalComments = &fc
	}

	finalRating := rating
	if fr, ok := input["final_rating"].(float64); ok && fr >= 1 && fr <= 5 {
		finalRating = int32(fr)
	}

	review, err := r.queries.SubmitManagerReview(ctx, store.SubmitManagerReviewParams{
		ID:              int64(reviewID),
		CompanyID:       companyID,
		ManagerRating:   &rating,
		ManagerComments: comments,
		Strengths:       strengths,
		Improvements:    improvements,
		FinalRating:     &finalRating,
		FinalComments:   finalComments,
	})
	if err != nil {
		return "", fmt.Errorf("submit manager review: %w", err)
	}

	return toJSON(map[string]any{
		"success":      true,
		"review_id":    review.ID,
		"status":       review.Status,
		"final_rating": finalRating,
		"message":      "Manager review submitted and performance review completed.",
	})
}

func (r *ToolRegistry) toolListTrainings(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	trainings, err := r.queries.ListTrainings(ctx, store.ListTrainingsParams{
		CompanyID: companyID,
		Limit:     30,
		Offset:    0,
	})
	if err != nil {
		return "", fmt.Errorf("list trainings: %w", err)
	}

	type trainingResult struct {
		ID               int64  `json:"id"`
		Title            string `json:"title"`
		TrainingType     string `json:"training_type"`
		Trainer          string `json:"trainer,omitempty"`
		StartDate        string `json:"start_date"`
		EndDate          string `json:"end_date"`
		Status           string `json:"status"`
		ParticipantCount int64  `json:"participant_count"`
		MaxParticipants  *int32 `json:"max_participants,omitempty"`
	}
	results := make([]trainingResult, len(trainings))
	for i, t := range trainings {
		tr := trainingResult{
			ID:               t.ID,
			Title:            t.Title,
			TrainingType:     t.TrainingType,
			StartDate:        t.StartDate.Format("2006-01-02"),
			Status:           t.Status,
			ParticipantCount: t.ParticipantCount,
			MaxParticipants:  t.MaxParticipants,
		}
		if t.EndDate.Valid {
			tr.EndDate = t.EndDate.Time.Format("2006-01-02")
		}
		if t.Trainer != nil {
			tr.Trainer = *t.Trainer
		}
		results[i] = tr
	}
	return toJSON(map[string]any{"total": len(results), "trainings": results})
}

func (r *ToolRegistry) toolListMyCertifications(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	certs, err := r.queries.ListMyCertifications(ctx, emp.ID)
	if err != nil {
		return "", fmt.Errorf("list certifications: %w", err)
	}

	type certResult struct {
		ID           int64  `json:"id"`
		Name         string `json:"name"`
		IssuingBody  string `json:"issuing_body"`
		IssueDate    string `json:"issue_date"`
		ExpiryDate   string `json:"expiry_date,omitempty"`
		Status       string `json:"status"`
		CredentialID string `json:"credential_id,omitempty"`
	}
	results := make([]certResult, len(certs))
	for i, c := range certs {
		cr := certResult{
			ID:        c.ID,
			Name:      c.Name,
			IssueDate: c.IssueDate.Format("2006-01-02"),
			Status:    c.Status,
		}
		if c.IssuingBody != nil {
			cr.IssuingBody = *c.IssuingBody
		}
		if c.ExpiryDate.Valid {
			cr.ExpiryDate = c.ExpiryDate.Time.Format("2006-01-02")
		}
		if c.CredentialID != nil {
			cr.CredentialID = *c.CredentialID
		}
		results[i] = cr
	}

	// Also check expiring certs
	expiring, _ := r.queries.ListExpiringCertifications(ctx, companyID)
	var expiringCount int
	for _, e := range expiring {
		if e.EmployeeID == emp.ID {
			expiringCount++
		}
	}

	return toJSON(map[string]any{
		"total":          len(results),
		"certifications": results,
		"expiring_soon":  expiringCount,
	})
}

func (r *ToolRegistry) toolEnrollInTraining(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	trainingID, ok := input["training_id"].(float64)
	if !ok || trainingID <= 0 {
		return "", fmt.Errorf("training_id is required")
	}

	// Verify training exists and is open
	training, err := r.queries.GetTraining(ctx, store.GetTrainingParams{
		ID:        int64(trainingID),
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("training not found: %w", err)
	}
	if training.Status != "scheduled" && training.Status != "in_progress" {
		return toJSON(map[string]any{
			"success": false,
			"message": fmt.Sprintf("Training is '%s' and not accepting enrollments.", training.Status),
		})
	}

	participant, err := r.queries.AddTrainingParticipant(ctx, store.AddTrainingParticipantParams{
		TrainingID: int64(trainingID),
		EmployeeID: emp.ID,
	})
	if err != nil {
		return "", fmt.Errorf("enroll in training: %w", err)
	}

	return toJSON(map[string]any{
		"success":        true,
		"participant_id": participant.ID,
		"training_title": training.Title,
		"message":        fmt.Sprintf("Successfully enrolled in '%s'.", training.Title),
	})
}

func (r *ToolRegistry) toolMarkTrainingComplete(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}

	participantID, ok := input["participant_id"].(float64)
	if !ok || participantID <= 0 {
		return "", fmt.Errorf("participant_id is required")
	}

	trainingID, ok := input["training_id"].(float64)
	if !ok || trainingID <= 0 {
		return "", fmt.Errorf("training_id is required")
	}

	var score pgtype.Numeric
	if s, ok := input["score"].(float64); ok && s >= 0 {
		_ = score.Scan(fmt.Sprintf("%.0f", s))
	}

	var feedback *string
	if f, ok := input["feedback"].(string); ok && f != "" {
		feedback = &f
	}

	participant, err := r.queries.UpdateParticipantStatus(ctx, store.UpdateParticipantStatusParams{
		ID:         int64(participantID),
		TrainingID: int64(trainingID),
		Status:     "completed",
		Score:      score,
		Feedback:   feedback,
	})
	if err != nil {
		return "", fmt.Errorf("mark training complete: %w", err)
	}

	return toJSON(map[string]any{
		"success":        true,
		"participant_id": participant.ID,
		"status":         participant.Status,
		"message":        "Training marked as completed.",
	})
}

// =====================================================
// Phase 3: Disciplinary + Grievance Tools
// =====================================================

func (r *ToolRegistry) toolQueryEmployeeDisciplinary(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	employeeID, ok := input["employee_id"].(float64)
	if !ok || employeeID <= 0 {
		return "", fmt.Errorf("employee_id is required")
	}

	summary, err := r.queries.GetEmployeeDisciplinarySummary(ctx, store.GetEmployeeDisciplinarySummaryParams{
		CompanyID:  companyID,
		EmployeeID: int64(employeeID),
	})
	if err != nil {
		return "", fmt.Errorf("get disciplinary summary: %w", err)
	}

	actions, err := r.queries.GetEmployeeActionCounts(ctx, store.GetEmployeeActionCountsParams{
		CompanyID:  companyID,
		EmployeeID: int64(employeeID),
	})
	if err != nil {
		return "", fmt.Errorf("get action counts: %w", err)
	}

	return toJSON(map[string]any{
		"incidents": map[string]any{
			"total":       summary.TotalIncidents,
			"grave":       summary.GraveCount,
			"open":        summary.OpenCount,
		},
		"actions": map[string]any{
			"total":            actions.TotalActions,
			"verbal_warnings":  actions.VerbalWarnings,
			"written_warnings": actions.WrittenWarnings,
			"final_warnings":   actions.FinalWarnings,
			"suspensions":      actions.Suspensions,
		},
	})
}

func (r *ToolRegistry) toolCreateDisciplinaryIncident(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	employeeID, ok := input["employee_id"].(float64)
	if !ok || employeeID <= 0 {
		return "", fmt.Errorf("employee_id is required")
	}

	category, _ := input["category"].(string)
	if category == "" {
		return "", fmt.Errorf("category is required (tardiness, absence, misconduct, insubordination, policy_violation, performance, safety)")
	}

	severity := "minor"
	if s, ok := input["severity"].(string); ok && s != "" {
		severity = s
	}

	description, _ := input["description"].(string)
	if description == "" {
		return "", fmt.Errorf("description is required")
	}

	incidentDateStr, _ := input["incident_date"].(string)
	if incidentDateStr == "" {
		incidentDateStr = time.Now().Format("2006-01-02")
	}
	incidentDate, err := time.Parse("2006-01-02", incidentDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid incident_date format, use YYYY-MM-DD")
	}

	var witnesses, evidenceNotes *string
	if w, ok := input["witnesses"].(string); ok && w != "" {
		witnesses = &w
	}
	if e, ok := input["evidence_notes"].(string); ok && e != "" {
		evidenceNotes = &e
	}

	incident, err := r.queries.CreateDisciplinaryIncident(ctx, store.CreateDisciplinaryIncidentParams{
		CompanyID:     companyID,
		EmployeeID:    int64(employeeID),
		ReportedBy:    &userID,
		IncidentDate:  incidentDate,
		Category:      category,
		Severity:      severity,
		Description:   description,
		Witnesses:     witnesses,
		EvidenceNotes: evidenceNotes,
	})
	if err != nil {
		return "", fmt.Errorf("create incident: %w", err)
	}

	return toJSON(map[string]any{
		"success":     true,
		"incident_id": incident.ID,
		"message":     "Disciplinary incident recorded successfully.",
	})
}

func (r *ToolRegistry) toolCreateDisciplinaryAction(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	employeeID, ok := input["employee_id"].(float64)
	if !ok || employeeID <= 0 {
		return "", fmt.Errorf("employee_id is required")
	}

	actionType, _ := input["action_type"].(string)
	if actionType == "" {
		return "", fmt.Errorf("action_type is required (verbal_warning, written_warning, final_warning, suspension, termination)")
	}

	description, _ := input["description"].(string)
	if description == "" {
		return "", fmt.Errorf("description is required")
	}

	var incidentID *int64
	if iid, ok := input["incident_id"].(float64); ok && iid > 0 {
		id := int64(iid)
		incidentID = &id
	}

	var suspensionDays *int32
	if sd, ok := input["suspension_days"].(float64); ok && sd > 0 {
		d := int32(sd)
		suspensionDays = &d
	}

	var notes *string
	if n, ok := input["notes"].(string); ok && n != "" {
		notes = &n
	}

	var effectiveDate, endDate pgtype.Date
	if ed, ok := input["effective_date"].(string); ok && ed != "" {
		t, err := time.Parse("2006-01-02", ed)
		if err == nil {
			effectiveDate = pgtype.Date{Time: t, Valid: true}
		}
	}
	if ed, ok := input["end_date"].(string); ok && ed != "" {
		t, err := time.Parse("2006-01-02", ed)
		if err == nil {
			endDate = pgtype.Date{Time: t, Valid: true}
		}
	}

	action, err := r.queries.CreateDisciplinaryAction(ctx, store.CreateDisciplinaryActionParams{
		CompanyID:      companyID,
		EmployeeID:     int64(employeeID),
		IncidentID:     incidentID,
		ActionType:     actionType,
		ActionDate:     time.Now(),
		IssuedBy:       userID,
		Description:    description,
		SuspensionDays: suspensionDays,
		EffectiveDate:  effectiveDate,
		EndDate:        endDate,
		Notes:          notes,
	})
	if err != nil {
		return "", fmt.Errorf("create action: %w", err)
	}

	return toJSON(map[string]any{
		"success":   true,
		"action_id": action.ID,
		"type":      action.ActionType,
		"message":   fmt.Sprintf("Disciplinary action '%s' created successfully.", actionType),
	})
}

func (r *ToolRegistry) toolListRecentIncidents(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	incidents, err := r.queries.ListDisciplinaryIncidents(ctx, store.ListDisciplinaryIncidentsParams{
		CompanyID:  companyID,
		Status:     "",
		EmployeeID: int64(0),
		Lim:        20,
		Off:        0,
	})
	if err != nil {
		return "", fmt.Errorf("list incidents: %w", err)
	}

	type incidentResult struct {
		ID           int64  `json:"id"`
		EmployeeName string `json:"employee_name"`
		EmployeeNo   string `json:"employee_no"`
		Category     string `json:"category"`
		Severity     string `json:"severity"`
		Status       string `json:"status"`
		IncidentDate string `json:"incident_date"`
		Description  string `json:"description"`
	}
	results := make([]incidentResult, len(incidents))
	for i, inc := range incidents {
		results[i] = incidentResult{
			ID:           inc.ID,
			EmployeeName: inc.FirstName + " " + inc.LastName,
			EmployeeNo:   inc.EmployeeNo,
			Category:     inc.Category,
			Severity:     inc.Severity,
			Status:       inc.Status,
			IncidentDate: inc.IncidentDate.Format("2006-01-02"),
			Description:  inc.Description,
		}
	}

	return toJSON(map[string]any{"total": len(results), "incidents": results})
}

func (r *ToolRegistry) toolQueryGrievanceSummary(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	summary, err := r.queries.GetGrievanceSummary(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("get grievance summary: %w", err)
	}

	return toJSON(map[string]any{
		"total":        summary.Total,
		"open":         summary.OpenCount,
		"under_review": summary.UnderReview,
		"in_mediation": summary.InMediation,
		"resolved":     summary.Resolved,
		"critical":     summary.CriticalOpen,
	})
}

func (r *ToolRegistry) toolGetGrievanceDetail(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	grievanceID, ok := input["grievance_id"].(float64)
	if !ok || grievanceID <= 0 {
		return "", fmt.Errorf("grievance_id is required")
	}

	grievance, err := r.queries.GetGrievance(ctx, store.GetGrievanceParams{
		ID:        int64(grievanceID),
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("grievance not found: %w", err)
	}

	comments, _ := r.queries.ListGrievanceComments(ctx, int64(grievanceID))

	type commentResult struct {
		UserName   string `json:"user_name"`
		Comment    string `json:"comment"`
		IsInternal bool   `json:"is_internal"`
		CreatedAt  string `json:"created_at"`
	}
	commentResults := make([]commentResult, len(comments))
	for i, c := range comments {
		commentResults[i] = commentResult{
			UserName:   fmt.Sprintf("%v", c.UserName),
			Comment:    c.Comment,
			IsInternal: c.IsInternal,
			CreatedAt:  c.CreatedAt.Format("2006-01-02 15:04"),
		}
	}

	return toJSON(map[string]any{
		"id":            grievance.ID,
		"case_number":   grievance.CaseNumber,
		"employee_name": grievance.FirstName + " " + grievance.LastName,
		"category":      grievance.Category,
		"subject":       grievance.Subject,
		"description":   grievance.Description,
		"severity":      grievance.Severity,
		"status":        grievance.Status,
		"assigned_to":   grievance.AssignedToName,
		"comments":      commentResults,
	})
}

func (r *ToolRegistry) toolResolveGrievance(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}

	grievanceID, ok := input["grievance_id"].(float64)
	if !ok || grievanceID <= 0 {
		return "", fmt.Errorf("grievance_id is required")
	}

	resolution, _ := input["resolution"].(string)
	if resolution == "" {
		return "", fmt.Errorf("resolution is required")
	}

	gc, err := r.queries.ResolveGrievance(ctx, store.ResolveGrievanceParams{
		ID:         int64(grievanceID),
		CompanyID:  companyID,
		Resolution: &resolution,
	})
	if err != nil {
		return "", fmt.Errorf("resolve grievance: %w", err)
	}

	return toJSON(map[string]any{
		"success":      true,
		"grievance_id": gc.ID,
		"status":       gc.Status,
		"message":      "Grievance resolved successfully.",
	})
}

// =====================================================
// Phase 4: Analytics + Tax Tools
// =====================================================

func (r *ToolRegistry) toolQueryCompanyAnalytics(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	summary, err := r.queries.GetAnalyticsSummary(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("get analytics summary: %w", err)
	}

	deptCosts, err := r.queries.GetDepartmentCostAnalysis(ctx, companyID)
	if err != nil {
		deptCosts = nil
	}

	type deptCost struct {
		Department    string `json:"department"`
		EmployeeCount int64  `json:"employee_count"`
		TotalSalary   string `json:"total_salary"`
	}
	departments := make([]deptCost, len(deptCosts))
	for i, d := range deptCosts {
		departments[i] = deptCost{
			Department:    d.DepartmentName,
			EmployeeCount: d.EmployeeCount,
			TotalSalary:   fmt.Sprintf("%v", d.TotalSalaryCost),
		}
	}

	return toJSON(map[string]any{
		"active_employees":    summary.ActiveEmployees,
		"separated_employees": summary.SeparatedEmployees,
		"new_hires_this_month": summary.NewHiresThisMonth,
		"probationary_count":  summary.ProbationaryCount,
		"avg_tenure_years":    numericToString(summary.AvgTenureYears),
		"departments":         departments,
	})
}

func (r *ToolRegistry) toolQueryHeadcountTrend(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	startDate := time.Now().AddDate(-1, 0, 0)
	trends, err := r.queries.GetHeadcountTrend(ctx, store.GetHeadcountTrendParams{
		CompanyID: companyID,
		HireDate:  startDate,
	})
	if err != nil {
		return "", fmt.Errorf("get headcount trend: %w", err)
	}

	return toJSON(map[string]any{
		"period": fmt.Sprintf("%s to %s", startDate.Format("2006-01"), time.Now().Format("2006-01")),
		"months": trends,
	})
}

func (r *ToolRegistry) toolQueryLeaveUtilization(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	startDate := time.Date(time.Now().Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	utilization, err := r.queries.GetLeaveUtilization(ctx, store.GetLeaveUtilizationParams{
		CompanyID: companyID,
		StartDate: startDate,
	})
	if err != nil {
		return "", fmt.Errorf("get leave utilization: %w", err)
	}

	return toJSON(map[string]any{
		"year":        time.Now().Year(),
		"utilization": utilization,
	})
}

func (r *ToolRegistry) toolQueryTaxFilingStatus(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}

	year := int32(time.Now().Year())
	summary, err := r.queries.GetFilingSummary(ctx, store.GetFilingSummaryParams{
		CompanyID:  companyID,
		PeriodYear: year,
	})
	if err != nil {
		return "", fmt.Errorf("get filing summary: %w", err)
	}

	overdue, _ := r.queries.ListOverdueFilings(ctx, companyID)
	upcoming, _ := r.queries.ListUpcomingFilings(ctx, companyID)

	type filingItem struct {
		ID         int64  `json:"id"`
		FilingType string `json:"filing_type"`
		DueDate    string `json:"due_date"`
		Status     string `json:"status"`
	}

	overdueItems := make([]filingItem, len(overdue))
	for i, f := range overdue {
		overdueItems[i] = filingItem{
			ID: f.ID, FilingType: f.FilingType,
			DueDate: f.DueDate.Format("2006-01-02"), Status: f.Status,
		}
	}

	upcomingItems := make([]filingItem, len(upcoming))
	for i, f := range upcoming {
		upcomingItems[i] = filingItem{
			ID: f.ID, FilingType: f.FilingType,
			DueDate: f.DueDate.Format("2006-01-02"), Status: f.Status,
		}
	}

	return toJSON(map[string]any{
		"year":            year,
		"total_filings":   summary.Total,
		"filed":           summary.Filed,
		"overdue_count":   summary.Overdue,
		"upcoming_count":  summary.Upcoming,
		"total_amount":    summary.TotalAmount,
		"total_penalties": summary.TotalPenalties,
		"overdue_items":   overdueItems,
		"upcoming_items":  upcomingItems,
	})
}

func (r *ToolRegistry) toolCreateTaxFilingRecord(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}

	filingType, _ := input["filing_type"].(string)
	if filingType == "" {
		return "", fmt.Errorf("filing_type is required (bir_1601c, sss_r3, philhealth_rf1, pagibig_ml1, bir_2316, bir_0619e)")
	}

	periodType := "monthly"
	if pt, ok := input["period_type"].(string); ok && pt != "" {
		periodType = pt
	}

	periodYear := int32(time.Now().Year())
	if py, ok := input["period_year"].(float64); ok && py > 0 {
		periodYear = int32(py)
	}

	var periodMonth, periodQuarter *int32
	if pm, ok := input["period_month"].(float64); ok && pm > 0 {
		m := int32(pm)
		periodMonth = &m
	}
	if pq, ok := input["period_quarter"].(float64); ok && pq > 0 {
		q := int32(pq)
		periodQuarter = &q
	}

	dueDateStr, _ := input["due_date"].(string)
	if dueDateStr == "" {
		return "", fmt.Errorf("due_date is required in YYYY-MM-DD format")
	}
	dueDate, err := time.Parse("2006-01-02", dueDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid due_date format")
	}

	var amount pgtype.Numeric
	if a, ok := input["amount"].(float64); ok && a > 0 {
		_ = amount.Scan(fmt.Sprintf("%.2f", a))
	}

	filing, err := r.queries.CreateTaxFiling(ctx, store.CreateTaxFilingParams{
		CompanyID:     companyID,
		FilingType:    filingType,
		PeriodType:    periodType,
		PeriodYear:    periodYear,
		PeriodMonth:   periodMonth,
		PeriodQuarter: periodQuarter,
		DueDate:       dueDate,
		Amount:        amount,
		Status:        "pending",
	})
	if err != nil {
		return "", fmt.Errorf("create tax filing: %w", err)
	}

	return toJSON(map[string]any{
		"success":   true,
		"filing_id": filing.ID,
		"type":      filing.FilingType,
		"due_date":  dueDateStr,
		"message":   fmt.Sprintf("Tax filing record '%s' created for %d.", filingType, periodYear),
	})
}

// =====================================================
// Phase 5: Clearance + Final Pay Tools
// =====================================================

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

	items, _ := r.queries.ListClearanceItems(ctx, int64(clearanceID))
	statusCounts, _ := r.queries.CountClearanceItemsByStatus(ctx, int64(clearanceID))

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
		"clearance_id":    cr.ID,
		"employee_name":   fmt.Sprintf("%v", cr.EmployeeName),
		"employee_no":     cr.EmployeeNo,
		"status":          cr.Status,
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
	// Assume 15 working days as estimate
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
		"employee_id":      empID,
		"separation_date":  separationDateStr,
		"basic_salary":     basicSalary,
		"daily_rate":       dailyRate,
		"components": map[string]any{
			"unpaid_salary":    unpaidSalary,
			"unpaid_days":      unpaidDays,
			"leave_encashment": leaveEncashment,
			"leave_days":       leaveDays,
			"13th_month_prorate": thirteenthMonth,
			"months_worked":    monthsWorked,
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
	statusCounts, err := r.queries.CountClearanceItemsByStatus(ctx, int64(clearanceID))
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

// =====================================================
// Phase 6: Schedule Tools
// =====================================================

func (r *ToolRegistry) toolListScheduleTemplates(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	templates, err := r.queries.ListScheduleTemplates(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("list schedule templates: %w", err)
	}

	type templateResult struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	}
	results := make([]templateResult, len(templates))
	for i, t := range templates {
		tr := templateResult{ID: t.ID, Name: t.Name}
		if t.Description != nil {
			tr.Description = *t.Description
		}
		results[i] = tr
	}
	return toJSON(map[string]any{"total": len(results), "templates": results})
}

func (r *ToolRegistry) toolGetEmployeeSchedule(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	employeeID := emp.ID
	if eid, ok := input["employee_id"].(float64); ok && eid > 0 {
		employeeID = int64(eid)
	}

	assignment, err := r.queries.GetEmployeeCurrentTemplate(ctx, store.GetEmployeeCurrentTemplateParams{
		CompanyID:     companyID,
		EmployeeID:    employeeID,
		EffectiveFrom: time.Now(),
	})
	if err != nil {
		return toJSON(map[string]any{"message": "No schedule template assigned.", "assigned": false})
	}

	// Get template days
	days, _ := r.queries.ListScheduleTemplateDays(ctx, assignment.TemplateID)

	dayNames := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	type dayResult struct {
		Day       string `json:"day"`
		ShiftName string `json:"shift_name,omitempty"`
		StartTime string `json:"start_time,omitempty"`
		EndTime   string `json:"end_time,omitempty"`
		IsRestDay bool   `json:"is_rest_day"`
	}
	dayResults := make([]dayResult, len(days))
	for i, d := range days {
		dr := dayResult{
			Day:       dayNames[d.DayOfWeek],
			IsRestDay: d.IsRestDay,
		}
		if d.ShiftName != nil {
			dr.ShiftName = *d.ShiftName
		}
		if d.StartTime.Valid {
			dr.StartTime = fmt.Sprintf("%02d:%02d", d.StartTime.Microseconds/3600000000, (d.StartTime.Microseconds%3600000000)/60000000)
		}
		if d.EndTime.Valid {
			dr.EndTime = fmt.Sprintf("%02d:%02d", d.EndTime.Microseconds/3600000000, (d.EndTime.Microseconds%3600000000)/60000000)
		}
		dayResults[i] = dr
	}

	return toJSON(map[string]any{
		"assigned":       true,
		"template_name":  assignment.TemplateName,
		"effective_from": assignment.EffectiveFrom.Format("2006-01-02"),
		"schedule":       dayResults,
	})
}

func (r *ToolRegistry) toolAssignSchedule(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	employeeID, ok := input["employee_id"].(float64)
	if !ok || employeeID <= 0 {
		return "", fmt.Errorf("employee_id is required")
	}

	templateID, ok := input["template_id"].(float64)
	if !ok || templateID <= 0 {
		return "", fmt.Errorf("template_id is required")
	}

	effectiveDateStr, _ := input["effective_date"].(string)
	if effectiveDateStr == "" {
		effectiveDateStr = time.Now().Format("2006-01-02")
	}
	effectiveDate, err := time.Parse("2006-01-02", effectiveDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid effective_date format")
	}

	assignment, err := r.queries.AssignScheduleTemplate(ctx, store.AssignScheduleTemplateParams{
		CompanyID:     companyID,
		EmployeeID:    int64(employeeID),
		TemplateID:    int64(templateID),
		EffectiveFrom: effectiveDate,
		EffectiveTo:   pgtype.Date{Valid: false},
	})
	if err != nil {
		return "", fmt.Errorf("assign schedule: %w", err)
	}

	return toJSON(map[string]any{
		"success":       true,
		"assignment_id": assignment.ID,
		"message":       "Schedule template assigned successfully.",
	})
}

// =====================================================
// Helpers
// =====================================================

func (r *ToolRegistry) requireRole(ctx context.Context, userID, companyID int64, roles ...string) error {
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}
	if user.CompanyID != companyID {
		return fmt.Errorf("access denied")
	}
	for _, role := range roles {
		if user.Role == role || user.Role == "super_admin" {
			return nil
		}
	}
	return fmt.Errorf("only %v can perform this action", roles)
}
