package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/store"
)

func (r *ToolRegistry) toolQueryOrgOverview(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	snap, err := r.queries.GetLatestOrgSnapshot(ctx, companyID)
	if err != nil {
		return toJSON(map[string]any{
			"message": "No org intelligence data available yet. Scores are computed weekly on Monday.",
		})
	}

	return toJSON(map[string]any{
		"week_date":            snap.WeekDate.Format("2006-01-02"),
		"avg_flight_risk":      numericToString(snap.AvgFlightRisk),
		"avg_burnout":          numericToString(snap.AvgBurnout),
		"avg_team_health":      numericToString(snap.AvgTeamHealth),
		"high_risk_count":      snap.HighRiskCount,
		"high_burnout_count":   snap.HighBurnoutCount,
		"low_health_dept_count": snap.LowHealthDeptCount,
		"total_employees":      snap.TotalEmployees,
		"total_departments":    snap.TotalDepartments,
	})
}

func (r *ToolRegistry) toolQueryOrgTrends(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	weeks := 12
	if w, ok := input["weeks"].(float64); ok && w > 0 && w <= 52 {
		weeks = int(w)
	}

	since := time.Now().AddDate(0, 0, -weeks*7)
	trends, err := r.queries.GetOrgScoreTrend(ctx, store.GetOrgScoreTrendParams{
		CompanyID: companyID,
		WeekDate:  since,
	})
	if err != nil {
		return "", fmt.Errorf("get org trends: %w", err)
	}

	type trendRow struct {
		WeekDate       string `json:"week_date"`
		AvgFlightRisk  string `json:"avg_flight_risk"`
		AvgBurnout     string `json:"avg_burnout"`
		AvgTeamHealth  string `json:"avg_team_health"`
		HighRiskCount  int32  `json:"high_risk_count"`
		HighBurnout    int32  `json:"high_burnout_count"`
	}
	rows := make([]trendRow, len(trends))
	for i, t := range trends {
		rows[i] = trendRow{
			WeekDate:      t.WeekDate.Format("2006-01-02"),
			AvgFlightRisk: numericToString(t.AvgFlightRisk),
			AvgBurnout:    numericToString(t.AvgBurnout),
			AvgTeamHealth: numericToString(t.AvgTeamHealth),
			HighRiskCount: t.HighRiskCount,
			HighBurnout:   t.HighBurnoutCount,
		}
	}

	return toJSON(map[string]any{
		"weeks":  weeks,
		"trends": rows,
	})
}

func (r *ToolRegistry) toolQueryEmployeeRiskTrend(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	employeeID, ok := input["employee_id"].(float64)
	if !ok || employeeID <= 0 {
		return "", fmt.Errorf("employee_id is required")
	}

	weeks := 12
	if w, ok := input["weeks"].(float64); ok && w > 0 {
		weeks = int(w)
	}
	since := time.Now().AddDate(0, 0, -weeks*7)

	frTrend, _ := r.queries.GetFlightRiskTrend(ctx, store.GetFlightRiskTrendParams{
		CompanyID: companyID, EmployeeID: int64(employeeID), WeekDate: since,
	})
	boTrend, _ := r.queries.GetBurnoutTrend(ctx, store.GetBurnoutTrendParams{
		CompanyID: companyID, EmployeeID: int64(employeeID), WeekDate: since,
	})

	return toJSON(map[string]any{
		"employee_id": int64(employeeID),
		"flight_risk": frTrend,
		"burnout":     boTrend,
	})
}

