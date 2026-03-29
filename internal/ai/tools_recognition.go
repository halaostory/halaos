package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/store"
)

func recognitionDefs() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "send_kudos",
			Description: "Send a recognition/kudos to a colleague. Requires the recipient's employee ID, a message, and optionally a category (kudos, teamwork, innovation, leadership, above_and_beyond, customer_focus).",
			Parameters: map[string]any{
				"type":     "object",
				"required": []string{"to_employee_id", "message"},
				"properties": map[string]any{
					"to_employee_id": map[string]any{"type": "number", "description": "Employee ID of the recipient"},
					"message":        map[string]any{"type": "string", "description": "Recognition message"},
					"category":       map[string]any{"type": "string", "description": "Category: kudos, teamwork, innovation, leadership, above_and_beyond, customer_focus"},
				},
			},
		},
		{
			Name:        "query_kudos",
			Description: "Query recognition/kudos data. Returns recent recognitions, top recognized employees, and stats.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_id": map[string]any{"type": "number", "description": "Optional: filter by employee ID"},
				},
			},
		},
	}
}

func (r *ToolRegistry) registerRecognitionTools() {
	r.tools["send_kudos"] = r.toolSendKudos
	r.tools["query_kudos"] = r.toolQueryKudos
}

func (r *ToolRegistry) toolSendKudos(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	toID, ok := input["to_employee_id"].(float64)
	if !ok || toID <= 0 {
		return "", fmt.Errorf("to_employee_id is required")
	}
	msg, ok := input["message"].(string)
	if !ok || msg == "" {
		return "", fmt.Errorf("message is required")
	}

	category := "kudos"
	if cat, ok := input["category"].(string); ok && cat != "" {
		category = cat
	}

	fromEmp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("sender employee not found: %w", err)
	}

	rec, err := r.queries.CreateRecognition(ctx, store.CreateRecognitionParams{
		CompanyID:      companyID,
		FromEmployeeID: fromEmp.ID,
		ToEmployeeID:   int64(toID),
		Category:       category,
		Message:        msg,
		IsPublic:       true,
		Points:         1,
	})
	if err != nil {
		return "", fmt.Errorf("create recognition: %w", err)
	}

	return toJSON(map[string]any{
		"id":       rec.ID,
		"message":  "Recognition sent successfully",
		"category": rec.Category,
	})
}

func (r *ToolRegistry) toolQueryKudos(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	since := time.Now().AddDate(0, -1, 0)

	stats, err := r.queries.GetRecognitionStats(ctx, store.GetRecognitionStatsParams{
		CompanyID: companyID,
		CreatedAt: since,
	})
	if err != nil {
		return "", fmt.Errorf("get recognition stats: %w", err)
	}

	topRecognized, _ := r.queries.GetTopRecognized(ctx, store.GetTopRecognizedParams{
		CompanyID: companyID,
		CreatedAt: since,
		Limit:     5,
	})

	categories, _ := r.queries.GetCategoryBreakdown(ctx, store.GetCategoryBreakdownParams{
		CompanyID: companyID,
		CreatedAt: since,
	})

	// If specific employee, get their count
	var empCount int64
	if eid, ok := input["employee_id"].(float64); ok && eid > 0 {
		empCount, _ = r.queries.CountRecognitionsReceived(ctx, store.CountRecognitionsReceivedParams{
			ToEmployeeID: int64(eid),
			CompanyID:    companyID,
		})
	}

	result := map[string]any{
		"period":              "last 30 days",
		"total_recognitions":  stats.TotalRecognitions,
		"unique_givers":       stats.UniqueGivers,
		"unique_receivers":    stats.UniqueReceivers,
		"top_recognized":      topRecognized,
		"category_breakdown":  categories,
	}
	if empCount > 0 {
		result["employee_total_received"] = empCount
	}

	return toJSON(result)
}
