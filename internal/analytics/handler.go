package analytics

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

// GetSummary returns high-level analytics summary
func (h *Handler) GetSummary(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	summary, err := h.queries.GetAnalyticsSummary(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get analytics summary")
		return
	}
	response.OK(c, summary)
}

// GetHeadcountTrend returns monthly headcount trend
func (h *Handler) GetHeadcountTrend(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	months := 12
	if m := c.Query("months"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v > 0 {
			months = v
		}
	}

	since := time.Now().AddDate(0, -months, 0)
	trend, err := h.queries.GetHeadcountTrend(c.Request.Context(), store.GetHeadcountTrendParams{
		CompanyID: companyID,
		HireDate:  since,
	})
	if err != nil {
		response.InternalError(c, "Failed to get headcount trend")
		return
	}
	response.OK(c, trend)
}

// GetTurnoverStats returns monthly turnover statistics
func (h *Handler) GetTurnoverStats(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	since := time.Now().AddDate(-1, 0, 0)

	stats, err := h.queries.GetTurnoverStats(c.Request.Context(), store.GetTurnoverStatsParams{
		CompanyID: companyID,
		UpdatedAt: since,
	})
	if err != nil {
		response.InternalError(c, "Failed to get turnover stats")
		return
	}
	response.OK(c, stats)
}

// GetDepartmentCosts returns department cost analysis
func (h *Handler) GetDepartmentCosts(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	costs, err := h.queries.GetDepartmentCostAnalysis(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get department costs")
		return
	}
	response.OK(c, costs)
}

// GetAttendancePatterns returns attendance patterns by day of week
func (h *Handler) GetAttendancePatterns(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	since := time.Now().AddDate(0, -3, 0) // last 3 months

	patterns, err := h.queries.GetAttendancePatterns(c.Request.Context(), store.GetAttendancePatternsParams{
		CompanyID: companyID,
		ClockInAt: pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		response.InternalError(c, "Failed to get attendance patterns")
		return
	}
	response.OK(c, patterns)
}

// GetEmploymentTypeBreakdown returns breakdown by employment type
func (h *Handler) GetEmploymentTypeBreakdown(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	breakdown, err := h.queries.GetEmploymentTypeBreakdown(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get employment type breakdown")
		return
	}
	response.OK(c, breakdown)
}

// GetLeaveUtilization returns leave utilization by type
func (h *Handler) GetLeaveUtilization(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	year := int32(time.Now().Year())
	if y := c.Query("year"); y != "" {
		if v, err := strconv.ParseInt(y, 10, 32); err == nil {
			year = int32(v)
		}
	}

	// Use Jan 1 of the target year as the start_date param
	// (sqlc interprets EXTRACT(YEAR FROM lr.start_date) = $2 as start_date param)
	yearStart := time.Date(int(year), 1, 1, 0, 0, 0, 0, time.UTC)
	util, err := h.queries.GetLeaveUtilization(c.Request.Context(), store.GetLeaveUtilizationParams{
		CompanyID: companyID,
		StartDate: yearStart,
	})
	if err != nil {
		response.InternalError(c, "Failed to get leave utilization")
		return
	}
	response.OK(c, util)
}
