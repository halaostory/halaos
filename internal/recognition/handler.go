package recognition

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
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

// SendRecognition creates a new recognition.
func (h *Handler) SendRecognition(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	var req struct {
		ToEmployeeID int64  `json:"to_employee_id" binding:"required"`
		Category     string `json:"category"`
		Message      string `json:"message" binding:"required"`
		IsPublic     bool   `json:"is_public"`
		Points       int32  `json:"points"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	ctx := c.Request.Context()

	// Resolve sender employee
	fromEmp, err := h.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.BadRequest(c, "Sender employee not found")
		return
	}

	if fromEmp.ID == req.ToEmployeeID {
		response.BadRequest(c, "Cannot recognize yourself")
		return
	}

	category := req.Category
	if category == "" {
		category = "kudos"
	}
	points := req.Points
	if points <= 0 {
		points = 1
	}

	rec, err := h.queries.CreateRecognition(ctx, store.CreateRecognitionParams{
		CompanyID:      companyID,
		FromEmployeeID: fromEmp.ID,
		ToEmployeeID:   req.ToEmployeeID,
		Category:       category,
		Message:        req.Message,
		IsPublic:       req.IsPublic,
		Points:         points,
	})
	if err != nil {
		h.logger.Error("failed to create recognition", "error", err)
		response.InternalError(c, "Failed to send recognition")
		return
	}

	response.Created(c, rec)
}

// ListRecognitions returns public recognition wall.
func (h *Handler) ListRecognitions(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	limit := int32(30)
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

	recs, err := h.queries.ListRecognitions(c.Request.Context(), store.ListRecognitionsParams{
		CompanyID: companyID,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		h.logger.Error("failed to list recognitions", "error", err)
		response.InternalError(c, "Failed to list recognitions")
		return
	}
	response.OK(c, recs)
}

// ListMyRecognitions returns recognitions given/received by current user.
func (h *Handler) ListMyRecognitions(c *gin.Context) {
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

	limit := int32(30)
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

	recs, err := h.queries.ListMyRecognitions(c.Request.Context(), store.ListMyRecognitionsParams{
		CompanyID:    companyID,
		ToEmployeeID: emp.ID,
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		response.InternalError(c, "Failed to list recognitions")
		return
	}
	response.OK(c, recs)
}

// GetStats returns recognition statistics and leaderboard.
func (h *Handler) GetStats(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	since := time.Now().AddDate(0, -1, 0) // last 30 days

	ctx := c.Request.Context()

	stats, err := h.queries.GetRecognitionStats(ctx, store.GetRecognitionStatsParams{
		CompanyID: companyID,
		CreatedAt: since,
	})
	if err != nil {
		response.InternalError(c, "Failed to get stats")
		return
	}

	topRecognized, _ := h.queries.GetTopRecognized(ctx, store.GetTopRecognizedParams{
		CompanyID: companyID,
		CreatedAt: since,
		Limit:     10,
	})

	categories, _ := h.queries.GetCategoryBreakdown(ctx, store.GetCategoryBreakdownParams{
		CompanyID: companyID,
		CreatedAt: since,
	})

	response.OK(c, gin.H{
		"stats":          stats,
		"top_recognized": topRecognized,
		"categories":     categories,
	})
}
