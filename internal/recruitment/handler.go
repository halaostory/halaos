package recruitment

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/pkg/response"
)

// Handler manages recruitment endpoints.
type Handler struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewHandler creates a recruitment handler.
func NewHandler(pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{pool: pool, logger: logger}
}

// --- Job Postings ---

type createJobPostingRequest struct {
	Title          string  `json:"title" binding:"required"`
	DepartmentID   *int64  `json:"department_id"`
	PositionID     *int64  `json:"position_id"`
	Description    string  `json:"description"`
	Requirements   string  `json:"requirements"`
	SalaryMin      *string `json:"salary_min"`
	SalaryMax      *string `json:"salary_max"`
	EmploymentType string  `json:"employment_type"`
	Location       string  `json:"location"`
}

func (h *Handler) ListJobPostings(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	status := c.DefaultQuery("status", "")

	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT jp.id, jp.title, d.name AS department, p.title AS position,
		       jp.employment_type, jp.location, jp.status,
		       jp.posted_at, jp.closes_at, jp.created_at,
		       (SELECT COUNT(*) FROM applicants a WHERE a.job_posting_id = jp.id) AS applicant_count
		FROM job_postings jp
		LEFT JOIN departments d ON d.id = jp.department_id
		LEFT JOIN positions p ON p.id = jp.position_id
		WHERE jp.company_id = $1
		  AND ($2 = '' OR jp.status = $2)
		ORDER BY jp.created_at DESC
	`, companyID, status)
	if err != nil {
		h.logger.Error("failed to list job postings", "error", err)
		response.InternalError(c, "Failed to list job postings")
		return
	}
	defer rows.Close()

	var results []gin.H
	for rows.Next() {
		var (
			id              int64
			title           string
			department      *string
			position        *string
			employmentType  string
			location        string
			jpStatus        string
			postedAt        *time.Time
			closesAt        *time.Time
			createdAt       time.Time
			applicantCount  int64
		)
		if err := rows.Scan(&id, &title, &department, &position,
			&employmentType, &location, &jpStatus,
			&postedAt, &closesAt, &createdAt, &applicantCount); err != nil {
			h.logger.Error("scan job posting", "error", err)
			continue
		}
		results = append(results, gin.H{
			"id":              id,
			"title":           title,
			"department":      department,
			"position":        position,
			"employment_type": employmentType,
			"location":        location,
			"status":          jpStatus,
			"posted_at":       postedAt,
			"closes_at":       closesAt,
			"created_at":      createdAt,
			"applicant_count": applicantCount,
		})
	}
	if results == nil {
		results = []gin.H{}
	}
	response.OK(c, results)
}

func (h *Handler) GetJobPosting(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid job posting ID")
		return
	}

	var result struct {
		ID             int64      `json:"id"`
		Title          string     `json:"title"`
		DepartmentID   *int64     `json:"department_id"`
		PositionID     *int64     `json:"position_id"`
		Description    string     `json:"description"`
		Requirements   string     `json:"requirements"`
		SalaryMin      *float64   `json:"salary_min"`
		SalaryMax      *float64   `json:"salary_max"`
		EmploymentType string     `json:"employment_type"`
		Location       string     `json:"location"`
		Status         string     `json:"status"`
		PostedAt       *time.Time `json:"posted_at"`
		ClosesAt       *time.Time `json:"closes_at"`
		CreatedAt      time.Time  `json:"created_at"`
	}

	err = h.pool.QueryRow(c.Request.Context(), `
		SELECT id, title, department_id, position_id, description, requirements,
		       salary_min, salary_max, employment_type, location, status,
		       posted_at, closes_at, created_at
		FROM job_postings WHERE id = $1 AND company_id = $2
	`, id, companyID).Scan(
		&result.ID, &result.Title, &result.DepartmentID, &result.PositionID,
		&result.Description, &result.Requirements,
		&result.SalaryMin, &result.SalaryMax, &result.EmploymentType, &result.Location,
		&result.Status, &result.PostedAt, &result.ClosesAt, &result.CreatedAt,
	)
	if err != nil {
		response.NotFound(c, "Job posting not found")
		return
	}
	response.OK(c, result)
}

func (h *Handler) CreateJobPosting(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	var req createJobPostingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.EmploymentType == "" {
		req.EmploymentType = "regular"
	}

	var id int64
	err := h.pool.QueryRow(c.Request.Context(), `
		INSERT INTO job_postings (company_id, title, department_id, position_id,
		    description, requirements, salary_min, salary_max,
		    employment_type, location, created_by)
		VALUES ($1, $2, $3, $4, $5, $6,
		    CASE WHEN $7 = '' OR $7 IS NULL THEN NULL ELSE $7::NUMERIC END,
		    CASE WHEN $8 = '' OR $8 IS NULL THEN NULL ELSE $8::NUMERIC END,
		    $9, $10, $11)
		RETURNING id
	`, companyID, req.Title, req.DepartmentID, req.PositionID,
		req.Description, req.Requirements,
		req.SalaryMin, req.SalaryMax,
		req.EmploymentType, req.Location, userID,
	).Scan(&id)
	if err != nil {
		h.logger.Error("failed to create job posting", "error", err)
		response.InternalError(c, "Failed to create job posting")
		return
	}

	response.Created(c, gin.H{"id": id, "message": "Job posting created"})
}

func (h *Handler) UpdateJobPosting(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid job posting ID")
		return
	}

	var req struct {
		Title          *string `json:"title"`
		Description    *string `json:"description"`
		Requirements   *string `json:"requirements"`
		SalaryMin      *string `json:"salary_min"`
		SalaryMax      *string `json:"salary_max"`
		EmploymentType *string `json:"employment_type"`
		Location       *string `json:"location"`
		Status         *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tag, err := h.pool.Exec(c.Request.Context(), `
		UPDATE job_postings SET
		    title = COALESCE($3, title),
		    description = COALESCE($4, description),
		    requirements = COALESCE($5, requirements),
		    employment_type = COALESCE($6, employment_type),
		    location = COALESCE($7, location),
		    status = COALESCE($8, status),
		    posted_at = CASE WHEN $8 = 'open' AND posted_at IS NULL THEN now() ELSE posted_at END,
		    updated_at = now()
		WHERE id = $1 AND company_id = $2
	`, id, companyID, req.Title, req.Description, req.Requirements,
		req.EmploymentType, req.Location, req.Status,
	)
	if err != nil {
		h.logger.Error("failed to update job posting", "error", err)
		response.InternalError(c, "Failed to update job posting")
		return
	}
	if tag.RowsAffected() == 0 {
		response.NotFound(c, "Job posting not found")
		return
	}
	response.OK(c, gin.H{"message": "Job posting updated"})
}

// --- Applicants ---

type createApplicantRequest struct {
	JobPostingID int64  `json:"job_posting_id" binding:"required"`
	FirstName    string `json:"first_name" binding:"required"`
	LastName     string `json:"last_name" binding:"required"`
	Email        string `json:"email" binding:"required,email"`
	Phone        string `json:"phone"`
	ResumeURL    string `json:"resume_url"`
	ResumeText   string `json:"resume_text"`
	Source       string `json:"source"`
	Notes        string `json:"notes"`
}

func (h *Handler) ListApplicants(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	jobPostingID := c.DefaultQuery("job_posting_id", "0")
	status := c.DefaultQuery("status", "")

	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT a.id, a.first_name, a.last_name, a.email, a.phone,
		       a.ai_score, a.ai_summary, a.status, a.source, a.applied_at,
		       jp.title AS job_title
		FROM applicants a
		JOIN job_postings jp ON jp.id = a.job_posting_id
		WHERE a.company_id = $1
		  AND ($2 = '0' OR $2 = '' OR a.job_posting_id = $2::BIGINT)
		  AND ($3 = '' OR a.status = $3)
		ORDER BY COALESCE(a.ai_score, 0) DESC, a.applied_at DESC
	`, companyID, jobPostingID, status)
	if err != nil {
		h.logger.Error("failed to list applicants", "error", err)
		response.InternalError(c, "Failed to list applicants")
		return
	}
	defer rows.Close()

	var results []gin.H
	for rows.Next() {
		var (
			id        int64
			firstName string
			lastName  string
			email     string
			phone     *string
			aiScore   *int
			aiSummary *string
			aStatus   string
			source    string
			appliedAt time.Time
			jobTitle  string
		)
		if err := rows.Scan(&id, &firstName, &lastName, &email, &phone,
			&aiScore, &aiSummary, &aStatus, &source, &appliedAt, &jobTitle); err != nil {
			h.logger.Error("scan applicant", "error", err)
			continue
		}
		results = append(results, gin.H{
			"id":         id,
			"first_name": firstName,
			"last_name":  lastName,
			"email":      email,
			"phone":      phone,
			"ai_score":   aiScore,
			"ai_summary": aiSummary,
			"status":     aStatus,
			"source":     source,
			"applied_at": appliedAt,
			"job_title":  jobTitle,
		})
	}
	if results == nil {
		results = []gin.H{}
	}
	response.OK(c, results)
}

