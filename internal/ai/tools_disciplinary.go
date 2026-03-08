package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/store"
)

func (r *ToolRegistry) toolQueryEmployeeDisciplinary(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	employeeID, ok := input["employee_id"].(float64)
	if !ok || employeeID <= 0 {
		return "", fmt.Errorf("employee_id is required")
	}

	summary, err := r.queries.GetEmployeeDisciplinarySummary(ctx, store.GetEmployeeDisciplinarySummaryParams{
		CompanyID:  companyID,
		EmployeeID: int64(employeeID),
	})
	if err != nil {
		return "", fmt.Errorf("get disciplinary summary: %w", err)
	}

	actions, err := r.queries.GetEmployeeActionCounts(ctx, store.GetEmployeeActionCountsParams{
		CompanyID:  companyID,
		EmployeeID: int64(employeeID),
	})
	if err != nil {
		return "", fmt.Errorf("get action counts: %w", err)
	}

	return toJSON(map[string]any{
		"incidents": map[string]any{
			"total": summary.TotalIncidents,
			"grave": summary.GraveCount,
			"open":  summary.OpenCount,
		},
		"actions": map[string]any{
			"total":            actions.TotalActions,
			"verbal_warnings":  actions.VerbalWarnings,
			"written_warnings": actions.WrittenWarnings,
			"final_warnings":   actions.FinalWarnings,
			"suspensions":      actions.Suspensions,
		},
	})
}

func (r *ToolRegistry) toolCreateDisciplinaryIncident(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	employeeID, ok := input["employee_id"].(float64)
	if !ok || employeeID <= 0 {
		return "", fmt.Errorf("employee_id is required")
	}

	category, _ := input["category"].(string)
	if category == "" {
		return "", fmt.Errorf("category is required (tardiness, absence, misconduct, insubordination, policy_violation, performance, safety)")
	}

	severity := "minor"
	if s, ok := input["severity"].(string); ok && s != "" {
		severity = s
	}

	description, _ := input["description"].(string)
	if description == "" {
		return "", fmt.Errorf("description is required")
	}

	incidentDateStr, _ := input["incident_date"].(string)
	if incidentDateStr == "" {
		incidentDateStr = time.Now().Format("2006-01-02")
	}
	incidentDate, err := time.Parse("2006-01-02", incidentDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid incident_date format, use YYYY-MM-DD")
	}

	var witnesses, evidenceNotes *string
	if w, ok := input["witnesses"].(string); ok && w != "" {
		witnesses = &w
	}
	if e, ok := input["evidence_notes"].(string); ok && e != "" {
		evidenceNotes = &e
	}

	incident, err := r.queries.CreateDisciplinaryIncident(ctx, store.CreateDisciplinaryIncidentParams{
		CompanyID:     companyID,
		EmployeeID:    int64(employeeID),
		ReportedBy:    &userID,
		IncidentDate:  incidentDate,
		Category:      category,
		Severity:      severity,
		Description:   description,
		Witnesses:     witnesses,
		EvidenceNotes: evidenceNotes,
	})
	if err != nil {
		return "", fmt.Errorf("create incident: %w", err)
	}

	return toJSON(map[string]any{
		"success":     true,
		"incident_id": incident.ID,
		"message":     "Disciplinary incident recorded successfully.",
	})
}

func (r *ToolRegistry) toolCreateDisciplinaryAction(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	employeeID, ok := input["employee_id"].(float64)
	if !ok || employeeID <= 0 {
		return "", fmt.Errorf("employee_id is required")
	}

	actionType, _ := input["action_type"].(string)
	if actionType == "" {
		return "", fmt.Errorf("action_type is required (verbal_warning, written_warning, final_warning, suspension, termination)")
	}

	description, _ := input["description"].(string)
	if description == "" {
		return "", fmt.Errorf("description is required")
	}

	var incidentID *int64
	if iid, ok := input["incident_id"].(float64); ok && iid > 0 {
		id := int64(iid)
		incidentID = &id
	}

	var suspensionDays *int32
	if sd, ok := input["suspension_days"].(float64); ok && sd > 0 {
		d := int32(sd)
		suspensionDays = &d
	}

	var notes *string
	if n, ok := input["notes"].(string); ok && n != "" {
		notes = &n
	}

	var effectiveDate, endDate pgtype.Date
	if ed, ok := input["effective_date"].(string); ok && ed != "" {
		t, err := time.Parse("2006-01-02", ed)
		if err == nil {
			effectiveDate = pgtype.Date{Time: t, Valid: true}
		}
	}
	if ed, ok := input["end_date"].(string); ok && ed != "" {
		t, err := time.Parse("2006-01-02", ed)
		if err == nil {
			endDate = pgtype.Date{Time: t, Valid: true}
		}
	}

	action, err := r.queries.CreateDisciplinaryAction(ctx, store.CreateDisciplinaryActionParams{
		CompanyID:      companyID,
		EmployeeID:     int64(employeeID),
		IncidentID:     incidentID,
		ActionType:     actionType,
		ActionDate:     time.Now(),
		IssuedBy:       userID,
		Description:    description,
		SuspensionDays: suspensionDays,
		EffectiveDate:  effectiveDate,
		EndDate:        endDate,
		Notes:          notes,
	})
	if err != nil {
		return "", fmt.Errorf("create action: %w", err)
	}

	return toJSON(map[string]any{
		"success":   true,
		"action_id": action.ID,
		"type":      action.ActionType,
		"message":   fmt.Sprintf("Disciplinary action '%s' created successfully.", actionType),
	})
}

