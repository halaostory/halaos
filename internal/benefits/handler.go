package benefits

import (
	"fmt"
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

func (h *Handler) ListPlans(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	plans, err := h.queries.ListBenefitPlans(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list benefit plans")
		return
	}
	response.OK(c, plans)
}

func (h *Handler) GetPlan(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	plan, err := h.queries.GetBenefitPlan(c.Request.Context(), store.GetBenefitPlanParams{
		ID: planID, CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Plan not found")
		return
	}
	response.OK(c, plan)
}

func (h *Handler) CreatePlan(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	var req struct {
		Name              string  `json:"name" binding:"required"`
		Category          string  `json:"category" binding:"required"`
		Description       string  `json:"description"`
		Provider          string  `json:"provider"`
		EmployerShare     float64 `json:"employer_share"`
		EmployeeShare     float64 `json:"employee_share"`
		CoverageAmount    float64 `json:"coverage_amount"`
		EligibilityType   string  `json:"eligibility_type"`
		EligibilityMonths int32   `json:"eligibility_months"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	var erShare, eeShare, covAmt pgtype.Numeric
	_ = erShare.Scan(fmt.Sprintf("%.2f", req.EmployerShare))
	_ = eeShare.Scan(fmt.Sprintf("%.2f", req.EmployeeShare))
	_ = covAmt.Scan(fmt.Sprintf("%.2f", req.CoverageAmount))
	eligType := req.EligibilityType
	if eligType == "" {
		eligType = "all"
	}
	var desc, prov *string
	if req.Description != "" {
		desc = &req.Description
	}
	if req.Provider != "" {
		prov = &req.Provider
	}
	plan, err := h.queries.CreateBenefitPlan(c.Request.Context(), store.CreateBenefitPlanParams{
		CompanyID:         companyID,
		Name:              req.Name,
		Category:          req.Category,
		Description:       desc,
		Provider:          prov,
		EmployerShare:     erShare,
		EmployeeShare:     eeShare,
		CoverageAmount:    covAmt,
		EligibilityType:   eligType,
		EligibilityMonths: req.EligibilityMonths,
	})
	if err != nil {
		response.InternalError(c, "Failed to create benefit plan")
		return
	}
	response.Created(c, plan)
}

func (h *Handler) UpdatePlan(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	var req struct {
		Name              string  `json:"name" binding:"required"`
		Category          string  `json:"category" binding:"required"`
		Description       string  `json:"description"`
		Provider          string  `json:"provider"`
		EmployerShare     float64 `json:"employer_share"`
		EmployeeShare     float64 `json:"employee_share"`
		CoverageAmount    float64 `json:"coverage_amount"`
		EligibilityType   string  `json:"eligibility_type"`
		EligibilityMonths int32   `json:"eligibility_months"`
		IsActive          bool    `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	var erShare, eeShare, covAmt pgtype.Numeric
	_ = erShare.Scan(fmt.Sprintf("%.2f", req.EmployerShare))
	_ = eeShare.Scan(fmt.Sprintf("%.2f", req.EmployeeShare))
	_ = covAmt.Scan(fmt.Sprintf("%.2f", req.CoverageAmount))
	var desc, prov *string
	if req.Description != "" {
		desc = &req.Description
	}
	if req.Provider != "" {
		prov = &req.Provider
	}
	plan, err := h.queries.UpdateBenefitPlan(c.Request.Context(), store.UpdateBenefitPlanParams{
		ID:                planID,
		CompanyID:         companyID,
		Name:              req.Name,
		Category:          req.Category,
		Description:       desc,
		Provider:          prov,
		EmployerShare:     erShare,
		EmployeeShare:     eeShare,
		CoverageAmount:    covAmt,
		EligibilityType:   req.EligibilityType,
		EligibilityMonths: req.EligibilityMonths,
		IsActive:          req.IsActive,
	})
	if err != nil {
		response.InternalError(c, "Failed to update benefit plan")
		return
	}
	response.OK(c, plan)
}

func (h *Handler) GetSummary(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	summary, err := h.queries.GetBenefitsSummary(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get benefits summary")
		return
	}
	response.OK(c, summary)
}

func (h *Handler) ListEnrollments(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	status := c.Query("status")
	employeeID, _ := strconv.ParseInt(c.Query("employee_id"), 10, 64)
	enrollments, err := h.queries.ListBenefitEnrollments(c.Request.Context(), store.ListBenefitEnrollmentsParams{
		CompanyID:  companyID,
		Status:     status,
		EmployeeID: employeeID,
	})
	if err != nil {
		response.InternalError(c, "Failed to list enrollments")
		return
	}
	response.OK(c, enrollments)
}

func (h *Handler) ListMyEnrollments(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		CompanyID: companyID, UserID: &userID,
	})
	if err != nil {
		response.OK(c, []any{})
		return
	}
	enrollments, err := h.queries.ListMyEnrollments(c.Request.Context(), store.ListMyEnrollmentsParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
	})
	if err != nil {
		response.InternalError(c, "Failed to list enrollments")
		return
	}
	response.OK(c, enrollments)
}

