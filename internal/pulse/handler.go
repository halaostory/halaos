package pulse

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger}
}

// CreateSurvey creates a new pulse survey with questions.
func (h *Handler) CreateSurvey(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	var req struct {
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
		Frequency   string `json:"frequency" binding:"required"`
		IsAnonymous bool   `json:"is_anonymous"`
		Questions   []struct {
			Question     string `json:"question" binding:"required"`
			QuestionType string `json:"question_type"`
			SortOrder    int32  `json:"sort_order"`
			IsRequired   bool   `json:"is_required"`
		} `json:"questions" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	ctx := c.Request.Context()

	desc := &req.Description
	if req.Description == "" {
		desc = nil
	}

	survey, err := h.queries.CreatePulseSurvey(ctx, store.CreatePulseSurveyParams{
		CompanyID:   companyID,
		Title:       req.Title,
		Description: desc,
		Frequency:   req.Frequency,
		IsAnonymous: req.IsAnonymous,
		CreatedBy:   userID,
	})
	if err != nil {
		h.logger.Error("failed to create pulse survey", "error", err)
		response.InternalError(c, "Failed to create survey")
		return
	}

	for _, q := range req.Questions {
		qType := q.QuestionType
		if qType == "" {
			qType = "rating"
		}
		_, err := h.queries.CreatePulseQuestion(ctx, store.CreatePulseQuestionParams{
			SurveyID:     survey.ID,
			Question:     q.Question,
			QuestionType: qType,
			SortOrder:    q.SortOrder,
			IsRequired:   q.IsRequired,
		})
		if err != nil {
			h.logger.Error("failed to create pulse question", "survey_id", survey.ID, "error", err)
		}
	}

	questions, _ := h.queries.ListPulseQuestions(ctx, survey.ID)

	response.Created(c, gin.H{
		"survey":    survey,
		"questions": questions,
	})
}

// ListSurveys lists pulse surveys for the company.
func (h *Handler) ListSurveys(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	limit := int32(20)
	offset := int32(0)
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = int32(v)
		}
	}
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = int32(v)
		}
	}

	surveys, err := h.queries.ListPulseSurveys(c.Request.Context(), store.ListPulseSurveysParams{
		CompanyID: companyID,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		h.logger.Error("failed to list pulse surveys", "error", err)
		response.InternalError(c, "Failed to list surveys")
		return
	}
	response.OK(c, surveys)
}

// GetSurvey returns a survey with its questions.
func (h *Handler) GetSurvey(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid survey ID")
		return
	}

	ctx := c.Request.Context()

	survey, err := h.queries.GetPulseSurvey(ctx, store.GetPulseSurveyParams{
		ID:        id,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Survey not found")
		return
	}

	questions, _ := h.queries.ListPulseQuestions(ctx, survey.ID)

	response.OK(c, gin.H{
		"survey":    survey,
		"questions": questions,
	})
}

// UpdateSurvey updates a survey and replaces its questions.
func (h *Handler) UpdateSurvey(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid survey ID")
		return
	}

	var req struct {
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
		Frequency   string `json:"frequency" binding:"required"`
		IsAnonymous bool   `json:"is_anonymous"`
		IsActive    bool   `json:"is_active"`
		Questions   []struct {
			Question     string `json:"question" binding:"required"`
			QuestionType string `json:"question_type"`
			SortOrder    int32  `json:"sort_order"`
			IsRequired   bool   `json:"is_required"`
		} `json:"questions" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	ctx := c.Request.Context()

	desc := &req.Description
	if req.Description == "" {
		desc = nil
	}

	survey, err := h.queries.UpdatePulseSurvey(ctx, store.UpdatePulseSurveyParams{
		ID:          id,
		CompanyID:   companyID,
		Title:       req.Title,
		Description: desc,
		Frequency:   req.Frequency,
		IsAnonymous: req.IsAnonymous,
		IsActive:    req.IsActive,
	})
	if err != nil {
		h.logger.Error("failed to update pulse survey", "error", err)
		response.InternalError(c, "Failed to update survey")
		return
	}

	// Replace questions
	_ = h.queries.DeletePulseQuestions(ctx, id)
	for _, q := range req.Questions {
		qType := q.QuestionType
		if qType == "" {
			qType = "rating"
		}
		_, err := h.queries.CreatePulseQuestion(ctx, store.CreatePulseQuestionParams{
			SurveyID:     id,
			Question:     q.Question,
			QuestionType: qType,
			SortOrder:    q.SortOrder,
			IsRequired:   q.IsRequired,
		})
		if err != nil {
			h.logger.Error("failed to create pulse question", "survey_id", id, "error", err)
		}
	}

	questions, _ := h.queries.ListPulseQuestions(ctx, id)

	response.OK(c, gin.H{
		"survey":    survey,
		"questions": questions,
	})
}

// DeactivateSurvey deactivates a survey.
func (h *Handler) DeactivateSurvey(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid survey ID")
		return
	}

	if err := h.queries.DeactivatePulseSurvey(c.Request.Context(), store.DeactivatePulseSurveyParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		h.logger.Error("failed to deactivate pulse survey", "error", err)
		response.InternalError(c, "Failed to deactivate survey")
		return
	}
	response.OK(c, gin.H{"message": "Survey deactivated"})
}

