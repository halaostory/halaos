package ai

import (
	"context"
	"fmt"

	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/store"
)

func (r *ToolRegistry) toolListEmployees(ctx context.Context, companyID, _ int64, input map[string]any) (string, error) {
	limit := int32(20)
	if l, ok := input["limit"].(float64); ok && l > 0 {
		limit = int32(l)
	}
	if limit > 50 {
		limit = 50
	}

	employees, err := r.queries.ListEmployees(ctx, store.ListEmployeesParams{
		CompanyID: companyID,
		Limit:     limit,
		Offset:    0,
	})
	if err != nil {
		return "", fmt.Errorf("list employees: %w", err)
	}
	return toJSON(employees)
}

func (r *ToolRegistry) toolUpdateEmployeeProfile(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	// Get existing profile (may not exist yet)
	existing, err := r.queries.GetEmployeeProfile(ctx, store.GetEmployeeProfileParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err != nil {
		// No existing profile — start with empty
		existing = store.EmployeeProfile{EmployeeID: emp.ID}
	}

	// Merge: input overrides existing
	merged := store.UpsertEmployeeProfileParams{
		EmployeeID:        emp.ID,
		AddressLine1:      coalesceStrPtr(input, "address_line1", existing.AddressLine1),
		AddressLine2:      coalesceStrPtr(input, "address_line2", existing.AddressLine2),
		City:              coalesceStrPtr(input, "city", existing.City),
		Province:          coalesceStrPtr(input, "province", existing.Province),
		ZipCode:           coalesceStrPtr(input, "zip_code", existing.ZipCode),
		EmergencyName:     coalesceStrPtr(input, "emergency_name", existing.EmergencyName),
		EmergencyPhone:    coalesceStrPtr(input, "emergency_phone", existing.EmergencyPhone),
		EmergencyRelation: coalesceStrPtr(input, "emergency_relation", existing.EmergencyRelation),
		BankName:          coalesceStrPtr(input, "bank_name", existing.BankName),
		BankAccountNo:     coalesceStrPtr(input, "bank_account_no", existing.BankAccountNo),
		BankAccountName:   coalesceStrPtr(input, "bank_account_name", existing.BankAccountName),
		Tin:               coalesceStrPtr(input, "tin", existing.Tin),
		SssNo:             coalesceStrPtr(input, "sss_no", existing.SssNo),
		PhilhealthNo:      coalesceStrPtr(input, "philhealth_no", existing.PhilhealthNo),
		PagibigNo:         coalesceStrPtr(input, "pagibig_no", existing.PagibigNo),
	}

	profile, err := r.queries.UpsertEmployeeProfile(ctx, merged)
	if err != nil {
		return "", fmt.Errorf("update employee profile: %w", err)
	}

	// Build list of updated fields
	updated := make([]string, 0)
	for _, key := range []string{"address_line1", "address_line2", "city", "province", "zip_code",
		"emergency_name", "emergency_phone", "emergency_relation",
		"bank_name", "bank_account_no", "bank_account_name",
		"tin", "sss_no", "philhealth_no", "pagibig_no"} {
		if v, ok := input[key].(string); ok && v != "" {
			updated = append(updated, key)
		}
	}

	return toJSON(map[string]any{
		"success":        true,
		"employee_id":    profile.EmployeeID,
		"updated_fields": updated,
		"message":        "Profile updated successfully.",
	})
}

// employeeDefs returns tool definitions for employee-related tools.
func employeeDefs() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "list_employees",
			Description: "List employees in the company. Admin/Manager only. Returns employee number, name, department, status.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status": map[string]any{"type": "string", "description": "Filter by status: active, probationary, suspended, separated."},
					"limit":  map[string]any{"type": "integer", "description": "Max results. Default 20."},
				},
			}),
		},
		{
			Name:        "update_employee_profile",
			Description: "Update the current user's personal profile information. Only provided fields are updated; omitted fields remain unchanged. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"address_line1":      map[string]any{"type": "string", "description": "Address line 1."},
					"address_line2":      map[string]any{"type": "string", "description": "Address line 2."},
					"city":               map[string]any{"type": "string", "description": "City."},
					"province":           map[string]any{"type": "string", "description": "Province."},
					"zip_code":           map[string]any{"type": "string", "description": "ZIP code."},
					"emergency_name":     map[string]any{"type": "string", "description": "Emergency contact name."},
					"emergency_phone":    map[string]any{"type": "string", "description": "Emergency contact phone."},
					"emergency_relation": map[string]any{"type": "string", "description": "Relationship to emergency contact."},
					"bank_name":          map[string]any{"type": "string", "description": "Bank name for payroll."},
					"bank_account_no":    map[string]any{"type": "string", "description": "Bank account number."},
					"bank_account_name":  map[string]any{"type": "string", "description": "Bank account holder name."},
					"tin":                map[string]any{"type": "string", "description": "Tax Identification Number."},
					"sss_no":             map[string]any{"type": "string", "description": "SSS number."},
					"philhealth_no":      map[string]any{"type": "string", "description": "PhilHealth number."},
					"pagibig_no":         map[string]any{"type": "string", "description": "Pag-IBIG number."},
				},
			}),
		},
	}
}

// registerEmployeeTools registers employee-related tool executors.
func (r *ToolRegistry) registerEmployeeTools() {
	r.tools["list_employees"] = r.toolListEmployees
	r.tools["update_employee_profile"] = r.toolUpdateEmployeeProfile
}
