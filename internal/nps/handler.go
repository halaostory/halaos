package nps

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// cooldown is the minimum interval between NPS submissions per user.
const cooldown = 30 * 24 * time.Hour // 30 days

// Handler handles NPS feedback endpoints.
type Handler struct {
	queries *store.Queries
	logger  *slog.Logger
}

// NewHandler creates an NPS handler.
func NewHandler(queries *store.Queries, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, logger: logger}
}

// RegisterRoutes adds NPS routes to the router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	nps := rg.Group("/nps")
	{
		nps.POST("", h.Submit)
		nps.GET("/status", h.Status)
		nps.GET("/summary", h.Summary)
	}
}

type submitRequest struct {
	Score   int    `json:"score" binding:"required,min=0,max=10"`
	Comment string `json:"comment"`
}

// Submit records an NPS response.
func (h *Handler) Submit(c *gin.Context) {
	var req submitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Score must be between 0 and 10")
		return
	}

	userID := auth.GetUserID(c)
	companyID := auth.GetCompanyID(c)

	// Check cooldown
	lastAt, err := h.queries.GetLastNPSFeedback(c.Request.Context(), userID)
	if err == nil && time.Since(lastAt) < cooldown {
		response.Error(c, http.StatusTooManyRequests, "RATE_LIMITED", "You have already submitted feedback recently. Please try again later.")
		return
	}

	row, err := h.queries.InsertNPSFeedback(c.Request.Context(), store.InsertNPSFeedbackParams{
		CompanyID: companyID,
		UserID:    userID,
		Score:     int16(req.Score),
		Comment:   req.Comment,
	})
	if err != nil {
		h.logger.Error("failed to insert NPS feedback", "error", err)
		response.InternalError(c, "Failed to save feedback")
		return
	}

	response.OK(c, gin.H{
		"id":         row.ID,
		"created_at": row.CreatedAt,
	})
}

// Status returns whether the current user should see the NPS prompt.
func (h *Handler) Status(c *gin.Context) {
	userID := auth.GetUserID(c)

	lastAt, err := h.queries.GetLastNPSFeedback(c.Request.Context(), userID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Never submitted — eligible
		response.OK(c, gin.H{"eligible": true, "last_submitted_at": nil})
		return
	}
	if err != nil {
		h.logger.Error("failed to get NPS status", "error", err)
		response.InternalError(c, "Failed to check feedback status")
		return
	}

	eligible := time.Since(lastAt) >= cooldown
	response.OK(c, gin.H{
		"eligible":          eligible,
		"last_submitted_at": lastAt,
	})
}

// Summary returns aggregate NPS stats (admin only).
func (h *Handler) Summary(c *gin.Context) {
	role := auth.GetRole(c)
	if role != "super_admin" && role != "admin" {
		response.Forbidden(c, "Admin access required")
		return
	}

	companyID := auth.GetCompanyID(c)

	summary, err := h.queries.GetNPSSummary(c.Request.Context(), companyID)
	if err != nil {
		h.logger.Error("failed to get NPS summary", "error", err)
		response.InternalError(c, "Failed to get NPS summary")
		return
	}

	// NPS score = %promoters - %detractors
	var npsScore float64
	if summary.TotalResponses > 0 {
		npsScore = float64(summary.Promoters-summary.Detractors) / float64(summary.TotalResponses) * 100
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	feedback, err := h.queries.ListNPSFeedback(c.Request.Context(), store.ListNPSFeedbackParams{
		CompanyID: companyID,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		h.logger.Error("failed to list NPS feedback", "error", err)
		response.InternalError(c, "Failed to list feedback")
		return
	}

	response.OK(c, gin.H{
		"nps_score":       npsScore,
		"total_responses": summary.TotalResponses,
		"avg_score":       summary.AvgScore,
		"promoters":       summary.Promoters,
		"passives":        summary.Passives,
		"detractors":      summary.Detractors,
		"feedback":        feedback,
	})
}
