package performance

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger}
}

// --- Review Cycles ---

func (h *Handler) ListCycles(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	cycles, err := h.queries.ListReviewCycles(c.Request.Context(), store.ListReviewCyclesParams{
		CompanyID: companyID,
		Limit:     50,
		Offset:    0,
	})
	if err != nil {
		response.InternalError(c, "Failed to list review cycles")
		return
	}
	response.OK(c, cycles)
}

func (h *Handler) CreateCycle(c *gin.Context) {
	var req struct {
		Name           string  `json:"name" binding:"required"`
		CycleType      string  `json:"cycle_type"`
		PeriodStart    string  `json:"period_start" binding:"required"`
		PeriodEnd      string  `json:"period_end" binding:"required"`
		ReviewDeadline *string `json:"review_deadline"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	start, _ := time.Parse("2006-01-02", req.PeriodStart)
	end, _ := time.Parse("2006-01-02", req.PeriodEnd)

	cycleType := req.CycleType
	if cycleType == "" {
		cycleType = "annual"
	}

	var deadline pgtype.Date
	if req.ReviewDeadline != nil {
		if d, err := time.Parse("2006-01-02", *req.ReviewDeadline); err == nil {
			deadline = pgtype.Date{Time: d, Valid: true}
		}
	}

	cycle, err := h.queries.CreateReviewCycle(c.Request.Context(), store.CreateReviewCycleParams{
		CompanyID:      companyID,
		Name:           req.Name,
		CycleType:      cycleType,
		PeriodStart:    start,
		PeriodEnd:      end,
		ReviewDeadline: deadline,
		CreatedBy:      &userID,
	})
	if err != nil {
		h.logger.Error("failed to create review cycle", "error", err)
		response.InternalError(c, "Failed to create review cycle")
		return
	}
	response.Created(c, cycle)
}

func (h *Handler) InitiateReviews(c *gin.Context) {
	cycleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid cycle ID")
		return
	}

	companyID := auth.GetCompanyID(c)

	// Create review records for all active employees
	if err := h.queries.InitiateReviews(c.Request.Context(), store.InitiateReviewsParams{
		CompanyID:     companyID,
		ReviewCycleID: cycleID,
	}); err != nil {
		h.logger.Error("failed to initiate reviews", "error", err)
		response.InternalError(c, "Failed to initiate reviews")
		return
	}

	// Mark cycle as active
	cycle, err := h.queries.UpdateReviewCycleStatus(c.Request.Context(), store.UpdateReviewCycleStatusParams{
		ID:        cycleID,
		CompanyID: companyID,
		Status:    "active",
	})
	if err != nil {
		response.InternalError(c, "Failed to activate cycle")
		return
	}

	response.OK(c, cycle)
}

// --- Reviews ---

func (h *Handler) ListReviewsByCycle(c *gin.Context) {
	cycleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid cycle ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	reviews, err := h.queries.ListReviewsByCycle(c.Request.Context(), store.ListReviewsByCycleParams{
		CompanyID:     companyID,
		ReviewCycleID: cycleID,
	})
	if err != nil {
		response.InternalError(c, "Failed to list reviews")
		return
	}

	// Also get stats
	stats, _ := h.queries.GetReviewStats(c.Request.Context(), store.GetReviewStatsParams{
		CompanyID:     companyID,
		ReviewCycleID: cycleID,
	})

	response.OK(c, gin.H{
		"reviews": reviews,
		"stats":   stats,
	})
}

func (h *Handler) ListMyReviews(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.OK(c, []any{})
		return
	}

	reviews, err := h.queries.ListMyReviews(c.Request.Context(), emp.ID)
	if err != nil {
		response.InternalError(c, "Failed to list reviews")
		return
	}
	response.OK(c, reviews)
}

func (h *Handler) GetReview(c *gin.Context) {
	reviewID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid review ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	review, err := h.queries.GetPerformanceReview(c.Request.Context(), store.GetPerformanceReviewParams{
		ID:        reviewID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Review not found")
		return
	}

	// Get goals for this employee+cycle
	cycleID := review.ReviewCycleID
	goals, _ := h.queries.ListGoalsByCycle(c.Request.Context(), store.ListGoalsByCycleParams{
		CompanyID:     companyID,
		ReviewCycleID: &cycleID,
	})

	// Filter to just this employee's goals
	var empGoals []store.ListGoalsByCycleRow
	for _, g := range goals {
		if g.EmployeeID == review.EmployeeID {
			empGoals = append(empGoals, g)
		}
	}

	response.OK(c, gin.H{
		"review": review,
		"goals":  empGoals,
	})
}

func (h *Handler) SubmitSelfReview(c *gin.Context) {
	reviewID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid review ID")
		return
	}

	var req struct {
		SelfRating   int32  `json:"self_rating" binding:"required"`
		SelfComments string `json:"self_comments"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)

	review, err := h.queries.SubmitSelfReview(c.Request.Context(), store.SubmitSelfReviewParams{
		ID:           reviewID,
		CompanyID:    companyID,
		SelfRating:   &req.SelfRating,
		SelfComments: &req.SelfComments,
	})
	if err != nil {
		response.InternalError(c, "Failed to submit self review")
		return
	}
	response.OK(c, review)
}