func (h *Handler) CreateApplicant(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	var req createApplicantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.Source == "" {
		req.Source = "manual"
	}

	var id int64
	err := h.pool.QueryRow(c.Request.Context(), `
		INSERT INTO applicants (company_id, job_posting_id, first_name, last_name,
		    email, phone, resume_url, resume_text, source, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`, companyID, req.JobPostingID, req.FirstName, req.LastName,
		req.Email, nilIfEmpty(req.Phone), nilIfEmpty(req.ResumeURL),
		nilIfEmpty(req.ResumeText), req.Source, nilIfEmpty(req.Notes),
	).Scan(&id)
	if err != nil {
		h.logger.Error("failed to create applicant", "error", err)
		response.InternalError(c, "Failed to create applicant")
		return
	}

	response.Created(c, gin.H{"id": id, "message": "Applicant added"})
}

func (h *Handler) UpdateApplicantStatus(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid applicant ID")
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
		Notes  string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tag, err := h.pool.Exec(c.Request.Context(), `
		UPDATE applicants SET status = $3, notes = COALESCE(NULLIF($4, ''), notes), updated_at = now()
		WHERE id = $1 AND company_id = $2
	`, id, companyID, req.Status, req.Notes)
	if err != nil {
		h.logger.Error("failed to update applicant status", "error", err)
		response.InternalError(c, "Failed to update applicant")
		return
	}
	if tag.RowsAffected() == 0 {
		response.NotFound(c, "Applicant not found")
		return
	}
	response.OK(c, gin.H{"message": "Applicant status updated"})
}

