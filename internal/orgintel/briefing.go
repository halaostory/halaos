package orgintel

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/store"
)

// BriefingGenerator generates AI-powered executive briefings.
type BriefingGenerator struct {
	queries  *store.Queries
	provider provider.Provider
	logger   *slog.Logger
}

// NewBriefingGenerator creates a new BriefingGenerator.
func NewBriefingGenerator(queries *store.Queries, prov provider.Provider, logger *slog.Logger) *BriefingGenerator {
	return &BriefingGenerator{queries: queries, provider: prov, logger: logger}
}

// BriefingResult holds the generated briefing data.
type BriefingResult struct {
	Narrative  string    `json:"narrative"`
	WeekDate   time.Time `json:"week_date"`
	TokensUsed int       `json:"tokens_used"`
}

// Generate creates an executive briefing for the given company.
func (bg *BriefingGenerator) Generate(ctx context.Context, companyID int64) (*BriefingResult, error) {
	weekDate := mondayOfWeek(time.Now())

	// Gather data
	data, err := bg.gatherBriefingData(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("gather briefing data: %w", err)
	}

	dataJSON, _ := json.Marshal(data)

	// Build prompt
	prompt := buildBriefingPrompt(data)

	resp, err := bg.provider.Generate(ctx, provider.Request{
		System: `You are an HR analytics expert generating a weekly executive briefing.
Write a clear, actionable narrative covering: Executive Summary, Key Metrics, Risk Highlights, Recommendations, and Positive Trends.
Use markdown formatting. Keep it concise but insightful. Focus on actionable insights.`,
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: prompt},
		},
		MaxTokens: 2000,
	})
	if err != nil {
		return nil, fmt.Errorf("generate briefing: %w", err)
	}

	tokensUsed := resp.Usage.InputTokens + resp.Usage.OutputTokens

	// Cache in database
	if err := bg.queries.UpsertExecutiveBriefing(ctx, store.UpsertExecutiveBriefingParams{
		CompanyID:    companyID,
		WeekDate:     weekDate,
		Narrative:    resp.Content,
		DataSnapshot: json.RawMessage(dataJSON),
		TokensUsed:   int32(tokensUsed),
	}); err != nil {
		bg.logger.Error("failed to cache briefing", "company_id", companyID, "error", err)
	}

	return &BriefingResult{
		Narrative:  resp.Content,
		WeekDate:   weekDate,
		TokensUsed: tokensUsed,
	}, nil
}

type briefingData struct {
	WeekDate       string         `json:"week_date"`
	Current        map[string]any `json:"current_snapshot"`
	Previous       map[string]any `json:"previous_snapshot"`
	TopFlightRisk  []map[string]any `json:"top_flight_risk"`
	TopBurnout     []map[string]any `json:"top_burnout"`
	LowestHealth   []map[string]any `json:"lowest_health_depts"`
}

func (bg *BriefingGenerator) gatherBriefingData(ctx context.Context, companyID int64) (*briefingData, error) {
	data := &briefingData{
		WeekDate: mondayOfWeek(time.Now()).Format("2006-01-02"),
	}

	// Current org snapshot
	current, err := bg.queries.GetLatestOrgSnapshot(ctx, companyID)
	if err == nil {
		data.Current = map[string]any{
			"avg_flight_risk":      numericFloat(current.AvgFlightRisk),
			"avg_burnout":          numericFloat(current.AvgBurnout),
			"avg_team_health":      numericFloat(current.AvgTeamHealth),
			"high_risk_count":      current.HighRiskCount,
			"high_burnout_count":   current.HighBurnoutCount,
			"low_health_dept_count": current.LowHealthDeptCount,
			"total_employees":      current.TotalEmployees,
			"total_departments":    current.TotalDepartments,
		}
	}

	// Previous week snapshot
	previous, err := bg.queries.GetPreviousOrgSnapshot(ctx, companyID)
	if err == nil {
		data.Previous = map[string]any{
			"avg_flight_risk":    numericFloat(previous.AvgFlightRisk),
			"avg_burnout":        numericFloat(previous.AvgBurnout),
			"avg_team_health":    numericFloat(previous.AvgTeamHealth),
			"high_risk_count":    previous.HighRiskCount,
			"high_burnout_count": previous.HighBurnoutCount,
		}
	}

	// Top 5 flight risk employees
	flightRisks, _ := bg.queries.ListAllFlightRiskScores(ctx, companyID)
	data.TopFlightRisk = topNFlightRisk(flightRisks, 5)

	// Top 5 burnout employees
	burnouts, _ := bg.queries.ListAllBurnoutScores(ctx, companyID)
	data.TopBurnout = topNBurnout(burnouts, 5)

	// Lowest health departments
	teamHealths, _ := bg.queries.ListAllTeamHealthScores(ctx, companyID)
	n := 5
	if len(teamHealths) < n {
		n = len(teamHealths)
	}
	data.LowestHealth = make([]map[string]any, n)
	for i := 0; i < n; i++ {
		data.LowestHealth[i] = map[string]any{
			"department":   teamHealths[i].DepartmentName,
			"health_score": teamHealths[i].HealthScore,
		}
	}

	return data, nil
}

func buildBriefingPrompt(data *briefingData) string {
	dataJSON, _ := json.MarshalIndent(data, "", "  ")
	return fmt.Sprintf(`Generate a weekly executive HR briefing for the week of %s based on the following organizational data:

%s

Structure the briefing with these sections:
1. **Executive Summary** — 2-3 sentence overview
2. **Key Metrics** — bullet points of current scores with week-over-week changes
3. **Risk Highlights** — top concerns requiring attention
4. **Recommendations** — 3-5 actionable items
5. **Positive Trends** — areas of improvement or stability`, data.WeekDate, string(dataJSON))
}

// mondayOfWeek is duplicated here for the orgintel package.
func mondayOfWeek(t time.Time) time.Time {
	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	monday := t.AddDate(0, 0, -int(weekday-time.Monday))
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
}
