package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/store"
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

// performanceDefs returns tool definitions for performance-related tools.
func performanceDefs() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "list_review_cycles",
			Description: "List performance review cycles for the company. Returns cycle name, type, period, deadline, and status.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "get_my_performance_review",
			Description: "Get the current user's performance reviews. Returns review status, self-rating, final rating, and cycle information.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "create_goal",
			Description: "Create a performance goal from natural language. AI parses the input to extract title, category, weight, target value, and due date. Always confirm details with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title":           map[string]any{"type": "string", "description": "Goal title."},
					"description":     map[string]any{"type": "string", "description": "Detailed goal description."},
					"category":        map[string]any{"type": "string", "description": "Category: individual, team, company. Default: individual."},
					"weight":          map[string]any{"type": "number", "description": "Goal weight percentage (0-100)."},
					"target_value":    map[string]any{"type": "string", "description": "Target value to achieve (e.g., '90%', '100 units')."},
					"due_date":        map[string]any{"type": "string", "description": "Due date in YYYY-MM-DD format."},
					"review_cycle_id": map[string]any{"type": "integer", "description": "Optional review cycle to link the goal to."},
				},
				"required": []string{"title"},
			}),
		},
		{
			Name:        "submit_self_review",
			Description: "Submit a self-assessment for a performance review. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"review_id": map[string]any{"type": "integer", "description": "Performance review ID (from get_my_performance_review)."},
					"rating":    map[string]any{"type": "integer", "description": "Self-rating 1-5 (1=Unsatisfactory, 3=Meets, 5=Outstanding)."},
					"comments":  map[string]any{"type": "string", "description": "Self-assessment comments."},
				},
				"required": []string{"review_id", "rating"},
			}),
		},
		{
			Name:        "submit_manager_review",
			Description: "Submit a manager review for an employee's performance. Manager/Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"review_id":      map[string]any{"type": "integer", "description": "Performance review ID."},
					"rating":         map[string]any{"type": "integer", "description": "Manager rating 1-5."},
					"comments":       map[string]any{"type": "string", "description": "Manager comments."},
					"strengths":      map[string]any{"type": "string", "description": "Employee strengths."},
					"improvements":   map[string]any{"type": "string", "description": "Areas for improvement."},
					"final_rating":   map[string]any{"type": "integer", "description": "Final overall rating 1-5. Defaults to manager rating if omitted."},
					"final_comments": map[string]any{"type": "string", "description": "Final review comments."},
				},
				"required": []string{"review_id", "rating"},
			}),
		},
	}
}

// registerPerformanceTools registers performance-related tool executors.
func (r *ToolRegistry) registerPerformanceTools() {
	r.tools["list_review_cycles"] = r.toolListReviewCycles
	r.tools["get_my_performance_review"] = r.toolGetMyPerformanceReview
	r.tools["create_goal"] = r.toolCreateGoal
	r.tools["submit_self_review"] = r.toolSubmitSelfReview
	r.tools["submit_manager_review"] = r.toolSubmitManagerReview
}
