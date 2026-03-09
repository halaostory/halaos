package ai

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/tonypk/aigonhr/internal/ai/provider"
)

func (r *ToolRegistry) toolListJobPostings(ctx context.Context, companyID, _ int64, input map[string]any) (string, error) {
	status, _ := input["status"].(string)

	rows, err := r.pool.Query(ctx, `
		SELECT jp.id, jp.title, COALESCE(d.name, '') AS department,
		       jp.employment_type, jp.location, jp.status, jp.created_at,
		       (SELECT COUNT(*) FROM applicants a WHERE a.job_posting_id = jp.id) AS applicant_count
		FROM job_postings jp
		LEFT JOIN departments d ON d.id = jp.department_id
		WHERE jp.company_id = $1
		  AND ($2 = '' OR jp.status = $2)
		ORDER BY jp.created_at DESC
		LIMIT 50
	`, companyID, status)
	if err != nil {
		return "", fmt.Errorf("list job postings: %w", err)
	}
	defer rows.Close()

	type jobResult struct {
		ID             int64  `json:"id"`
		Title          string `json:"title"`
		Department     string `json:"department"`
		EmploymentType string `json:"employment_type"`
		Location       string `json:"location"`
		Status         string `json:"status"`
		CreatedAt      string `json:"created_at"`
		ApplicantCount int64  `json:"applicant_count"`
	}
	var results []jobResult
	for rows.Next() {
		var j jobResult
		var createdAt time.Time
		if err := rows.Scan(&j.ID, &j.Title, &j.Department,
			&j.EmploymentType, &j.Location, &j.Status, &createdAt, &j.ApplicantCount); err != nil {
			continue
		}
		j.CreatedAt = createdAt.Format("2006-01-02")
		results = append(results, j)
	}
	if results == nil {
		results = []jobResult{}
	}
	return toJSON(map[string]any{
		"total": len(results),
		"jobs":  results,
	})
}

func (r *ToolRegistry) toolCreateJobPosting(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if user.Role != "admin" && user.Role != "super_admin" && user.Role != "manager" {
		return "", fmt.Errorf("only admins and managers can create job postings")
	}

	title, _ := input["title"].(string)
	if title == "" {
		return "", fmt.Errorf("title is required")
	}

	description, _ := input["description"].(string)
	requirements, _ := input["requirements"].(string)
	employmentType, _ := input["employment_type"].(string)
	if employmentType == "" {
		employmentType = "regular"
	}
	location, _ := input["location"].(string)

	// Fuzzy-match department
	var deptID *int64
	if deptName, ok := input["department"].(string); ok && deptName != "" {
		var did int64
		err := r.pool.QueryRow(ctx, `
			SELECT id FROM departments
			WHERE company_id = $1 AND is_active = true AND name ILIKE '%' || $2 || '%'
			ORDER BY CASE WHEN LOWER(name) = LOWER($2) THEN 0 ELSE 1 END, id
			LIMIT 1
		`, companyID, deptName).Scan(&did)
		if err == nil {
			deptID = &did
		}
	}

	// Salary
	var salMin, salMax *string
	if v, ok := input["salary_min"].(float64); ok && v > 0 {
		s := fmt.Sprintf("%.2f", v)
		salMin = &s
	}
	if v, ok := input["salary_max"].(float64); ok && v > 0 {
		s := fmt.Sprintf("%.2f", v)
		salMax = &s
	}

	var id int64
	err = r.pool.QueryRow(ctx, `
		INSERT INTO job_postings (company_id, title, department_id, description, requirements,
			salary_min, salary_max, employment_type, location, status, created_by)
		VALUES ($1, $2, $3, $4, $5,
			CASE WHEN $6 IS NULL THEN NULL ELSE $6::NUMERIC END,
			CASE WHEN $7 IS NULL THEN NULL ELSE $7::NUMERIC END,
			$8, $9, 'draft', $10)
		RETURNING id
	`, companyID, title, deptID, description, requirements,
		salMin, salMax, employmentType, location, userID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create job posting: %w", err)
	}

	return toJSON(map[string]any{
		"success": true,
		"id":      id,
		"title":   title,
		"status":  "draft",
		"message": fmt.Sprintf("Job posting '%s' created as draft. Use the Recruitment page to publish it.", title),
	})
}

