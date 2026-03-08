package ai

import (
	"context"
	"fmt"

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
	existing, err := r.queries.GetEmployeeProfile(ctx, emp.ID)
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