func (r *ToolRegistry) toolListRecentIncidents(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	incidents, err := r.queries.ListDisciplinaryIncidents(ctx, store.ListDisciplinaryIncidentsParams{
		CompanyID:  companyID,
		Status:     "",
		EmployeeID: int64(0),
		Lim:        20,
		Off:        0,
	})
	if err != nil {
		return "", fmt.Errorf("list incidents: %w", err)
	}

	type incidentResult struct {
		ID           int64  `json:"id"`
		EmployeeName string `json:"employee_name"`
		EmployeeNo   string `json:"employee_no"`
		Category     string `json:"category"`
		Severity     string `json:"severity"`
		Status       string `json:"status"`
		IncidentDate string `json:"incident_date"`
		Description  string `json:"description"`
	}
	results := make([]incidentResult, len(incidents))
	for i, inc := range incidents {
		results[i] = incidentResult{
			ID:           inc.ID,
			EmployeeName: inc.FirstName + " " + inc.LastName,
			EmployeeNo:   inc.EmployeeNo,
			Category:     inc.Category,
			Severity:     inc.Severity,
			Status:       inc.Status,
			IncidentDate: inc.IncidentDate.Format("2006-01-02"),
			Description:  inc.Description,
		}
	}

	return toJSON(map[string]any{"total": len(results), "incidents": results})
}

func (r *ToolRegistry) toolQueryGrievanceSummary(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	summary, err := r.queries.GetGrievanceSummary(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("get grievance summary: %w", err)
	}

	return toJSON(map[string]any{
		"total":        summary.Total,
		"open":         summary.OpenCount,
		"under_review": summary.UnderReview,
		"in_mediation": summary.InMediation,
		"resolved":     summary.Resolved,
		"critical":     summary.CriticalOpen,
	})
}

func (r *ToolRegistry) toolGetGrievanceDetail(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin", "manager"); err != nil {
		return "", err
	}

	grievanceID, ok := input["grievance_id"].(float64)
	if !ok || grievanceID <= 0 {
		return "", fmt.Errorf("grievance_id is required")
	}

	grievance, err := r.queries.GetGrievance(ctx, store.GetGrievanceParams{
		ID:        int64(grievanceID),
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("grievance not found: %w", err)
	}

	comments, _ := r.queries.ListGrievanceComments(ctx, int64(grievanceID))

	type commentResult struct {
		UserName   string `json:"user_name"`
		Comment    string `json:"comment"`
		IsInternal bool   `json:"is_internal"`
		CreatedAt  string `json:"created_at"`
	}
	commentResults := make([]commentResult, len(comments))
	for i, c := range comments {
		commentResults[i] = commentResult{
			UserName:   fmt.Sprintf("%v", c.UserName),
			Comment:    c.Comment,
			IsInternal: c.IsInternal,
			CreatedAt:  c.CreatedAt.Format("2006-01-02 15:04"),
		}
	}

	return toJSON(map[string]any{
		"id":            grievance.ID,
		"case_number":   grievance.CaseNumber,
		"employee_name": grievance.FirstName + " " + grievance.LastName,
		"category":      grievance.Category,
		"subject":       grievance.Subject,
		"description":   grievance.Description,
		"severity":      grievance.Severity,
		"status":        grievance.Status,
		"assigned_to":   grievance.AssignedToName,
		"comments":      commentResults,
	})
}

func (r *ToolRegistry) toolResolveGrievance(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}

	grievanceID, ok := input["grievance_id"].(float64)
	if !ok || grievanceID <= 0 {
		return "", fmt.Errorf("grievance_id is required")
	}

	resolution, _ := input["resolution"].(string)
	if resolution == "" {
		return "", fmt.Errorf("resolution is required")
	}

	gc, err := r.queries.ResolveGrievance(ctx, store.ResolveGrievanceParams{
		ID:         int64(grievanceID),
		CompanyID:  companyID,
		Resolution: &resolution,
	})
	if err != nil {
		return "", fmt.Errorf("resolve grievance: %w", err)
	}

	return toJSON(map[string]any{
		"success":      true,
		"grievance_id": gc.ID,
		"status":       gc.Status,
		"message":      "Grievance resolved successfully.",
	})
}