func (h *Handler) GetApplicant(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid applicant ID")
		return
	}

	row := h.pool.QueryRow(c.Request.Context(), `
		SELECT a.id, a.first_name, a.last_name, a.email, a.phone,
		       a.resume_url, a.resume_text, a.ai_score, a.ai_summary,
		       a.status, a.source, a.notes, a.applied_at,
		       jp.title AS job_title, jp.id AS job_posting_id
		FROM applicants a
		JOIN job_postings jp ON jp.id = a.job_posting_id
		WHERE a.id = $1 AND a.company_id = $2
	`, id, companyID)

	var result struct {
		ID           int64      `json:"id"`
		FirstName    string     `json:"first_name"`
		LastName     string     `json:"last_name"`
		Email        string     `json:"email"`
		Phone        *string    `json:"phone"`
		ResumeURL    *string    `json:"resume_url"`
		ResumeText   *string    `json:"resume_text"`
		AIScore      *int       `json:"ai_score"`
		AISummary    *string    `json:"ai_summary"`
		Status       string     `json:"status"`
		Source       string     `json:"source"`
		Notes        *string    `json:"notes"`
		AppliedAt    time.Time  `json:"applied_at"`
		JobTitle     string     `json:"job_title"`
		JobPostingID int64      `json:"job_posting_id"`
	}
	if err := row.Scan(
		&result.ID, &result.FirstName, &result.LastName, &result.Email, &result.Phone,
		&result.ResumeURL, &result.ResumeText, &result.AIScore, &result.AISummary,
		&result.Status, &result.Source, &result.Notes, &result.AppliedAt,
		&result.JobTitle, &result.JobPostingID,
	); err != nil {
		response.NotFound(c, "Applicant not found")
		return
	}

	// Also fetch interviews
	iRows, err := h.pool.Query(c.Request.Context(), `
		SELECT id, scheduled_at, duration_minutes, location, interview_type,
		       status, feedback, rating
		FROM interview_schedules WHERE applicant_id = $1
		ORDER BY scheduled_at
	`, id)
	var interviews []gin.H
	if err == nil {
		defer iRows.Close()
		for iRows.Next() {
			var (
				iID       int64
				schedAt   time.Time
				dur       int
				loc       *string
				iType     string
				iStatus   string
				feedback  *string
				rating    *int
			)
			if err := iRows.Scan(&iID, &schedAt, &dur, &loc, &iType, &iStatus, &feedback, &rating); err != nil {
				continue
			}
			interviews = append(interviews, gin.H{
				"id": iID, "scheduled_at": schedAt, "duration_minutes": dur,
				"location": loc, "interview_type": iType, "status": iStatus,
				"feedback": feedback, "rating": rating,
			})
		}
	}

	response.OK(c, gin.H{
		"applicant":  result,
		"interviews": interviews,
	})
}