func (r *ToolRegistry) toolQueryFlightRiskScores(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	scores, err := r.queries.ListAllFlightRiskScores(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("list flight risk scores: %w", err)
	}

	type item struct {
		EmployeeID int64  `json:"employee_id"`
		Name       string `json:"name"`
		EmployeeNo string `json:"employee_no"`
		Department string `json:"department"`
		RiskScore  int32  `json:"risk_score"`
	}
	n := len(scores)
	if n > 20 {
		n = 20
	}
	items := make([]item, n)
	for i := 0; i < n; i++ {
		items[i] = item{
			EmployeeID: scores[i].EmployeeID,
			Name:       scores[i].FirstName + " " + scores[i].LastName,
			EmployeeNo: scores[i].EmployeeNo,
			Department: scores[i].Department,
			RiskScore:  scores[i].RiskScore,
		}
	}

	return toJSON(map[string]any{
		"total":     len(scores),
		"employees": items,
	})
}

func (r *ToolRegistry) toolQueryBurnoutRiskScores(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	scores, err := r.queries.ListAllBurnoutScores(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("list burnout scores: %w", err)
	}

	type item struct {
		EmployeeID   int64  `json:"employee_id"`
		Name         string `json:"name"`
		EmployeeNo   string `json:"employee_no"`
		Department   string `json:"department"`
		BurnoutScore int32  `json:"burnout_score"`
	}
	n := len(scores)
	if n > 20 {
		n = 20
	}
	items := make([]item, n)
	for i := 0; i < n; i++ {
		items[i] = item{
			EmployeeID:   scores[i].EmployeeID,
			Name:         scores[i].FirstName + " " + scores[i].LastName,
			EmployeeNo:   scores[i].EmployeeNo,
			Department:   scores[i].Department,
			BurnoutScore: scores[i].BurnoutScore,
		}
	}

	return toJSON(map[string]any{
		"total":     len(scores),
		"employees": items,
	})
}

func (r *ToolRegistry) toolQueryTeamHealthScores(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	scores, err := r.queries.ListAllTeamHealthScores(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("list team health scores: %w", err)
	}

	type item struct {
		DepartmentID   int64  `json:"department_id"`
		DepartmentName string `json:"department_name"`
		HealthScore    int32  `json:"health_score"`
	}
	items := make([]item, len(scores))
	for i, s := range scores {
		items[i] = item{
			DepartmentID:   s.DepartmentID,
			DepartmentName: s.DepartmentName,
			HealthScore:    s.HealthScore,
		}
	}

	return toJSON(map[string]any{
		"total":       len(scores),
		"departments": items,
	})
}

func orgIntelDefs() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "query_org_overview",
			Description: "Get organization-level health overview: average flight risk, burnout, team health scores, high-risk counts. Manager+ only.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "query_org_trends",
			Description: "Get weekly organization score trends over time. Shows avg flight risk, burnout, and team health by week. Manager+ only.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"weeks": map[string]any{"type": "integer", "description": "Number of weeks to look back (default 12, max 52)."},
				},
			}),
		},
		{
			Name:        "query_employee_risk_trend",
			Description: "Get individual employee flight risk and burnout score trend over time. Manager+ only.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_id": map[string]any{"type": "integer", "description": "Employee ID to query."},
					"weeks":       map[string]any{"type": "integer", "description": "Number of weeks (default 12)."},
				},
				"required": []string{"employee_id"},
			}),
		},
		{
			Name:        "query_flight_risk_scores",
			Description: "Get top employees by flight risk score (sorted highest first, up to 20). Manager+ only.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "query_burnout_risk_scores",
			Description: "Get top employees by burnout risk score (sorted highest first, up to 20). Manager+ only.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "query_team_health_scores",
			Description: "Get all department health scores (sorted lowest first). Manager+ only.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
	}
}

func (r *ToolRegistry) registerOrgIntelTools() {
	r.tools["query_org_overview"] = r.toolQueryOrgOverview
	r.tools["query_org_trends"] = r.toolQueryOrgTrends
	r.tools["query_employee_risk_trend"] = r.toolQueryEmployeeRiskTrend
	r.tools["query_flight_risk_scores"] = r.toolQueryFlightRiskScores
	r.tools["query_burnout_risk_scores"] = r.toolQueryBurnoutRiskScores
	r.tools["query_team_health_scores"] = r.toolQueryTeamHealthScores
}
