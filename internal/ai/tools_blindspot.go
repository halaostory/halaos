package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/store"
)

func (r *ToolRegistry) toolQueryBlindSpots(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	// Get manager's employee ID
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	weekDate := time.Now().Truncate(24 * time.Hour)
	// Find most recent Monday
	for weekDate.Weekday() != time.Monday {
		weekDate = weekDate.AddDate(0, 0, -1)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT spot_type, severity, title, description, employees, is_resolved, created_at
		FROM manager_blind_spots
		WHERE company_id = $1
		  AND manager_id = $2
		  AND week_date >= $3 - INTERVAL '30 days'
		ORDER BY
		  CASE severity WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END,
		  created_at DESC
		LIMIT 20
	`, companyID, emp.ID, weekDate)
	if err != nil {
		return "", fmt.Errorf("query blind spots: %w", err)
	}
	defer rows.Close()

	type spotResult struct {
		SpotType    string `json:"spot_type"`
		Severity    string `json:"severity"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Employees   []byte `json:"employees"`
		IsResolved  bool   `json:"is_resolved"`
		CreatedAt   string `json:"created_at"`
	}

	var spots []spotResult
	for rows.Next() {
		var s spotResult
		var createdAt time.Time
		if err := rows.Scan(&s.SpotType, &s.Severity, &s.Title, &s.Description, &s.Employees, &s.IsResolved, &createdAt); err != nil {
			return "", fmt.Errorf("scan: %w", err)
		}
		s.CreatedAt = createdAt.Format("2006-01-02")
		spots = append(spots, s)
	}

	if len(spots) == 0 {
		return `{"message":"No blind spots detected for your team. Great job staying on top of things!","spots":[]}`, nil
	}

	return toJSON(map[string]any{
		"total": len(spots),
		"spots": spots,
	})
}

func blindspotDefs() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "query_blind_spots",
			Description: "Get manager blind spot insights — patterns in your team that you might be overlooking. Returns weekly analysis of chronic tardiness, overtime concentration, leave imbalance, high flight risk, and feedback gaps. Manager only.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
	}
}

func (r *ToolRegistry) registerBlindspotTools() {
	r.tools["query_blind_spots"] = r.toolQueryBlindSpots
}
