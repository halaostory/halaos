package selfservice

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger}
}

// GetMyInfo returns the employee's full info including department, position, manager
func (h *Handler) GetMyInfo(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	emp, err := h.queries.GetEmployeeFullInfo(c.Request.Context(), store.GetEmployeeFullInfoParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.OK(c, nil)
		return
	}

	// Also get profile
	profile, _ := h.queries.GetEmployeeProfile(c.Request.Context(), store.GetEmployeeProfileParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})

	response.OK(c, gin.H{
		"employee": emp,
		"profile":  profile,
	})
}

// GetMyTeam returns team members (employees with same manager)
func (h *Handler) GetMyTeam(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.OK(c, gin.H{"team": []any{}, "manager": nil})
		return
	}

	// Get team members who share the same manager
	var team []store.ListTeamMembersRow
	if emp.ManagerID != nil {
		team, _ = h.queries.ListTeamMembers(c.Request.Context(), store.ListTeamMembersParams{
			CompanyID: companyID,
			ManagerID: emp.ManagerID,
		})
	}

	// Also get direct reports if I'm a manager
	directReports, _ := h.queries.ListTeamMembers(c.Request.Context(), store.ListTeamMembersParams{
		CompanyID: companyID,
		ManagerID: &emp.ID,
	})

	// Get manager info
	var manager *store.GetMyManagerRow
	if emp.ManagerID != nil {
		m, err := h.queries.GetMyManager(c.Request.Context(), store.GetMyManagerParams{
			ID:        *emp.ManagerID,
			CompanyID: companyID,
		})
		if err == nil {
			manager = &m
		}
	}

	response.OK(c, gin.H{
		"team":           team,
		"direct_reports": directReports,
		"manager":        manager,
	})
}

// GetMyCompensation returns salary + latest payslip summary
func (h *Handler) GetMyCompensation(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.OK(c, gin.H{"salary": nil, "latest_payslip": nil})
		return
	}

	salary, _ := h.queries.GetMyCompensation(c.Request.Context(), store.GetMyCompensationParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
	})

	payslip, _ := h.queries.GetMyLatestPayslip(c.Request.Context(), store.GetMyLatestPayslipParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
	})

	response.OK(c, gin.H{
		"salary":         salary,
		"latest_payslip": payslip,
	})
}

// GetMyOnboarding returns onboarding tasks for the employee
func (h *Handler) GetMyOnboarding(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.OK(c, gin.H{"tasks": []any{}, "progress": nil})
		return
	}

	tasks, _ := h.queries.ListOnboardingTasks(c.Request.Context(), store.ListOnboardingTasksParams{
		EmployeeID:   emp.ID,
		WorkflowType: "onboarding",
		CompanyID:    companyID,
	})
	progress, _ := h.queries.GetOnboardingProgress(c.Request.Context(), store.GetOnboardingProgressParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})

	response.OK(c, gin.H{
		"tasks":    tasks,
		"progress": progress,
	})
}
