package orgintel

import (
	"encoding/json"
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
	queries   *store.Queries
	pool      *pgxpool.Pool
	logger    *slog.Logger
	briefGen  *BriefingGenerator
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger, briefGen *BriefingGenerator) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger, briefGen: briefGen}
}

// GetOverview returns the latest org snapshot with previous-week delta.
func (h *Handler) GetOverview(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	ctx := c.Request.Context()

	current, err := h.queries.GetLatestOrgSnapshot(ctx, companyID)
	if err != nil {
		response.OK(c, map[string]any{
			"current":  nil,
			"previous": nil,
			"deltas":   nil,
			"org_health_score": 0,
		})
		return
	}

	previous, prevErr := h.queries.GetPreviousOrgSnapshot(ctx, companyID)

	var deltas map[string]any
	if prevErr == nil {
		deltas = computeDeltas(current, previous)
	}

	orgHealth := computeOrgHealthScore(current)

	response.OK(c, map[string]any{
		"current":          current,
		"previous":         nilIfErr(previous, prevErr),
		"deltas":           deltas,
		"org_health_score": orgHealth,
	})
}

// GetTrends returns org-level weekly trends.
func (h *Handler) GetTrends(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	weeks := 12
	if w := c.Query("weeks"); w != "" {
		if v, err := strconv.Atoi(w); err == nil && v > 0 && v <= 52 {
			weeks = v
		}
	}

	since := time.Now().AddDate(0, 0, -weeks*7)
	trends, err := h.queries.GetOrgScoreTrend(c.Request.Context(), store.GetOrgScoreTrendParams{
		CompanyID: companyID,
		WeekDate:  since,
	})
	if err != nil {
		response.InternalError(c, "Failed to get org trends")
		return
	}
	response.OK(c, trends)
}

// GetRiskDistribution returns employee counts per risk tier.
func (h *Handler) GetRiskDistribution(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	tiers, err := h.queries.CountFlightRiskByTier(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get risk distribution")
		return
	}

	// Also get dept-level risk averages for the latest week
	since := time.Now().AddDate(0, 0, -7)
	deptRisks, _ := h.queries.GetDeptFlightRiskAvg(c.Request.Context(), store.GetDeptFlightRiskAvgParams{
		CompanyID: companyID,
		WeekDate:  since,
	})

	response.OK(c, map[string]any{
		"tiers":       tiers,
		"departments": deptRisks,
	})
}

// GetEmployeeTrends returns flight risk + burnout trend for a single employee.
func (h *Handler) GetEmployeeTrends(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	employeeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}

	weeks := 12
	if w := c.Query("weeks"); w != "" {
		if v, err := strconv.Atoi(w); err == nil && v > 0 && v <= 52 {
			weeks = v
		}
	}
	since := time.Now().AddDate(0, 0, -weeks*7)
	ctx := c.Request.Context()

	frTrend, _ := h.queries.GetFlightRiskTrend(ctx, store.GetFlightRiskTrendParams{
		CompanyID: companyID, EmployeeID: employeeID, WeekDate: since,
	})
	boTrend, _ := h.queries.GetBurnoutTrend(ctx, store.GetBurnoutTrendParams{
		CompanyID: companyID, EmployeeID: employeeID, WeekDate: since,
	})

	response.OK(c, map[string]any{
		"flight_risk": frTrend,
		"burnout":     boTrend,
	})
}

// GetDepartmentTrends returns health + risk + burnout trends for a department.
func (h *Handler) GetDepartmentTrends(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	deptID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid department ID")
		return
	}

	weeks := 12
	if w := c.Query("weeks"); w != "" {
		if v, err := strconv.Atoi(w); err == nil && v > 0 && v <= 52 {
			weeks = v
		}
	}
	since := time.Now().AddDate(0, 0, -weeks*7)
	ctx := c.Request.Context()

	healthTrend, _ := h.queries.GetTeamHealthTrend(ctx, store.GetTeamHealthTrendParams{
		CompanyID: companyID, DepartmentID: deptID, WeekDate: since,
	})

	response.OK(c, map[string]any{
		"team_health": healthTrend,
	})
}