func (r *ToolRegistry) toolScreenApplicant(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if user.Role != "admin" && user.Role != "super_admin" && user.Role != "manager" {
		return "", fmt.Errorf("only admins and managers can screen applicants")
	}

	applicantID, ok := input["applicant_id"].(float64)
	if !ok || applicantID <= 0 {
		return "", fmt.Errorf("applicant_id is required")
	}

	score, ok := input["score"].(float64)
	if !ok || score < 0 || score > 100 {
		return "", fmt.Errorf("score must be between 0 and 100")
	}

	summary, _ := input["summary"].(string)
	if summary == "" {
		return "", fmt.Errorf("summary is required")
	}

	tag, err := r.pool.Exec(ctx, `
		UPDATE applicants
		SET ai_score = $3, ai_summary = $4, status = CASE WHEN status = 'new' THEN 'screening' ELSE status END, updated_at = now()
		WHERE id = $1 AND company_id = $2
	`, int64(applicantID), companyID, int(score), summary)
	if err != nil {
		return "", fmt.Errorf("screen applicant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return "", fmt.Errorf("applicant not found")
	}

	return toJSON(map[string]any{
		"success":      true,
		"applicant_id": int64(applicantID),
		"ai_score":     int(score),
		"message":      fmt.Sprintf("Applicant screened with score %d/100.", int(score)),
	})
}

func (r *ToolRegistry) toolRankCandidates(ctx context.Context, companyID, _ int64, input map[string]any) (string, error) {
	jobPostingID, ok := input["job_posting_id"].(float64)
	if !ok || jobPostingID <= 0 {
		return "", fmt.Errorf("job_posting_id is required")
	}

	// Verify job posting belongs to company
	var jobTitle string
	err := r.pool.QueryRow(ctx, `
		SELECT title FROM job_postings WHERE id = $1 AND company_id = $2
	`, int64(jobPostingID), companyID).Scan(&jobTitle)
	if err != nil {
		return "", fmt.Errorf("job posting not found")
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, first_name, last_name, email, ai_score, ai_summary, status, source
		FROM applicants
		WHERE job_posting_id = $1 AND company_id = $2
		ORDER BY COALESCE(ai_score, 0) DESC, applied_at ASC
	`, int64(jobPostingID), companyID)
	if err != nil {
		return "", fmt.Errorf("rank candidates: %w", err)
	}
	defer rows.Close()

	type candidate struct {
		Rank      int    `json:"rank"`
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AIScore   *int   `json:"ai_score"`
		AISummary string `json:"ai_summary,omitempty"`
		Status    string `json:"status"`
		Source    string `json:"source"`
	}
	var candidates []candidate
	rank := 1
	for rows.Next() {
		var c candidate
		var firstName, lastName string
		var aiSummary *string
		if err := rows.Scan(&c.ID, &firstName, &lastName, &c.Email,
			&c.AIScore, &aiSummary, &c.Status, &c.Source); err != nil {
			continue
		}
		c.Name = firstName + " " + lastName
		c.Rank = rank
		if aiSummary != nil {
			c.AISummary = *aiSummary
		}
		candidates = append(candidates, c)
		rank++
	}
	if candidates == nil {
		candidates = []candidate{}
	}

	return toJSON(map[string]any{
		"job_title":  jobTitle,
		"total":      len(candidates),
		"candidates": candidates,
	})
}

func (r *ToolRegistry) toolOnboardEmployee(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	// 1. Check admin/manager role
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if user.Role != "admin" && user.Role != "super_admin" && user.Role != "manager" {
		return "", fmt.Errorf("only admins and managers can onboard employees")
	}

	// 2. Extract required fields
	firstName, _ := input["first_name"].(string)
	lastName, _ := input["last_name"].(string)
	deptName, _ := input["department"].(string)
	if firstName == "" || lastName == "" {
		return "", fmt.Errorf("first_name and last_name are required")
	}

	hireDateStr, _ := input["hire_date"].(string)
	if hireDateStr == "" {
		hireDateStr = time.Now().Format("2006-01-02")
	}
	hireDate, err := time.Parse("2006-01-02", hireDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid hire_date format, use YYYY-MM-DD")
	}

	// 3. Fuzzy-match department
	var deptID int64
	var matchedDeptName string
	if deptName != "" {
		err := r.pool.QueryRow(ctx, `
			SELECT id, name FROM departments
			WHERE company_id = $1 AND is_active = true AND name ILIKE '%' || $2 || '%'
			ORDER BY CASE WHEN LOWER(name) = LOWER($2) THEN 0 ELSE 1 END, id
			LIMIT 1
		`, companyID, deptName).Scan(&deptID, &matchedDeptName)
		if err != nil {
			return "", fmt.Errorf("department '%s' not found. Please check available departments", deptName)
		}
	}

	// 4. Fuzzy-match position (optional)
	posName, _ := input["position"].(string)
	var posID int64
	var matchedPosName string
	if posName != "" {
		_ = r.pool.QueryRow(ctx, `
			SELECT id, title FROM positions
			WHERE company_id = $1 AND is_active = true AND title ILIKE '%' || $2 || '%'
			ORDER BY CASE WHEN LOWER(title) = LOWER($2) THEN 0 ELSE 1 END, id
			LIMIT 1
		`, companyID, posName).Scan(&posID, &matchedPosName)
	}

	// 5. Generate employee number (EMP-XXXXX format)
	var maxNum int
	_ = r.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(CAST(SUBSTRING(employee_no FROM 5) AS INTEGER)), 0)
		FROM employees WHERE company_id = $1 AND employee_no LIKE 'EMP-%'
	`, companyID).Scan(&maxNum)
	employeeNo := fmt.Sprintf("EMP-%05d", maxNum+1)

	empType := "probationary"
	if t, ok := input["employment_type"].(string); ok && t != "" {
		empType = t
	}

	email, _ := input["email"].(string)

	// 6. Create employee via raw SQL
	var empID int64
	err = r.pool.QueryRow(ctx, `
		INSERT INTO employees (
			company_id, employee_no, first_name, last_name,
			department_id, position_id, hire_date, employment_type,
			status, email
		) VALUES ($1, $2, $3, $4, $5, NULLIF($6::bigint, 0), $7, $8, 'active', NULLIF($9, ''))
		RETURNING id
	`, companyID, employeeNo, firstName, lastName,
		deptID, posID, hireDate, empType, email).Scan(&empID)
	if err != nil {
		return "", fmt.Errorf("create employee: %w", err)
	}

	// 7. Create employment history record
	if _, err := r.pool.Exec(ctx, `
		INSERT INTO employment_history (
			company_id, employee_id, action_type, effective_date,
			to_department_id, to_position_id, remarks, created_by
		) VALUES ($1, $2, 'hire', $3, NULLIF($4::bigint, 0), NULLIF($5::bigint, 0), $6, $7)
	`, companyID, empID, hireDate, deptID, posID,
		fmt.Sprintf("Onboarded via AI assistant by user %d", userID), userID); err != nil {
		slog.Error("failed to create employment history for onboarded employee", "employee_id", empID, "error", err)
	}

	// 8. Assign salary if provided
	salaryMsg := ""
	if salary, ok := input["basic_salary"].(float64); ok && salary > 0 {
		_, salErr := r.pool.Exec(ctx, `
			INSERT INTO employee_salaries (
				company_id, employee_id, basic_salary, effective_from, remarks, created_by
			) VALUES ($1, $2, $3, $4, 'Initial salary - onboarded via AI', $5)
		`, companyID, empID, salary, hireDate, userID)
		if salErr == nil {
			salaryMsg = fmt.Sprintf(" with PHP %.2f/month salary", salary)
		}
	}

	result := map[string]any{
		"success":         true,
		"employee_id":     empID,
		"employee_no":     employeeNo,
		"name":            firstName + " " + lastName,
		"department":      matchedDeptName,
		"hire_date":       hireDateStr,
		"employment_type": empType,
		"message": fmt.Sprintf("Employee %s (%s) has been successfully onboarded to %s department starting %s%s.",
			firstName+" "+lastName, employeeNo, matchedDeptName, hireDateStr, salaryMsg),
	}
	if matchedPosName != "" {
		result["position"] = matchedPosName
	}

	return toJSON(result)
}

// recruitmentDefs returns tool definitions for recruitment-related tools.
func recruitmentDefs() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "list_job_postings",
			Description: "List job postings for the company. Optionally filter by status (draft, open, closed, on_hold). Returns title, department, applicant count, and status.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status": map[string]any{"type": "string", "description": "Filter by status: draft, open, closed, on_hold. Omit for all."},
				},
			}),
		},
		{
			Name:        "create_job_posting",
			Description: "Create a new job posting. Admin/manager only. Always confirm details with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title":           map[string]any{"type": "string", "description": "Job title, e.g. 'Senior PHP Developer'."},
					"department":      map[string]any{"type": "string", "description": "Department name (will fuzzy-match)."},
					"description":     map[string]any{"type": "string", "description": "Full job description."},
					"requirements":    map[string]any{"type": "string", "description": "Job requirements/qualifications."},
					"salary_min":      map[string]any{"type": "number", "description": "Minimum monthly salary in PHP."},
					"salary_max":      map[string]any{"type": "number", "description": "Maximum monthly salary in PHP."},
					"employment_type": map[string]any{"type": "string", "description": "Employment type: regular, contractual, probationary, part_time, intern. Default: regular."},
					"location":        map[string]any{"type": "string", "description": "Work location, e.g. 'BGC, Taguig'."},
				},
				"required": []string{"title"},
			}),
		},
		{
			Name:        "screen_applicant",
			Description: "AI-screen an applicant: read their resume text, score them 0-100 against the job requirements, and save the AI score and summary. Admin/manager only.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"applicant_id": map[string]any{"type": "integer", "description": "Applicant ID to screen."},
					"score":        map[string]any{"type": "integer", "description": "AI assessment score 0-100."},
					"summary":      map[string]any{"type": "string", "description": "Brief assessment summary (2-3 sentences)."},
				},
				"required": []string{"applicant_id", "score", "summary"},
			}),
		},
		{
			Name:        "rank_candidates",
			Description: "List and rank applicants for a specific job posting by their AI score. Returns applicants sorted by score descending with their status and summary.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"job_posting_id": map[string]any{"type": "integer", "description": "Job posting ID to rank candidates for."},
				},
				"required": []string{"job_posting_id"},
			}),
		},
	}
}

// registerRecruitmentTools registers recruitment-related tool executors.
func (r *ToolRegistry) registerRecruitmentTools() {
	r.tools["list_job_postings"] = r.toolListJobPostings
	r.tools["create_job_posting"] = r.toolCreateJobPosting
	r.tools["screen_applicant"] = r.toolScreenApplicant
	r.tools["rank_candidates"] = r.toolRankCandidates
}
