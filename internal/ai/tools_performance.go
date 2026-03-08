package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/store"
)

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

func (r *ToolRegistry) toolSubmitSelfReview(ctx context.Context, companyID, _ int64, input map[string]any) (string, error) {
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
