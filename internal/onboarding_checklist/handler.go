package onboarding_checklist

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Valid step keys per persona.
var validSteps = map[string][]string{
	"employee": {"profile", "first_clock", "view_leave", "view_payslip", "ai_chat"},
	"admin":    {"company_info", "departments", "import_employees", "leave_policies", "schedules", "payroll_config", "first_payroll"},
}

type stepState struct {
	Done   bool   `json:"done"`
	DoneAt string `json:"done_at,omitempty"`
}

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger}
}

// GetMyProgress returns the current user's onboarding checklist state.
func (h *Handler) GetMyProgress(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	persona := c.DefaultQuery("persona", "employee")

	row, err := h.queries.GetOnboardingChecklist(c.Request.Context(), store.GetOnboardingChecklistParams{
		CompanyID: companyID,
		UserID:    userID,
		Persona:   persona,
	})
	if err == pgx.ErrNoRows {
		// Auto-create with all steps as not done
		steps := buildInitialSteps(persona)
		stepsJSON, _ := json.Marshal(steps)
		row, err = h.queries.UpsertOnboardingChecklist(c.Request.Context(), store.UpsertOnboardingChecklistParams{
			CompanyID: companyID,
			UserID:    userID,
			Persona:   persona,
			Steps:     stepsJSON,
		})
		if err != nil {
			h.logger.Error("failed to create onboarding progress", "error", err)
			response.InternalError(c, "Failed to create onboarding progress")
			return
		}
	} else if err != nil {
		h.logger.Error("failed to get onboarding progress", "error", err)
		response.InternalError(c, "Failed to get onboarding progress")
		return
	}

	// For admin persona, auto-detect step completion from DB state
	if persona == "admin" {
		steps := h.autoDetectAdminSteps(c, companyID, row.Steps)
		if steps != nil {
			stepsJSON, _ := json.Marshal(steps)
			row, _ = h.queries.UpsertOnboardingChecklist(c.Request.Context(), store.UpsertOnboardingChecklistParams{
				CompanyID: companyID,
				UserID:    userID,
				Persona:   persona,
				Steps:     stepsJSON,
			})
		}
	}

	response.OK(c, row)
}