// GetOpenRound returns the current open round for a survey (for employees to respond).
func (h *Handler) GetOpenRound(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	surveyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid survey ID")
		return
	}

	ctx := c.Request.Context()

	round, err := h.queries.GetOpenRound(ctx, store.GetOpenRoundParams{
		SurveyID:  surveyID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "No open round")
		return
	}

	questions, _ := h.queries.ListPulseQuestions(ctx, surveyID)

	// Check if current user already responded
	userID := auth.GetUserID(c)
	emp, empErr := h.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	responded := false
	if empErr == nil {
		responded, _ = h.queries.HasEmployeeRespondedToRound(ctx, store.HasEmployeeRespondedToRoundParams{
			RoundID:    round.ID,
			EmployeeID: emp.ID,
		})
	}

	response.OK(c, gin.H{
		"round":     round,
		"questions": questions,
		"responded": responded,
	})
}

// SubmitResponse submits responses for a round.
func (h *Handler) SubmitResponse(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	roundID, err := strconv.ParseInt(c.Param("round_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid round ID")
		return
	}

	var req struct {
		Responses []struct {
			QuestionID int64   `json:"question_id" binding:"required"`
			Rating     *int32  `json:"rating"`
			AnswerText *string `json:"answer_text"`
		} `json:"responses" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	ctx := c.Request.Context()

	emp, err := h.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.BadRequest(c, "Employee not found")
		return
	}

	// Check not already responded
	already, _ := h.queries.HasEmployeeRespondedToRound(ctx, store.HasEmployeeRespondedToRoundParams{
		RoundID:    roundID,
		EmployeeID: emp.ID,
	})
	isFirstResponse := !already

	for _, r := range req.Responses {
		if err := h.queries.UpsertPulseResponse(ctx, store.UpsertPulseResponseParams{
			RoundID:    roundID,
			QuestionID: r.QuestionID,
			EmployeeID: emp.ID,
			CompanyID:  companyID,
			Rating:     r.Rating,
			AnswerText: r.AnswerText,
		}); err != nil {
			h.logger.Error("failed to upsert pulse response", "round_id", roundID, "question_id", r.QuestionID, "error", err)
		}
	}

	// Increment responded count only on first submission
	if isFirstResponse {
		_ = h.queries.IncrementRoundResponded(ctx, roundID)
	}

	response.OK(c, gin.H{"message": "Response submitted"})
}

// GetResults returns aggregated results for a survey round.
func (h *Handler) GetResults(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	surveyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid survey ID")
		return
	}

	ctx := c.Request.Context()

	// Get rounds
	rounds, err := h.queries.ListPulseRounds(ctx, store.ListPulseRoundsParams{
		SurveyID: surveyID,
		Limit:    20,
		Offset:   0,
	})
	if err != nil {
		response.InternalError(c, "Failed to list rounds")
		return
	}

	questions, _ := h.queries.ListPulseQuestions(ctx, surveyID)

	// Aggregate responses per round via raw query
	type QuestionResult struct {
		QuestionID   int64    `json:"question_id"`
		Question     string   `json:"question"`
		QuestionType string   `json:"question_type"`
		AvgRating    *float64 `json:"avg_rating,omitempty"`
		ResponseCount int     `json:"response_count"`
		Answers      []string `json:"answers,omitempty"`
	}
	type RoundResult struct {
		RoundID        int64            `json:"round_id"`
		RoundDate      string           `json:"round_date"`
		Status         string           `json:"status"`
		TotalSent      int32            `json:"total_sent"`
		TotalResponded int32            `json:"total_responded"`
		Questions      []QuestionResult `json:"questions"`
	}

	results := make([]RoundResult, 0, len(rounds))
	for _, round := range rounds {
		rr := RoundResult{
			RoundID:        round.ID,
			RoundDate:      round.RoundDate.Format("2006-01-02"),
			Status:         round.Status,
			TotalSent:      round.TotalSent,
			TotalResponded: round.TotalResponded,
		}

		for _, q := range questions {
			qr := QuestionResult{
				QuestionID:   q.ID,
				Question:     q.Question,
				QuestionType: q.QuestionType,
			}

			if q.QuestionType == "rating" {
				// Get average rating
				rows, err := h.pool.Query(ctx,
					`SELECT AVG(rating)::float8, COUNT(*) FROM pulse_responses WHERE round_id = $1 AND question_id = $2 AND rating IS NOT NULL`,
					round.ID, q.ID)
				if err == nil {
					if rows.Next() {
						var avg *float64
						var count int
						_ = rows.Scan(&avg, &count)
						qr.AvgRating = avg
						qr.ResponseCount = count
					}
					rows.Close()
				}
			} else {
				// Get text answers
				rows, err := h.pool.Query(ctx,
					`SELECT answer_text FROM pulse_responses WHERE round_id = $1 AND question_id = $2 AND answer_text IS NOT NULL AND company_id = $3`,
					round.ID, q.ID, companyID)
				if err == nil {
					var answers []string
					for rows.Next() {
						var text *string
						_ = rows.Scan(&text)
						if text != nil {
							answers = append(answers, *text)
						}
					}
					rows.Close()
					qr.Answers = answers
					qr.ResponseCount = len(answers)
				}
			}

			rr.Questions = append(rr.Questions, qr)
		}

		results = append(results, rr)
	}

	response.OK(c, gin.H{
		"survey_id": surveyID,
		"results":   results,
	})
}

// ListActiveSurveys returns active surveys with open rounds (for employees).
func (h *Handler) ListActiveSurveys(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	ctx := c.Request.Context()

	// Get all active surveys
	surveys, err := h.queries.ListPulseSurveys(ctx, store.ListPulseSurveysParams{
		CompanyID: companyID,
		Limit:     50,
		Offset:    0,
	})
	if err != nil {
		response.InternalError(c, "Failed to list surveys")
		return
	}

	emp, _ := h.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})

	type SurveyWithRound struct {
		store.PulseSurvey
		OpenRound *store.PulseRound `json:"open_round,omitempty"`
		Responded bool              `json:"responded"`
	}

	var result []SurveyWithRound
	for _, s := range surveys {
		if !s.IsActive {
			continue
		}
		sr := SurveyWithRound{PulseSurvey: s}
		round, err := h.queries.GetOpenRound(ctx, store.GetOpenRoundParams{
			SurveyID:  s.ID,
			CompanyID: companyID,
		})
		if err == nil {
			sr.OpenRound = &round
			if emp.ID > 0 {
				sr.Responded, _ = h.queries.HasEmployeeRespondedToRound(ctx, store.HasEmployeeRespondedToRoundParams{
					RoundID:    round.ID,
					EmployeeID: emp.ID,
				})
			}
		}
		result = append(result, sr)
	}

	response.OK(c, result)
}
