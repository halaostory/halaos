package ai

import (
	"context"
	"fmt"

	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/store"
)

func pulseDefs() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "query_pulse_results",
			Description: "Get pulse survey results and sentiment trends. Returns survey list with latest round stats (response rate, average ratings). Use to understand employee sentiment.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"survey_id": map[string]any{
						"type":        "number",
						"description": "Optional: specific survey ID. If omitted, returns summary of all active surveys.",
					},
				},
			},
		},
	}
}

func (r *ToolRegistry) registerPulseTools() {
	r.tools["query_pulse_results"] = r.toolQueryPulseResults
}

func (r *ToolRegistry) toolQueryPulseResults(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if sid, ok := input["survey_id"].(float64); ok && sid > 0 {
		return r.pulseResultsForSurvey(ctx, companyID, int64(sid))
	}
	return r.pulseResultsSummary(ctx, companyID)
}

func (r *ToolRegistry) pulseResultsSummary(ctx context.Context, companyID int64) (string, error) {
	surveys, err := r.queries.ListPulseSurveys(ctx, store.ListPulseSurveysParams{
		CompanyID: companyID,
		Limit:     20,
		Offset:    0,
	})
	if err != nil {
		return "", fmt.Errorf("list pulse surveys: %w", err)
	}
	if len(surveys) == 0 {
		return `{"surveys":[],"message":"No pulse surveys configured"}`, nil
	}

	type SurveySummary struct {
		ID              int64    `json:"id"`
		Title           string   `json:"title"`
		Frequency       string   `json:"frequency"`
		IsActive        bool     `json:"is_active"`
		LatestRoundDate string   `json:"latest_round_date,omitempty"`
		ResponseRate    string   `json:"response_rate,omitempty"`
		AvgRating       *float64 `json:"avg_rating,omitempty"`
	}

	var summaries []SurveySummary
	for _, s := range surveys {
		ss := SurveySummary{
			ID:       s.ID,
			Title:    s.Title,
			Frequency: s.Frequency,
			IsActive: s.IsActive,
		}

		rounds, err := r.queries.ListPulseRounds(ctx, store.ListPulseRoundsParams{
			SurveyID: s.ID,
			Limit:    1,
			Offset:   0,
		})
		if err == nil && len(rounds) > 0 {
			latest := rounds[0]
			ss.LatestRoundDate = latest.RoundDate.Format("2006-01-02")
			if latest.TotalSent > 0 {
				rate := float64(latest.TotalResponded) / float64(latest.TotalSent) * 100
				ss.ResponseRate = fmt.Sprintf("%.0f%%", rate)
			}

			// Get overall average rating for latest round
			row := r.pool.QueryRow(ctx,
				`SELECT AVG(rating)::float8 FROM pulse_responses WHERE round_id = $1 AND rating IS NOT NULL`,
				latest.ID)
			var avg *float64
			if row.Scan(&avg) == nil {
				ss.AvgRating = avg
			}
		}

		summaries = append(summaries, ss)
	}

	return toJSON(summaries)
}

func (r *ToolRegistry) pulseResultsForSurvey(ctx context.Context, companyID int64, surveyID int64) (string, error) {
	survey, err := r.queries.GetPulseSurvey(ctx, store.GetPulseSurveyParams{
		ID:        surveyID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("get pulse survey: %w", err)
	}

	questions, _ := r.queries.ListPulseQuestions(ctx, surveyID)

	rounds, err := r.queries.ListPulseRounds(ctx, store.ListPulseRoundsParams{
		SurveyID: surveyID,
		Limit:    5,
		Offset:   0,
	})
	if err != nil {
		return "", fmt.Errorf("list rounds: %w", err)
	}

	type QuestionStat struct {
		Question     string   `json:"question"`
		QuestionType string   `json:"question_type"`
		AvgRating    *float64 `json:"avg_rating,omitempty"`
		Responses    int      `json:"responses"`
	}

	type RoundStat struct {
		RoundDate      string         `json:"round_date"`
		Status         string         `json:"status"`
		ResponseRate   string         `json:"response_rate"`
		QuestionStats  []QuestionStat `json:"question_stats"`
	}

	var roundStats []RoundStat
	for _, round := range rounds {
		rs := RoundStat{
			RoundDate: round.RoundDate.Format("2006-01-02"),
			Status:    round.Status,
		}
		if round.TotalSent > 0 {
			rate := float64(round.TotalResponded) / float64(round.TotalSent) * 100
			rs.ResponseRate = fmt.Sprintf("%.0f%% (%d/%d)", rate, round.TotalResponded, round.TotalSent)
		}

		for _, q := range questions {
			qs := QuestionStat{
				Question:     q.Question,
				QuestionType: q.QuestionType,
			}
			if q.QuestionType == "rating" {
				row := r.pool.QueryRow(ctx,
					`SELECT AVG(rating)::float8, COUNT(*) FROM pulse_responses WHERE round_id = $1 AND question_id = $2 AND rating IS NOT NULL`,
					round.ID, q.ID)
				var avg *float64
				var count int
				if row.Scan(&avg, &count) == nil {
					qs.AvgRating = avg
					qs.Responses = count
				}
			} else {
				row := r.pool.QueryRow(ctx,
					`SELECT COUNT(*) FROM pulse_responses WHERE round_id = $1 AND question_id = $2 AND answer_text IS NOT NULL`,
					round.ID, q.ID)
				var count int
				if row.Scan(&count) == nil {
					qs.Responses = count
				}
			}
			rs.QuestionStats = append(rs.QuestionStats, qs)
		}

		roundStats = append(roundStats, rs)
	}

	result := map[string]any{
		"survey": map[string]any{
			"id":           survey.ID,
			"title":        survey.Title,
			"frequency":    survey.Frequency,
			"is_anonymous": survey.IsAnonymous,
			"is_active":    survey.IsActive,
		},
		"rounds": roundStats,
	}

	return toJSON(result)
}
