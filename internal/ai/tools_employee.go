package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/store"
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

// toolSearchDepartments finds departments by name substring.
func (r *ToolRegistry) toolSearchDepartments(ctx context.Context, companyID, _ int64, input map[string]any) (string, error) {
	query, _ := input["query"].(string)
	depts, err := r.queries.ListDepartments(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("list departments: %w", err)
	}
	if query == "" {
		return toJSON(depts)
	}
	q := strings.ToLower(query)
	matched := make([]map[string]any, 0)
	for _, d := range depts {
		if strings.Contains(strings.ToLower(d.Name), q) || strings.Contains(strings.ToLower(d.Code), q) {
			matched = append(matched, map[string]any{
				"id": d.ID, "code": d.Code, "name": d.Name,
			})
		}
	}
	return toJSON(matched)
}

// toolSearchPositions finds positions by title substring.
func (r *ToolRegistry) toolSearchPositions(ctx context.Context, companyID, _ int64, input map[string]any) (string, error) {
	query, _ := input["query"].(string)
	positions, err := r.queries.ListPositions(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("list positions: %w", err)
	}
	if query == "" {
		return toJSON(positions)
	}
	q := strings.ToLower(query)
	matched := make([]map[string]any, 0)
	for _, p := range positions {
		if strings.Contains(strings.ToLower(p.Title), q) || strings.Contains(strings.ToLower(p.Code), q) {
			matched = append(matched, map[string]any{
				"id": p.ID, "code": p.Code, "title": p.Title,
				"department_id": p.DepartmentID,
			})
		}
	}
	return toJSON(matched)
}