func (h *Handler) CreateEnrollment(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	var req struct {
		EmployeeID    int64   `json:"employee_id" binding:"required"`
		PlanID        int64   `json:"plan_id" binding:"required"`
		EffectiveDate string  `json:"effective_date" binding:"required"`
		EmployerShare float64 `json:"employer_share"`
		EmployeeShare float64 `json:"employee_share"`
		Notes         string  `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	effDate, err := time.Parse("2006-01-02", req.EffectiveDate)
	if err != nil {
		response.BadRequest(c, "Invalid date format")
		return
	}
	var erShare, eeShare pgtype.Numeric
	_ = erShare.Scan(fmt.Sprintf("%.2f", req.EmployerShare))
	_ = eeShare.Scan(fmt.Sprintf("%.2f", req.EmployeeShare))
	var notes *string
	if req.Notes != "" {
		notes = &req.Notes
	}
	enrollment, err := h.queries.CreateBenefitEnrollment(c.Request.Context(), store.CreateBenefitEnrollmentParams{
		CompanyID:     companyID,
		EmployeeID:    req.EmployeeID,
		PlanID:        req.PlanID,
		Status:        "active",
		EffectiveDate: effDate,
		EmployerShare: erShare,
		EmployeeShare: eeShare,
		Notes:         notes,
	})
	if err != nil {
		response.InternalError(c, "Failed to create enrollment")
		return
	}
	response.Created(c, enrollment)
}

func (h *Handler) CancelEnrollment(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	enrollmentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	enrollment, err := h.queries.CancelBenefitEnrollment(c.Request.Context(), store.CancelBenefitEnrollmentParams{
		ID: enrollmentID, CompanyID: companyID,
	})
	if err != nil {
		response.InternalError(c, "Failed to cancel enrollment")
		return
	}
	response.OK(c, enrollment)
}

func (h *Handler) ListDependents(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	enrollmentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	deps, err := h.queries.ListBenefitDependents(c.Request.Context(), store.ListBenefitDependentsParams{
		EnrollmentID: enrollmentID, CompanyID: companyID,
	})
	if err != nil {
		response.InternalError(c, "Failed to list dependents")
		return
	}
	response.OK(c, deps)
}

func (h *Handler) AddDependent(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		CompanyID: companyID, UserID: &userID,
	})
	if err != nil {
		response.BadRequest(c, "Employee profile not found")
		return
	}
	employeeID := emp.ID
	enrollmentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	var req struct {
		Name         string `json:"name" binding:"required"`
		Relationship string `json:"relationship" binding:"required"`
		BirthDate    string `json:"birth_date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	var birthDate pgtype.Date
	if req.BirthDate != "" {
		t, err := time.Parse("2006-01-02", req.BirthDate)
		if err != nil {
			response.BadRequest(c, "Invalid birth date")
			return
		}
		birthDate = pgtype.Date{Time: t, Valid: true}
	}
	dep, err := h.queries.CreateBenefitDependent(c.Request.Context(), store.CreateBenefitDependentParams{
		CompanyID:    companyID,
		EmployeeID:   employeeID,
		EnrollmentID: enrollmentID,
		Name:         req.Name,
		Relationship: req.Relationship,
		BirthDate:    birthDate,
	})
	if err != nil {
		response.InternalError(c, "Failed to add dependent")
		return
	}
	response.Created(c, dep)
}

func (h *Handler) DeleteDependent(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	depID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	err = h.queries.DeleteBenefitDependent(c.Request.Context(), store.DeleteBenefitDependentParams{
		ID: depID, CompanyID: companyID,
	})
	if err != nil {
		response.InternalError(c, "Failed to delete dependent")
		return
	}
	response.OK(c, gin.H{"message": "Dependent deleted"})
}

func (h *Handler) ListClaims(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	status := c.Query("status")
	employeeID, _ := strconv.ParseInt(c.Query("employee_id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 { page = 1 }
	if limit < 1 { limit = 50 }
	claims, err := h.queries.ListBenefitClaims(c.Request.Context(), store.ListBenefitClaimsParams{
		CompanyID:  companyID,
		Status:     status,
		EmployeeID: employeeID,
		Off:        int32((page - 1) * limit),
		Lim:        int32(limit),
	})
	if err != nil {
		response.InternalError(c, "Failed to list claims")
		return
	}
	total, _ := h.queries.CountBenefitClaims(c.Request.Context(), store.CountBenefitClaimsParams{
		CompanyID:  companyID,
		Status:     status,
		EmployeeID: employeeID,
	})
	response.OK(c, gin.H{"items": claims, "total": total, "page": page, "limit": limit})
}

func (h *Handler) CreateClaim(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		CompanyID: companyID, UserID: &userID,
	})
	if err != nil {
		response.BadRequest(c, "Employee profile not found")
		return
	}
	employeeID := emp.ID
	var req struct {
		EnrollmentID int64   `json:"enrollment_id" binding:"required"`
		ClaimDate    string  `json:"claim_date" binding:"required"`
		Amount       float64 `json:"amount" binding:"required"`
		Description  string  `json:"description" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	claimDate, err := time.Parse("2006-01-02", req.ClaimDate)
	if err != nil {
		response.BadRequest(c, "Invalid date format")
		return
	}
	var amount pgtype.Numeric
	_ = amount.Scan(fmt.Sprintf("%.2f", req.Amount))
	claim, err := h.queries.CreateBenefitClaim(c.Request.Context(), store.CreateBenefitClaimParams{
		CompanyID:    companyID,
		EmployeeID:   employeeID,
		EnrollmentID: req.EnrollmentID,
		ClaimDate:    claimDate,
		Amount:       amount,
		Description:  req.Description,
	})
	if err != nil {
		response.InternalError(c, "Failed to create claim")
		return
	}
	response.Created(c, claim)
}

func (h *Handler) ApproveClaim(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	claimID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	claim, err := h.queries.ApproveBenefitClaim(c.Request.Context(), store.ApproveBenefitClaimParams{
		ID: claimID, CompanyID: companyID, ApprovedBy: &userID,
	})
	if err != nil {
		response.InternalError(c, "Failed to approve claim")
		return
	}
	response.OK(c, claim)
}

func (h *Handler) RejectClaim(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	claimID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	var reason *string
	if req.Reason != "" {
		reason = &req.Reason
	}
	claim, err := h.queries.RejectBenefitClaim(c.Request.Context(), store.RejectBenefitClaimParams{
		ID: claimID, CompanyID: companyID, RejectionReason: reason,
	})
	if err != nil {
		response.InternalError(c, "Failed to reject claim")
		return
	}
	response.OK(c, claim)
}
