package ai

import (
	"context"
	"fmt"

	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/store"
)

func hrrequestDefs() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "create_hr_request",
			Description: "Submit an HR service request on behalf of the employee. Types: coe (Certificate of Employment), salary_cert (Salary Certificate), id_replacement, equipment, schedule_change, general.",
			Parameters: map[string]any{
				"type":     "object",
				"required": []string{"request_type", "subject"},
				"properties": map[string]any{
					"request_type": map[string]any{"type": "string", "description": "Type: coe, salary_cert, id_replacement, equipment, schedule_change, general"},
					"subject":      map[string]any{"type": "string", "description": "Brief subject of the request"},
					"description":  map[string]any{"type": "string", "description": "Detailed description"},
					"priority":     map[string]any{"type": "string", "description": "Priority: low, normal, high, urgent. Default: normal"},
				},
			},
		},
		{
			Name:        "query_hr_requests",
			Description: "Query the current employee's HR service requests and their status.",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

func (r *ToolRegistry) registerHRRequestTools() {
	r.tools["create_hr_request"] = r.toolCreateHRRequest
	r.tools["query_hr_requests"] = r.toolQueryHRRequests
}

func (r *ToolRegistry) toolCreateHRRequest(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	reqType, _ := input["request_type"].(string)
	subject, _ := input["subject"].(string)
	if reqType == "" || subject == "" {
		return "", fmt.Errorf("request_type and subject are required")
	}

	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	priority := "normal"
	if p, ok := input["priority"].(string); ok && p != "" {
		priority = p
	}

	var desc *string
	if d, ok := input["description"].(string); ok && d != "" {
		desc = &d
	}

	hr, err := r.queries.CreateHRRequest(ctx, store.CreateHRRequestParams{
		CompanyID:   companyID,
		EmployeeID:  emp.ID,
		RequestType: reqType,
		Subject:     subject,
		Description: desc,
		Priority:    priority,
	})
	if err != nil {
		return "", fmt.Errorf("create hr request: %w", err)
	}

	return toJSON(map[string]any{
		"id":           hr.ID,
		"request_type": hr.RequestType,
		"subject":      hr.Subject,
		"status":       hr.Status,
		"message":      "HR request created successfully",
	})
}

func (r *ToolRegistry) toolQueryHRRequests(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return `{"requests":[],"message":"Employee not found"}`, nil
	}

	reqs, err := r.queries.ListHRRequestsByEmployee(ctx, store.ListHRRequestsByEmployeeParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
		Limit:      10,
		Offset:     0,
	})
	if err != nil {
		return "", fmt.Errorf("list hr requests: %w", err)
	}

	return toJSON(reqs)
}
