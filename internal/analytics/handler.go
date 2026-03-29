package analytics

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
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

// parseStartDate parses ?start_date=YYYY-MM-DD, defaulting to fallback.
func parseStartDate(c *gin.Context, fallback time.Time) time.Time {
	if s := c.Query("start_date"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			return t
		}
	}
	return fallback
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

	since := parseStartDate(c, time.Now().AddDate(0, -months, 0))
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
	since := parseStartDate(c, time.Now().AddDate(-1, 0, 0))

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
	since := parseStartDate(c, time.Now().AddDate(0, -3, 0))

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

// GetBlindSpots returns recent manager blind spots for the current company.
func (h *Handler) GetBlindSpots(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	ctx := c.Request.Context()

	rows, err := h.pool.Query(ctx, `
		SELECT id, manager_id, spot_type, severity, title, description, employees, is_resolved, week_date, created_at
		FROM manager_blind_spots
		WHERE company_id = $1
		ORDER BY
		  CASE severity WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END,
		  created_at DESC
		LIMIT 50
	`, companyID)
	if err != nil {
		response.InternalError(c, "Failed to get blind spots")
		return
	}
	defer rows.Close()

	type blindSpotRow struct {
		ID          int64  `json:"id"`
		ManagerID   int64  `json:"manager_id"`
		SpotType    string `json:"spot_type"`
		Severity    string `json:"severity"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Employees   []byte `json:"employees"`
		IsResolved  bool   `json:"is_resolved"`
		WeekDate    string `json:"week_date"`
		CreatedAt   string `json:"created_at"`
	}

	var spots []blindSpotRow
	for rows.Next() {
		var s blindSpotRow
		var weekDate, createdAt time.Time
		if err := rows.Scan(&s.ID, &s.ManagerID, &s.SpotType, &s.Severity, &s.Title, &s.Description, &s.Employees, &s.IsResolved, &weekDate, &createdAt); err != nil {
			h.logger.Error("failed to scan blind spot", "error", err)
			continue
		}
		s.WeekDate = weekDate.Format("2006-01-02")
		s.CreatedAt = createdAt.Format("2006-01-02T15:04:05Z")
		spots = append(spots, s)
	}
	if spots == nil {
		spots = []blindSpotRow{}
	}

	response.OK(c, spots)
}

// ExportCSV exports analytics data as CSV
func (h *Handler) ExportCSV(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	ctx := c.Request.Context()

	filename := fmt.Sprintf("analytics_%s.csv", time.Now().Format("20060102"))
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	w := csv.NewWriter(c.Writer)
	defer w.Flush()

	// Summary
	summary, err := h.queries.GetAnalyticsSummary(ctx, companyID)
	if err == nil {
		w.Write([]string{"=== HR Summary ==="})
		w.Write([]string{"Metric", "Value"})
		w.Write([]string{"Active Employees", fmt.Sprintf("%d", summary.ActiveEmployees)})
		w.Write([]string{"Separated Employees", fmt.Sprintf("%d", summary.SeparatedEmployees)})
		w.Write([]string{"New Hires This Month", fmt.Sprintf("%d", summary.NewHiresThisMonth)})
		w.Write([]string{"Probationary", fmt.Sprintf("%d", summary.ProbationaryCount)})
		w.Write([]string{"Avg Tenure (Years)", fmt.Sprintf("%v", summary.AvgTenureYears)})
		w.Write([]string{""})
	}

	// Headcount trend
	since := parseStartDate(c, time.Now().AddDate(-1, 0, 0))
	trend, err := h.queries.GetHeadcountTrend(ctx, store.GetHeadcountTrendParams{
		CompanyID: companyID,
		HireDate:  since,
	})
	if err == nil && len(trend) > 0 {
		w.Write([]string{"=== Headcount Trend ==="})
		w.Write([]string{"Month", "Active", "Separated", "Total"})
		for _, t := range trend {
			w.Write([]string{
				t.Month,
				fmt.Sprintf("%d", t.ActiveCount),
				fmt.Sprintf("%d", t.SeparatedCount),
				fmt.Sprintf("%d", t.TotalCount),
			})
		}
		w.Write([]string{""})
	}

	// Turnover
	stats, err := h.queries.GetTurnoverStats(ctx, store.GetTurnoverStatsParams{
		CompanyID: companyID,
		UpdatedAt: since,
	})
	if err == nil && len(stats) > 0 {
		w.Write([]string{"=== Turnover ==="})
		w.Write([]string{"Month", "Separations", "Active Count"})
		for _, s := range stats {
			w.Write([]string{
				s.Month,
				fmt.Sprintf("%d", s.Separations),
				fmt.Sprintf("%d", s.ActiveCount),
			})
		}
		w.Write([]string{""})
	}

	// Department costs
	costs, err := h.queries.GetDepartmentCostAnalysis(ctx, companyID)
	if err == nil && len(costs) > 0 {
		w.Write([]string{"=== Department Costs ==="})
		w.Write([]string{"Department", "Employees", "Total Salary Cost"})
		for _, dc := range costs {
			costStr := fmt.Sprintf("%v", dc.TotalSalaryCost)
			w.Write([]string{
				fmt.Sprintf("%v", dc.DepartmentName),
				fmt.Sprintf("%d", dc.EmployeeCount),
				costStr,
			})
		}
		w.Write([]string{""})
	}

	// Employment types
	breakdown, err := h.queries.GetEmploymentTypeBreakdown(ctx, companyID)
	if err == nil && len(breakdown) > 0 {
		w.Write([]string{"=== Employment Types ==="})
		w.Write([]string{"Type", "Count"})
		for _, b := range breakdown {
			w.Write([]string{
				fmt.Sprintf("%v", b.EmploymentType),
				fmt.Sprintf("%d", b.Count),
			})
		}
		w.Write([]string{""})
	}

	// Leave utilization
	year := int32(time.Now().Year())
	if y := c.Query("year"); y != "" {
		if v, err := strconv.ParseInt(y, 10, 32); err == nil {
			year = int32(v)
		}
	}
	yearStart := time.Date(int(year), 1, 1, 0, 0, 0, 0, time.UTC)
	util, err := h.queries.GetLeaveUtilization(ctx, store.GetLeaveUtilizationParams{
		CompanyID: companyID,
		StartDate: yearStart,
	})
	if err == nil && len(util) > 0 {
		w.Write([]string{fmt.Sprintf("=== Leave Utilization (%d) ===", year)})
		w.Write([]string{"Leave Type", "Requests", "Days Used"})
		for _, u := range util {
			daysStr := fmt.Sprintf("%v", u.TotalDaysUsed)
			w.Write([]string{
				fmt.Sprintf("%v", u.LeaveType),
				fmt.Sprintf("%d", u.TotalRequests),
				daysStr,
			})
		}
	}
}
