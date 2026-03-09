package onboarding

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger}
}

// ListTemplates returns all active onboarding/offboarding templates.
func (h *Handler) ListTemplates(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	templates, err := h.queries.ListOnboardingTemplates(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list templates")
		return
	}
	response.OK(c, templates)
}

// CreateTemplate creates a new onboarding/offboarding template.
func (h *Handler) CreateTemplate(c *gin.Context) {
	var req struct {
		WorkflowType string  `json:"workflow_type" binding:"required"`
		Title        string  `json:"title" binding:"required"`
		Description  *string `json:"description"`
		SortOrder    int32   `json:"sort_order"`
		IsRequired   bool    `json:"is_required"`
		AssigneeRole *string `json:"assignee_role"`
		DueDays      int32   `json:"due_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	template, err := h.queries.CreateOnboardingTemplate(c.Request.Context(), store.CreateOnboardingTemplateParams{
		CompanyID:    companyID,
		WorkflowType: req.WorkflowType,
		Title:        req.Title,
		Description:  req.Description,
		SortOrder:    req.SortOrder,
		IsRequired:   req.IsRequired,
		AssigneeRole: req.AssigneeRole,
		DueDays:      req.DueDays,
	})
	if err != nil {
		h.logger.Error("failed to create onboarding template", "error", err)
		response.InternalError(c, "Failed to create template")
		return
	}
	response.Created(c, template)
}

// InitiateWorkflow creates onboarding/offboarding tasks for an employee from templates.
func (h *Handler) InitiateWorkflow(c *gin.Context) {
	var req struct {
		EmployeeID   int64  `json:"employee_id" binding:"required"`
		WorkflowType string `json:"workflow_type" binding:"required"`
		ReferenceDate *string `json:"reference_date"` // defaults to today
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)

	refDate := time.Now()
	if req.ReferenceDate != nil {
		if d, err := time.Parse("2006-01-02", *req.ReferenceDate); err == nil {
			refDate = d
		}
	}

	// Get templates
	templates, err := h.queries.ListOnboardingTemplates(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get templates")
		return
	}

	var tasks []store.OnboardingTask
	for _, tmpl := range templates {
		if tmpl.WorkflowType != req.WorkflowType {
			continue
		}
		// company_id = 0 means global default template
		if tmpl.CompanyID != companyID && tmpl.CompanyID != 0 {
			continue
		}

		dueDate := pgtype.Date{}
		if tmpl.DueDays != 0 {
			d := refDate.AddDate(0, 0, int(tmpl.DueDays))
			dueDate = pgtype.Date{Time: d, Valid: true}
		}

		task, err := h.queries.CreateOnboardingTask(c.Request.Context(), store.CreateOnboardingTaskParams{
			CompanyID:    companyID,
			EmployeeID:   req.EmployeeID,
			TemplateID:   &tmpl.ID,
			WorkflowType: req.WorkflowType,
			Title:        tmpl.Title,
			Description:  tmpl.Description,
			IsRequired:   tmpl.IsRequired,
			AssigneeRole: tmpl.AssigneeRole,
			DueDate:      dueDate,
			SortOrder:    tmpl.SortOrder,
		})
		if err != nil {
			h.logger.Error("failed to create onboarding task", "template_id", tmpl.ID, "error", err)
			continue
		}
		tasks = append(tasks, task)
	}

	response.Created(c, tasks)
}

// ListTasks returns onboarding/offboarding tasks for a specific employee.
func (h *Handler) ListTasks(c *gin.Context) {
	employeeID, err := strconv.ParseInt(c.Param("employee_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}
	workflowType := c.DefaultQuery("type", "onboarding")

	companyID := auth.GetCompanyID(c)
	tasks, err := h.queries.ListOnboardingTasks(c.Request.Context(), store.ListOnboardingTasksParams{
		EmployeeID:   employeeID,
		WorkflowType: workflowType,
		CompanyID:    companyID,
	})
	if err != nil {
		response.InternalError(c, "Failed to list tasks")
		return
	}

	// Get progress
	progress, _ := h.queries.GetOnboardingProgress(c.Request.Context(), store.GetOnboardingProgressParams{
		EmployeeID: employeeID,
		CompanyID:  companyID,
	})

	response.OK(c, gin.H{
		"tasks":    tasks,
		"progress": progress,
	})
}

// ListPendingTasks returns all pending onboarding/offboarding tasks for the company.
func (h *Handler) ListPendingTasks(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	workflowType := c.DefaultQuery("type", "onboarding")

	tasks, err := h.queries.ListOnboardingTasksByCompany(c.Request.Context(), store.ListOnboardingTasksByCompanyParams{
		CompanyID:    companyID,
		WorkflowType: workflowType,
	})
	if err != nil {
		response.InternalError(c, "Failed to list tasks")
		return
	}
	response.OK(c, tasks)
}

// UpdateTaskStatus updates the status of an onboarding task.
func (h *Handler) UpdateTaskStatus(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid task ID")
		return
	}

	var req struct {
		Status string  `json:"status" binding:"required"`
		Notes  *string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	task, err := h.queries.UpdateOnboardingTaskStatus(c.Request.Context(), store.UpdateOnboardingTaskStatusParams{
		ID:          taskID,
		CompanyID:   companyID,
		Status:      req.Status,
		CompletedBy: &userID,
		Notes:       req.Notes,
	})
	if err != nil {
		response.NotFound(c, "Task not found")
		return
	}
	response.OK(c, task)
}

// GetProgress returns the onboarding progress for an employee.
func (h *Handler) GetProgress(c *gin.Context) {
	employeeID, err := strconv.ParseInt(c.Param("employee_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	progress, err := h.queries.GetOnboardingProgress(c.Request.Context(), store.GetOnboardingProgressParams{
		EmployeeID: employeeID,
		CompanyID:  companyID,
	})
	if err != nil {
		response.InternalError(c, "Failed to get progress")
		return
	}
	response.OK(c, progress)
}