// --- Interviews ---

func (h *Handler) ScheduleInterview(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	applicantID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid applicant ID")
		return
	}

	var req struct {
		ScheduledAt     string `json:"scheduled_at" binding:"required"`
		DurationMinutes int    `json:"duration_minutes"`
		Location        string `json:"location"`
		InterviewType   string `json:"interview_type"`
		InterviewerID   *int64 `json:"interviewer_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	schedAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		response.BadRequest(c, "Invalid scheduled_at format (use RFC3339)")
		return
	}
	if req.DurationMinutes <= 0 {
		req.DurationMinutes = 60
	}
	if req.InterviewType == "" {
		req.InterviewType = "initial"
	}

	// Verify applicant belongs to company
	var exists bool
	_ = h.pool.QueryRow(c.Request.Context(),
		"SELECT EXISTS(SELECT 1 FROM applicants WHERE id = $1 AND company_id = $2)",
		applicantID, companyID).Scan(&exists)
	if !exists {
		response.NotFound(c, "Applicant not found")
		return
	}

	var id int64
	err = h.pool.QueryRow(c.Request.Context(), `
		INSERT INTO interview_schedules (applicant_id, interviewer_id, scheduled_at,
		    duration_minutes, location, interview_type)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, applicantID, req.InterviewerID, schedAt,
		req.DurationMinutes, nilIfEmpty(req.Location), req.InterviewType,
	).Scan(&id)
	if err != nil {
		h.logger.Error("failed to schedule interview", "error", err)
		response.InternalError(c, "Failed to schedule interview")
		return
	}

	// Update applicant status to 'interview' if still in screening
	_, _ = h.pool.Exec(c.Request.Context(), `
		UPDATE applicants SET status = 'interview', updated_at = now()
		WHERE id = $1 AND company_id = $2 AND status IN ('new', 'screening')
	`, applicantID, companyID)

	response.Created(c, gin.H{"id": id, "message": "Interview scheduled"})
}

// --- Dashboard stats ---

func (h *Handler) GetRecruitmentStats(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	var stats struct {
		OpenPositions    int64 `json:"open_positions"`
		TotalApplicants  int64 `json:"total_applicants"`
		InPipeline       int64 `json:"in_pipeline"`
		HiredThisMonth   int64 `json:"hired_this_month"`
	}

	row := h.pool.QueryRow(c.Request.Context(), `
		SELECT
		    (SELECT COUNT(*) FROM job_postings WHERE company_id = $1 AND status = 'open'),
		    (SELECT COUNT(*) FROM applicants WHERE company_id = $1),
		    (SELECT COUNT(*) FROM applicants WHERE company_id = $1 AND status IN ('screening', 'interview', 'offer')),
		    (SELECT COUNT(*) FROM applicants WHERE company_id = $1 AND status = 'hired'
		        AND updated_at >= date_trunc('month', CURRENT_DATE))
	`, companyID)

	if err := row.Scan(&stats.OpenPositions, &stats.TotalApplicants,
		&stats.InPipeline, &stats.HiredThisMonth); err != nil {
		h.logger.Error("failed to get recruitment stats", "error", err)
		response.InternalError(c, "Failed to get stats")
		return
	}

	response.OK(c, stats)
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