func (h *Handler) SubmitManagerReview(c *gin.Context) {
	reviewID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid review ID")
		return
	}

	var req struct {
		ManagerRating   int32   `json:"manager_rating" binding:"required"`
		ManagerComments *string `json:"manager_comments"`
		Strengths       *string `json:"strengths"`
		Improvements    *string `json:"improvements"`
		FinalRating     int32   `json:"final_rating" binding:"required"`
		FinalComments   *string `json:"final_comments"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)

	review, err := h.queries.SubmitManagerReview(c.Request.Context(), store.SubmitManagerReviewParams{
		ID:              reviewID,
		CompanyID:       companyID,
		ManagerRating:   &req.ManagerRating,
		ManagerComments: req.ManagerComments,
		Strengths:       req.Strengths,
		Improvements:    req.Improvements,
		FinalRating:     &req.FinalRating,
		FinalComments:   req.FinalComments,
	})
	if err != nil {
		response.InternalError(c, "Failed to submit manager review")
		return
	}
	response.OK(c, review)
}

// --- Goals ---

func (h *Handler) ListGoals(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.OK(c, []any{})
		return
	}

	employeeID := emp.ID
	if eid := c.Query("employee_id"); eid != "" {
		if id, err := strconv.ParseInt(eid, 10, 64); err == nil {
			employeeID = id
		}
	}

	goals, err := h.queries.ListGoals(c.Request.Context(), store.ListGoalsParams{
		CompanyID:  companyID,
		EmployeeID: employeeID,
		Limit:      50,
		Offset:     0,
	})
	if err != nil {
		response.InternalError(c, "Failed to list goals")
		return
	}
	response.OK(c, goals)
}

func (h *Handler) CreateGoal(c *gin.Context) {
	var req struct {
		EmployeeID    int64   `json:"employee_id" binding:"required"`
		ReviewCycleID *int64  `json:"review_cycle_id"`
		Title         string  `json:"title" binding:"required"`
		Description   *string `json:"description"`
		Category      string  `json:"category"`
		Weight        float64 `json:"weight"`
		TargetValue   *string `json:"target_value"`
		DueDate       *string `json:"due_date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)

	category := req.Category
	if category == "" {
		category = "individual"
	}

	var weightNum pgtype.Numeric
	_ = weightNum.Scan(strconv.FormatFloat(req.Weight, 'f', 2, 64))

	var dueDate pgtype.Date
	if req.DueDate != nil {
		if d, err := time.Parse("2006-01-02", *req.DueDate); err == nil {
			dueDate = pgtype.Date{Time: d, Valid: true}
		}
	}

	goal, err := h.queries.CreateGoal(c.Request.Context(), store.CreateGoalParams{
		CompanyID:     companyID,
		EmployeeID:    req.EmployeeID,
		ReviewCycleID: req.ReviewCycleID,
		Title:         req.Title,
		Description:   req.Description,
		Category:      category,
		Weight:        weightNum,
		TargetValue:   req.TargetValue,
		DueDate:       dueDate,
	})
	if err != nil {
		h.logger.Error("failed to create goal", "error", err)
		response.InternalError(c, "Failed to create goal")
		return
	}
	response.Created(c, goal)
}

func (h *Handler) UpdateGoal(c *gin.Context) {
	goalID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid goal ID")
		return
	}

	var req struct {
		Title         string  `json:"title"`
		Description   *string `json:"description"`
		Status        string  `json:"status"`
		ActualValue   *string `json:"actual_value"`
		SelfRating    *int32  `json:"self_rating"`
		ManagerRating *int32  `json:"manager_rating"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)

	goal, err := h.queries.UpdateGoal(c.Request.Context(), store.UpdateGoalParams{
		ID:            goalID,
		CompanyID:     companyID,
		Column3:       req.Title,
		Description:   req.Description,
		Column5:       req.Status,
		ActualValue:   req.ActualValue,
		SelfRating:    req.SelfRating,
		ManagerRating: req.ManagerRating,
	})
	if err != nil {
		response.NotFound(c, "Goal not found")
		return
	}
	response.OK(c, goal)
}
