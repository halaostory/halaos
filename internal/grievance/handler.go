package grievance

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
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

func (h *Handler) GetSummary(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	summary, err := h.queries.GetGrievanceSummary(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get grievance summary")
		return
	}
	response.OK(c, summary)
}

func (h *Handler) List(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	status := c.Query("status")
	category := c.Query("category")
	employeeID, _ := strconv.ParseInt(c.Query("employee_id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	cases, err := h.queries.ListGrievances(c.Request.Context(), store.ListGrievancesParams{
		CompanyID:  companyID,
		Status:     status,
		Category:   category,
		EmployeeID: employeeID,
		Off:        int32((page - 1) * limit),
		Lim:        int32(limit),
	})
	if err != nil {
		response.InternalError(c, "Failed to list grievances")
		return
	}
	total, _ := h.queries.CountGrievances(c.Request.Context(), store.CountGrievancesParams{
		CompanyID:  companyID,
		Status:     status,
		Category:   category,
		EmployeeID: employeeID,
	})
	response.OK(c, gin.H{"items": cases, "total": total, "page": page, "limit": limit})
}

func (h *Handler) Get(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	g, err := h.queries.GetGrievance(c.Request.Context(), store.GetGrievanceParams{
		ID: id, CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Grievance not found")
		return
	}
	response.OK(c, g)
}

func (h *Handler) Create(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		CompanyID: companyID, UserID: &userID,
	})
	if err != nil {
		response.BadRequest(c, "Employee profile not found")
		return
	}
	var req struct {
		Category    string `json:"category" binding:"required"`
		Subject     string `json:"subject" binding:"required"`
		Description string `json:"description" binding:"required"`
		Severity    string `json:"severity"`
		IsAnonymous bool   `json:"is_anonymous"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	severity := req.Severity
	if severity == "" {
		severity = "medium"
	}
	nextNum, _ := h.queries.NextGrievanceCaseNumber(c.Request.Context(), companyID)
	caseNumber := fmt.Sprintf("GRV-%04d", nextNum)
	g, err := h.queries.CreateGrievance(c.Request.Context(), store.CreateGrievanceParams{
		CompanyID:   companyID,
		EmployeeID:  emp.ID,
		CaseNumber:  caseNumber,
		Category:    req.Category,
		Subject:     req.Subject,
		Description: req.Description,
		Severity:    severity,
		IsAnonymous: req.IsAnonymous,
	})
	if err != nil {
		response.InternalError(c, "Failed to create grievance")
		return
	}
	response.Created(c, g)
}

func (h *Handler) ListMy(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		CompanyID: companyID, UserID: &userID,
	})
	if err != nil {
		response.BadRequest(c, "Employee profile not found")
		return
	}
	cases, err := h.queries.ListMyGrievances(c.Request.Context(), store.ListMyGrievancesParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
	})
	if err != nil {
		response.InternalError(c, "Failed to list grievances")
		return
	}
	response.OK(c, cases)
}

func (h *Handler) UpdateStatus(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	g, err := h.queries.UpdateGrievanceStatus(c.Request.Context(), store.UpdateGrievanceStatusParams{
		ID: id, CompanyID: companyID, Status: req.Status,
	})
	if err != nil {
		response.InternalError(c, "Failed to update status")
		return
	}
	response.OK(c, g)
}

func (h *Handler) Assign(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		AssignedTo int64 `json:"assigned_to" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	g, err := h.queries.AssignGrievance(c.Request.Context(), store.AssignGrievanceParams{
		ID: id, CompanyID: companyID, AssignedTo: &req.AssignedTo,
	})
	if err != nil {
		response.InternalError(c, "Failed to assign grievance")
		return
	}
	response.OK(c, g)
}

func (h *Handler) Resolve(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Resolution string `json:"resolution" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	g, err := h.queries.ResolveGrievance(c.Request.Context(), store.ResolveGrievanceParams{
		ID: id, CompanyID: companyID, Resolution: &req.Resolution,
	})
	if err != nil {
		response.InternalError(c, "Failed to resolve grievance")
		return
	}
	response.OK(c, g)
}

func (h *Handler) Withdraw(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		CompanyID: companyID, UserID: &userID,
	})
	if err != nil {
		response.BadRequest(c, "Employee profile not found")
		return
	}
	g, err := h.queries.WithdrawGrievance(c.Request.Context(), store.WithdrawGrievanceParams{
		ID: id, CompanyID: companyID, EmployeeID: emp.ID,
	})
	if err != nil {
		response.InternalError(c, "Failed to withdraw grievance")
		return
	}
	response.OK(c, g)
}

func (h *Handler) ListComments(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	comments, err := h.queries.ListGrievanceComments(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "Failed to list comments")
		return
	}
	response.OK(c, comments)
}

func (h *Handler) AddComment(c *gin.Context) {
	userID := auth.GetUserID(c)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Comment    string `json:"comment" binding:"required"`
		IsInternal bool   `json:"is_internal"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	comment, err := h.queries.AddGrievanceComment(c.Request.Context(), store.AddGrievanceCommentParams{
		GrievanceID: id,
		UserID:      userID,
		Comment:     req.Comment,
		IsInternal:  req.IsInternal,
	})
	if err != nil {
		response.InternalError(c, "Failed to add comment")
		return
	}
	response.Created(c, comment)
}