// CompleteStep marks a single step as done.
func (h *Handler) CompleteStep(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	var req struct {
		Step    string `json:"step" binding:"required"`
		Persona string `json:"persona"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.Persona == "" {
		req.Persona = "employee"
	}

	// Validate step key
	keys, ok := validSteps[req.Persona]
	if !ok {
		response.BadRequest(c, "invalid persona")
		return
	}
	valid := false
	for _, k := range keys {
		if k == req.Step {
			valid = true
			break
		}
	}
	if !valid {
		response.BadRequest(c, "invalid step key")
		return
	}

	// Get or create progress
	row, err := h.queries.GetOnboardingChecklist(c.Request.Context(), store.GetOnboardingChecklistParams{
		CompanyID: companyID, UserID: userID, Persona: req.Persona,
	})
	if err == pgx.ErrNoRows {
		steps := buildInitialSteps(req.Persona)
		stepsJSON, _ := json.Marshal(steps)
		row, err = h.queries.UpsertOnboardingChecklist(c.Request.Context(), store.UpsertOnboardingChecklistParams{
			CompanyID: companyID, UserID: userID, Persona: req.Persona, Steps: stepsJSON,
		})
	}
	if err != nil {
		response.InternalError(c, "Failed to get onboarding progress")
		return
	}

	// Parse steps, mark done (idempotent)
	var steps map[string]stepState
	if err := json.Unmarshal(row.Steps, &steps); err != nil {
		steps = buildInitialSteps(req.Persona)
	}
	s := steps[req.Step]
	if !s.Done {
		s.Done = true
		s.DoneAt = time.Now().UTC().Format(time.RFC3339)
		steps[req.Step] = s
	}

	stepsJSON, _ := json.Marshal(steps)
	row, err = h.queries.UpsertOnboardingChecklist(c.Request.Context(), store.UpsertOnboardingChecklistParams{
		CompanyID: companyID, UserID: userID, Persona: req.Persona, Steps: stepsJSON,
	})
	if err != nil {
		response.InternalError(c, "Failed to update step")
		return
	}

	// Check if all steps done → mark completed
	allDone := true
	if err := json.Unmarshal(row.Steps, &steps); err == nil {
		for _, k := range validSteps[req.Persona] {
			if !steps[k].Done {
				allDone = false
				break
			}
		}
	}
	if allDone && row.CompletedAt.Time.IsZero() {
		_ = h.queries.CompleteOnboardingChecklist(c.Request.Context(), store.CompleteOnboardingChecklistParams{
			CompanyID: companyID, UserID: userID, Persona: req.Persona,
		})
	}

	response.OK(c, row)
}

// Dismiss hides the checklist for the user.
func (h *Handler) Dismiss(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	persona := c.DefaultQuery("persona", "employee")

	err := h.queries.DismissOnboardingChecklist(c.Request.Context(), store.DismissOnboardingChecklistParams{
		CompanyID: companyID, UserID: userID, Persona: persona,
	})
	if err != nil {
		response.InternalError(c, "Failed to dismiss")
		return
	}
	response.OK(c, gin.H{"dismissed": true})
}

func buildInitialSteps(persona string) map[string]stepState {
	steps := make(map[string]stepState)
	for _, k := range validSteps[persona] {
		steps[k] = stepState{Done: false}
	}
	return steps
}

func (h *Handler) autoDetectAdminSteps(c *gin.Context, companyID int64, currentSteps json.RawMessage) map[string]stepState {
	var steps map[string]stepState
	if err := json.Unmarshal(currentSteps, &steps); err != nil {
		steps = buildInitialSteps("admin")
	}

	changed := false
	now := time.Now().UTC().Format(time.RFC3339)
	ctx := c.Request.Context()

	// Step 1: company_info — companies.legal_name IS NOT NULL AND tin IS NOT NULL
	infoComplete, err := h.queries.CheckCompanyInfoComplete(ctx, companyID)
	if err == nil && infoComplete && !steps["company_info"].Done {
		steps["company_info"] = stepState{Done: true, DoneAt: now}
		changed = true
	}

	// Step 2: departments — count > 0 AND positions count > 0
	deptCount, err := h.queries.CountDepartmentsByCompany(ctx, companyID)
	if err == nil && deptCount > 0 {
		posCount, err2 := h.queries.CountPositionsByCompany(ctx, companyID)
		if err2 == nil && posCount > 0 && !steps["departments"].Done {
			steps["departments"] = stepState{Done: true, DoneAt: now}
			changed = true
		}
	}

	// Step 3: import_employees — count > 1
	empCount, err := h.queries.CountEmployeesByCompany(ctx, companyID)
	if err == nil && empCount > 1 && !steps["import_employees"].Done {
		steps["import_employees"] = stepState{Done: true, DoneAt: now}
		changed = true
	}

	// Step 4: leave_policies
	ltCount, err := h.queries.CountNonStatutoryLeaveTypes(ctx, companyID)
	if err == nil && ltCount > 0 && !steps["leave_policies"].Done {
		steps["leave_policies"] = stepState{Done: true, DoneAt: now}
		changed = true
	}

	// Step 5: schedules
	stCount, err := h.queries.CountScheduleTemplates(ctx, companyID)
	if err == nil && stCount > 0 && !steps["schedules"].Done {
		steps["schedules"] = stepState{Done: true, DoneAt: now}
		changed = true
	}

	// Step 6: payroll_config
	ssCount, err := h.queries.CountSalaryStructures(ctx, companyID)
	if err == nil && ssCount > 0 && !steps["payroll_config"].Done {
		steps["payroll_config"] = stepState{Done: true, DoneAt: now}
		changed = true
	}

	// Step 7: first_payroll
	prCount, err := h.queries.CountCompletedPayrollRuns(ctx, companyID)
	if err == nil && prCount > 0 && !steps["first_payroll"].Done {
		steps["first_payroll"] = stepState{Done: true, DoneAt: now}
		changed = true
	}

	if changed {
		return steps
	}
	return nil
}
