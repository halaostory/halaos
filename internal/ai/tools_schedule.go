package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/store"
)

func (r *ToolRegistry) toolListScheduleTemplates(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	templates, err := r.queries.ListScheduleTemplates(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("list schedule templates: %w", err)
	}

	type templateResult struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	}
	results := make([]templateResult, len(templates))
	for i, t := range templates {
		tr := templateResult{ID: t.ID, Name: t.Name}
		if t.Description != nil {
			tr.Description = *t.Description
		}
		results[i] = tr
	}
	return toJSON(map[string]any{"total": len(results), "templates": results})
}

func (r *ToolRegistry) toolGetEmployeeSchedule(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	employeeID := emp.ID
	if eid, ok := input["employee_id"].(float64); ok && eid > 0 {
		employeeID = int64(eid)
	}

	assignment, err := r.queries.GetEmployeeCurrentTemplate(ctx, store.GetEmployeeCurrentTemplateParams{
		CompanyID:     companyID,
		EmployeeID:    employeeID,
		EffectiveFrom: time.Now(),
	})
	if err != nil {
		return toJSON(map[string]any{"message": "No schedule template assigned.", "assigned": false})
	}

	// Get template days
	days, _ := r.queries.ListScheduleTemplateDays(ctx, assignment.TemplateID)

	dayNames := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	type dayResult struct {
		Day       string `json:"day"`
		ShiftName string `json:"shift_name,omitempty"`
		StartTime string `json:"start_time,omitempty"`
		EndTime   string `json:"end_time,omitempty"`
		IsRestDay bool   `json:"is_rest_day"`
	}
	dayResults := make([]dayResult, len(days))
	for i, d := range days {
		dr := dayResult{
			Day:       dayNames[d.DayOfWeek],
			IsRestDay: d.IsRestDay,
		}
		if d.ShiftName != nil {
			dr.ShiftName = *d.ShiftName
		}
		if d.StartTime.Valid {
			dr.StartTime = fmt.Sprintf("%02d:%02d", d.StartTime.Microseconds/3600000000, (d.StartTime.Microseconds%3600000000)/60000000)
		}
		if d.EndTime.Valid {
			dr.EndTime = fmt.Sprintf("%02d:%02d", d.EndTime.Microseconds/3600000000, (d.EndTime.Microseconds%3600000000)/60000000)
		}
		dayResults[i] = dr
	}

	return toJSON(map[string]any{
		"assigned":       true,
		"template_name":  assignment.TemplateName,
		"effective_from": assignment.EffectiveFrom.Format("2006-01-02"),
		"schedule":       dayResults,
	})
}

func (r *ToolRegistry) toolAssignSchedule(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	employeeID, ok := input["employee_id"].(float64)
	if !ok || employeeID <= 0 {
		return "", fmt.Errorf("employee_id is required")
	}

	templateID, ok := input["template_id"].(float64)
	if !ok || templateID <= 0 {
		return "", fmt.Errorf("template_id is required")
	}

	effectiveDateStr, _ := input["effective_date"].(string)
	if effectiveDateStr == "" {
		effectiveDateStr = time.Now().Format("2006-01-02")
	}
	effectiveDate, err := time.Parse("2006-01-02", effectiveDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid effective_date format")
	}

	assignment, err := r.queries.AssignScheduleTemplate(ctx, store.AssignScheduleTemplateParams{
		CompanyID:     companyID,
		EmployeeID:    int64(employeeID),
		TemplateID:    int64(templateID),
		EffectiveFrom: effectiveDate,
		EffectiveTo:   pgtype.Date{Valid: false},
	})
	if err != nil {
		return "", fmt.Errorf("assign schedule: %w", err)
	}

	return toJSON(map[string]any{
		"success":       true,
		"assignment_id": assignment.ID,
		"message":       "Schedule template assigned successfully.",
	})
}

// scheduleDefs returns tool definitions for schedule-related tools.
func scheduleDefs() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "list_schedule_templates",
			Description: "List available schedule templates (shift patterns) for the company.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "get_employee_schedule",
			Description: "Get the current schedule assignment for the user or a specific employee. Returns the weekly schedule with shift times and rest days.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_id": map[string]any{"type": "integer", "description": "Optional employee ID. Omit to check current user."},
				},
			}),
		},
		{
			Name:        "assign_schedule",
			Description: "Assign a schedule template to an employee. Admin/Manager only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_id":    map[string]any{"type": "integer", "description": "Employee ID."},
					"template_id":    map[string]any{"type": "integer", "description": "Schedule template ID (from list_schedule_templates)."},
					"effective_date": map[string]any{"type": "string", "description": "Effective date in YYYY-MM-DD. Default: today."},
				},
				"required": []string{"employee_id", "template_id"},
			}),
		},
	}
}

// registerScheduleTools registers schedule-related tool executors.
func (r *ToolRegistry) registerScheduleTools() {
	r.tools["list_schedule_templates"] = r.toolListScheduleTemplates
	r.tools["get_employee_schedule"] = r.toolGetEmployeeSchedule
	r.tools["assign_schedule"] = r.toolAssignSchedule
}