// toolCreateEmployee creates a new employee with optional department, position, and salary.
func (r *ToolRegistry) toolCreateEmployee(ctx context.Context, companyID, _ int64, input map[string]any) (string, error) {
	firstName, _ := input["first_name"].(string)
	lastName, _ := input["last_name"].(string)
	if firstName == "" || lastName == "" {
		return "", fmt.Errorf("first_name and last_name are required")
	}

	hireDateStr, _ := input["hire_date"].(string)
	if hireDateStr == "" {
		hireDateStr = time.Now().Format("2006-01-02")
	}
	hireDate, err := time.Parse("2006-01-02", hireDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid hire_date format, expected YYYY-MM-DD: %w", err)
	}

	// Generate employee number
	empNo, _ := input["employee_no"].(string)
	if empNo == "" {
		empNo = fmt.Sprintf("EMP-%d", time.Now().UnixMilli()%100000)
	}

	employmentType := "regular"
	if et, ok := input["employment_type"].(string); ok && et != "" {
		employmentType = et
	}

	params := store.CreateEmployeeParams{
		CompanyID:      companyID,
		EmployeeNo:     empNo,
		FirstName:      firstName,
		LastName:        lastName,
		HireDate:       hireDate,
		EmploymentType: employmentType,
	}

	if v, ok := input["middle_name"].(string); ok && v != "" {
		params.MiddleName = &v
	}
	if v, ok := input["email"].(string); ok && v != "" {
		params.Email = &v
	}
	if v, ok := input["phone"].(string); ok && v != "" {
		params.Phone = &v
	}
	if v, ok := input["gender"].(string); ok && v != "" {
		params.Gender = &v
	}

	// Resolve department
	if deptID, ok := input["department_id"].(float64); ok && deptID > 0 {
		did := int64(deptID)
		params.DepartmentID = &did
	} else if deptName, ok := input["department_name"].(string); ok && deptName != "" {
		depts, err := r.queries.ListDepartments(ctx, companyID)
		if err == nil {
			q := strings.ToLower(deptName)
			for _, d := range depts {
				if strings.EqualFold(d.Name, deptName) || strings.Contains(strings.ToLower(d.Name), q) {
					params.DepartmentID = &d.ID
					break
				}
			}
		}
	}

	// Resolve position
	if posID, ok := input["position_id"].(float64); ok && posID > 0 {
		pid := int64(posID)
		params.PositionID = &pid
	} else if posTitle, ok := input["position_title"].(string); ok && posTitle != "" {
		positions, err := r.queries.ListPositions(ctx, companyID)
		if err == nil {
			q := strings.ToLower(posTitle)
			for _, p := range positions {
				if strings.EqualFold(p.Title, posTitle) || strings.Contains(strings.ToLower(p.Title), q) {
					params.PositionID = &p.ID
					break
				}
			}
		}
	}

	emp, err := r.queries.CreateEmployee(ctx, params)
	if err != nil {
		return "", fmt.Errorf("create employee: %w", err)
	}

	result := map[string]any{
		"success":     true,
		"employee_id": emp.ID,
		"employee_no": emp.EmployeeNo,
		"name":        fmt.Sprintf("%s %s", emp.FirstName, emp.LastName),
		"hire_date":   emp.HireDate.Format("2006-01-02"),
		"message":     fmt.Sprintf("Employee %s %s (ID: %d) created successfully.", emp.FirstName, emp.LastName, emp.ID),
	}

	// Emit employee.hired event
	idempKey := fmt.Sprintf("employee.hired.%d", emp.ID)
	payload, _ := json.Marshal(map[string]any{
		"employee_id":     emp.ID,
		"first_name":      emp.FirstName,
		"last_name":       emp.LastName,
		"employment_type": emp.EmploymentType,
		"department_id":   emp.DepartmentID,
	})
	_, _ = r.queries.InsertHREvent(ctx, store.InsertHREventParams{
		CompanyID:      companyID,
		AggregateType:  "employee",
		AggregateID:    emp.ID,
		EventType:      "employee.hired",
		EventVersion:   1,
		Payload:        payload,
		IdempotencyKey: &idempKey,
	})

	// Auto-assign salary if provided
	if basicSalary, ok := input["basic_salary"].(float64); ok && basicSalary > 0 {
		var salaryNum pgtype.Numeric
		_ = salaryNum.Scan(fmt.Sprintf("%.2f", basicSalary))
		remark := "Auto-assigned via AI onboarding"
		_, salaryErr := r.queries.CreateEmployeeSalary(ctx, store.CreateEmployeeSalaryParams{
			CompanyID:     companyID,
			EmployeeID:    emp.ID,
			BasicSalary:   salaryNum,
			EffectiveFrom: hireDate,
			Remarks:       &remark,
		})
		if salaryErr != nil {
			result["salary_warning"] = fmt.Sprintf("Employee created but salary assignment failed: %s", salaryErr.Error())
		} else {
			result["basic_salary"] = basicSalary
			result["message"] = fmt.Sprintf("Employee %s %s (ID: %d) created with %.2f basic salary.", emp.FirstName, emp.LastName, emp.ID, basicSalary)
		}
	}

	// Auto-initiate onboarding if requested
	if initOnboarding, ok := input["initiate_onboarding"].(bool); ok && initOnboarding {
		templates, tplErr := r.queries.ListOnboardingTemplates(ctx, companyID)
		if tplErr != nil {
			result["onboarding_warning"] = fmt.Sprintf("Failed to list onboarding templates: %s", tplErr.Error())
		} else {
			created := 0
			for _, tpl := range templates {
				if !tpl.IsActive || tpl.WorkflowType != "onboarding" {
					continue
				}
				dueDate := pgtype.Date{}
				if tpl.DueDays > 0 {
					d := hireDate.AddDate(0, 0, int(tpl.DueDays))
					dueDate = pgtype.Date{Time: d, Valid: true}
				}
				tplID := tpl.ID
				_, err := r.queries.CreateOnboardingTask(ctx, store.CreateOnboardingTaskParams{
					CompanyID:    companyID,
					EmployeeID:   emp.ID,
					TemplateID:   &tplID,
					WorkflowType: tpl.WorkflowType,
					Title:        tpl.Title,
					Description:  tpl.Description,
					IsRequired:   tpl.IsRequired,
					AssigneeRole: tpl.AssigneeRole,
					DueDate:      dueDate,
					SortOrder:    tpl.SortOrder,
				})
				if err != nil {
					continue
				}
				created++
			}
			result["onboarding_tasks"] = created
		}
	}

	return toJSON(result)
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
		{
			Name:        "search_departments",
			Description: "Search company departments by name. Returns matching department IDs, codes, and names. Use this to resolve a department name to its ID before creating an employee.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "Search text to match department name or code. Leave empty to list all."},
				},
			}),
		},
		{
			Name:        "search_positions",
			Description: "Search company job positions by title. Returns matching position IDs, codes, and titles. Use this to resolve a position title to its ID before creating an employee.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "Search text to match position title or code. Leave empty to list all."},
				},
			}),
		},
		{
			Name:        "create_employee",
			Description: "Create a new employee in the company. Admin only. Optionally assigns salary and initiates onboarding workflow. Use search_departments and search_positions first to resolve names to IDs. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"first_name":           map[string]any{"type": "string", "description": "First name (required)."},
					"last_name":            map[string]any{"type": "string", "description": "Last name (required)."},
					"middle_name":          map[string]any{"type": "string", "description": "Middle name."},
					"email":                map[string]any{"type": "string", "description": "Email address."},
					"phone":                map[string]any{"type": "string", "description": "Phone number."},
					"gender":               map[string]any{"type": "string", "description": "Gender: male, female, other."},
					"hire_date":            map[string]any{"type": "string", "description": "Start date in YYYY-MM-DD format. Defaults to today."},
					"employee_no":          map[string]any{"type": "string", "description": "Employee number. Auto-generated if not provided."},
					"employment_type":      map[string]any{"type": "string", "description": "Employment type: regular, probationary, contractual, part_time. Default: regular."},
					"department_id":        map[string]any{"type": "integer", "description": "Department ID (use search_departments to find)."},
					"department_name":      map[string]any{"type": "string", "description": "Department name (auto-resolved to ID if department_id not provided)."},
					"position_id":          map[string]any{"type": "integer", "description": "Position ID (use search_positions to find)."},
					"position_title":       map[string]any{"type": "string", "description": "Position title (auto-resolved to ID if position_id not provided)."},
					"basic_salary":         map[string]any{"type": "number", "description": "Monthly basic salary in PHP. If provided, salary is assigned automatically."},
					"initiate_onboarding":  map[string]any{"type": "boolean", "description": "If true, auto-creates onboarding tasks from company templates."},
				},
				"required": []string{"first_name", "last_name"},
			}),
		},
	}
}

// registerEmployeeTools registers employee-related tool executors.
func (r *ToolRegistry) registerEmployeeTools() {
	r.tools["list_employees"] = r.toolListEmployees
	r.tools["update_employee_profile"] = r.toolUpdateEmployeeProfile
	r.tools["search_departments"] = r.toolSearchDepartments
	r.tools["search_positions"] = r.toolSearchPositions
	r.tools["create_employee"] = r.toolCreateEmployee
}