// GetCorrelations returns cross-metric overlap counts.
func (h *Handler) GetCorrelations(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	ctx := c.Request.Context()

	highBurnoutHighRisk, _ := h.queries.GetCorrelationHighBurnoutHighRisk(ctx, companyID)

	// Get dept-level averages for cross-comparison
	since := time.Now().AddDate(0, 0, -7)
	deptRisks, _ := h.queries.GetDeptFlightRiskAvg(ctx, store.GetDeptFlightRiskAvgParams{
		CompanyID: companyID, WeekDate: since,
	})
	deptBurnout, _ := h.queries.GetDeptBurnoutAvg(ctx, store.GetDeptBurnoutAvgParams{
		CompanyID: companyID, WeekDate: since,
	})

	response.OK(c, map[string]any{
		"high_burnout_and_high_risk": highBurnoutHighRisk,
		"dept_risk_averages":         deptRisks,
		"dept_burnout_averages":      deptBurnout,
	})
}

// GetExecutiveBriefing returns the latest cached briefing.
func (h *Handler) GetExecutiveBriefing(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	briefing, err := h.queries.GetLatestExecutiveBriefing(c.Request.Context(), companyID)
	if err != nil {
		response.OK(c, nil)
		return
	}
	response.OK(c, briefing)
}

// GenerateExecutiveBriefing forces generation of a new briefing.
func (h *Handler) GenerateExecutiveBriefing(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	if h.briefGen == nil {
		response.BadRequest(c, "AI is not enabled")
		return
	}

	briefing, err := h.briefGen.Generate(c.Request.Context(), companyID)
	if err != nil {
		h.logger.Error("failed to generate executive briefing", "company_id", companyID, "error", err)
		response.InternalError(c, "Failed to generate briefing")
		return
	}

	response.OK(c, briefing)
}

// computeOrgHealthScore calculates a weighted org health score.
// team_health 40%, inverted flight_risk 30%, inverted burnout 30%
func computeOrgHealthScore(snap store.OrgScoreSnapshot) float64 {
	fr := numericFloat(snap.AvgFlightRisk)
	bo := numericFloat(snap.AvgBurnout)
	th := numericFloat(snap.AvgTeamHealth)

	score := th*0.4 + (100-fr)*0.3 + (100-bo)*0.3
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}

func computeDeltas(current, previous store.OrgScoreSnapshot) map[string]any {
	return map[string]any{
		"avg_flight_risk":      numericFloat(current.AvgFlightRisk) - numericFloat(previous.AvgFlightRisk),
		"avg_burnout":          numericFloat(current.AvgBurnout) - numericFloat(previous.AvgBurnout),
		"avg_team_health":      numericFloat(current.AvgTeamHealth) - numericFloat(previous.AvgTeamHealth),
		"high_risk_count":      current.HighRiskCount - previous.HighRiskCount,
		"high_burnout_count":   current.HighBurnoutCount - previous.HighBurnoutCount,
		"low_health_dept_count": current.LowHealthDeptCount - previous.LowHealthDeptCount,
	}
}

func numericFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

func nilIfErr(v any, err error) any {
	if err != nil {
		return nil
	}
	return v
}

// topN returns up to n items from a scored list (already sorted desc).
func topNFlightRisk(items []store.ListAllFlightRiskScoresRow, n int) []map[string]any {
	if len(items) < n {
		n = len(items)
	}
	result := make([]map[string]any, n)
	for i := 0; i < n; i++ {
		var factors []map[string]string
		_ = json.Unmarshal(items[i].Factors, &factors)
		result[i] = map[string]any{
			"employee_id": items[i].EmployeeID,
			"name":        items[i].FirstName + " " + items[i].LastName,
			"employee_no": items[i].EmployeeNo,
			"department":  items[i].Department,
			"risk_score":  items[i].RiskScore,
			"factors":     factors,
		}
	}
	return result
}

func topNBurnout(items []store.ListAllBurnoutScoresRow, n int) []map[string]any {
	if len(items) < n {
		n = len(items)
	}
	result := make([]map[string]any, n)
	for i := 0; i < n; i++ {
		var factors []map[string]string
		_ = json.Unmarshal(items[i].Factors, &factors)
		result[i] = map[string]any{
			"employee_id":  items[i].EmployeeID,
			"name":         items[i].FirstName + " " + items[i].LastName,
			"employee_no":  items[i].EmployeeNo,
			"department":   items[i].Department,
			"burnout_score": items[i].BurnoutScore,
			"factors":       factors,
		}
	}
	return result
}
