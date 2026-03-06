package disciplinary

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

func (h *Handler) CreateIncident(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	var req struct {
		EmployeeID    int64  `json:"employee_id" binding:"required"`
		IncidentDate  string `json:"incident_date" binding:"required"`
		Category      string `json:"category" binding:"required"`
		Severity      string `json:"severity" binding:"required"`
		Description   string `json:"description" binding:"required"`
		Witnesses     string `json:"witnesses"`
		EvidenceNotes string `json:"evidence_notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	incDate, err := time.Parse("2006-01-02", req.IncidentDate)
	if err != nil {
		response.BadRequest(c, "Invalid date format")
		return
	}
	incident, err := h.queries.CreateDisciplinaryIncident(c.Request.Context(), store.CreateDisciplinaryIncidentParams{
		CompanyID:     companyID,
		EmployeeID:    req.EmployeeID,
		ReportedBy:    &userID,
		IncidentDate:  incDate,
		Category:      req.Category,
		Severity:      req.Severity,
		Description:   req.Description,
		Witnesses:     &req.Witnesses,
		EvidenceNotes: &req.EvidenceNotes,
	})
	if err != nil {
		response.InternalError(c, "Failed to create incident")
		return
	}
	response.Created(c, incident)
}

func (h *Handler) ListIncidents(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	status := c.Query("status")
	empIDStr := c.Query("employee_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 {
		page = 1
	}
	var empID int64
	if empIDStr != "" {
		empID, _ = strconv.ParseInt(empIDStr, 10, 64)
	}
	incidents, err := h.queries.ListDisciplinaryIncidents(c.Request.Context(), store.ListDisciplinaryIncidentsParams{
		CompanyID:  companyID,
		Status:     status,
		EmployeeID: empID,
		Lim:        int32(limit),
		Off:        int32((page - 1) * limit),
	})
	if err != nil {
		response.InternalError(c, "Failed to list incidents")
		return
	}
	count, err := h.queries.CountDisciplinaryIncidents(c.Request.Context(), store.CountDisciplinaryIncidentsParams{
		CompanyID:  companyID,
		Status:     status,
		EmployeeID: empID,
	})
	if err != nil {
		response.InternalError(c, "Failed to count incidents")
		return
	}
	response.OK(c, gin.H{"data": incidents, "total": count, "page": page, "limit": limit})
}

func (h *Handler) GetIncident(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	incident, err := h.queries.GetDisciplinaryIncident(c.Request.Context(), store.GetDisciplinaryIncidentParams{
		ID:        id,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Incident not found")
		return
	}
	actions, err := h.queries.ListActionsByIncident(c.Request.Context(), store.ListActionsByIncidentParams{
		IncidentID: &id,
		CompanyID:  companyID,
	})
	if err != nil {
		actions = []store.ListActionsByIncidentRow{}
	}
	response.OK(c, gin.H{"incident": incident, "actions": actions})
}

func (h *Handler) UpdateIncidentStatus(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Status          string `json:"status" binding:"required"`
		ResolutionNotes string `json:"resolution_notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	incident, err := h.queries.UpdateIncidentStatus(c.Request.Context(), store.UpdateIncidentStatusParams{
		ID:              id,
		CompanyID:       companyID,
		Status:          req.Status,
		ResolutionNotes: &req.ResolutionNotes,
		ResolvedBy:      &userID,
	})
	if err != nil {
		response.InternalError(c, "Failed to update incident")
		return
	}
	response.OK(c, incident)
}

func (h *Handler) CreateAction(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	var req struct {
		EmployeeID     int64  `json:"employee_id" binding:"required"`
		IncidentID     *int64 `json:"incident_id"`
		ActionType     string `json:"action_type" binding:"required"`
		ActionDate     string `json:"action_date" binding:"required"`
		Description    string `json:"description" binding:"required"`
		SuspensionDays *int32 `json:"suspension_days"`
		EffectiveDate  string `json:"effective_date"`
		EndDate        string `json:"end_date"`
		Notes          string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	actDate, err := time.Parse("2006-01-02", req.ActionDate)
	if err != nil {
		response.BadRequest(c, "Invalid action date")
		return
	}
	var effDate, endDatePg pgtype.Date
	if req.EffectiveDate != "" {
		t, _ := time.Parse("2006-01-02", req.EffectiveDate)
		effDate = pgtype.Date{Time: t, Valid: true}
	}
	if req.EndDate != "" {
		t, _ := time.Parse("2006-01-02", req.EndDate)
		endDatePg = pgtype.Date{Time: t, Valid: true}
	}
	action, err := h.queries.CreateDisciplinaryAction(c.Request.Context(), store.CreateDisciplinaryActionParams{
		CompanyID:      companyID,
		EmployeeID:     req.EmployeeID,
		IncidentID:     req.IncidentID,
		ActionType:     req.ActionType,
		ActionDate:     actDate,
		IssuedBy:       userID,
		Description:    req.Description,
		SuspensionDays: req.SuspensionDays,
		EffectiveDate:  effDate,
		EndDate:        endDatePg,
		Notes:          &req.Notes,
	})
	if err != nil {
		response.InternalError(c, "Failed to create disciplinary action")
		return
	}
	response.Created(c, action)
}

func (h *Handler) ListActions(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	empIDStr := c.Query("employee_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 {
		page = 1
	}
	var empID int64
	if empIDStr != "" {
		empID, _ = strconv.ParseInt(empIDStr, 10, 64)
	}
	actions, err := h.queries.ListDisciplinaryActions(c.Request.Context(), store.ListDisciplinaryActionsParams{
		CompanyID:  companyID,
		EmployeeID: empID,
		Lim:        int32(limit),
		Off:        int32((page - 1) * limit),
	})
	if err != nil {
		response.InternalError(c, "Failed to list actions")
		return
	}
	count, err := h.queries.CountDisciplinaryActions(c.Request.Context(), store.CountDisciplinaryActionsParams{
		CompanyID:  companyID,
		EmployeeID: empID,
	})
	if err != nil {
		response.InternalError(c, "Failed to count actions")
		return
	}
	response.OK(c, gin.H{"data": actions, "total": count, "page": page, "limit": limit})
}

func (h *Handler) AcknowledgeAction(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	action, err := h.queries.AcknowledgeDisciplinaryAction(c.Request.Context(), store.AcknowledgeDisciplinaryActionParams{
		ID:        id,
		CompanyID: companyID,
	})
	if err != nil {
		response.InternalError(c, "Failed to acknowledge action")
		return
	}
	response.OK(c, action)
}

func (h *Handler) AppealAction(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Appeal reason is required")
		return
	}
	action, err := h.queries.AppealDisciplinaryAction(c.Request.Context(), store.AppealDisciplinaryActionParams{
		ID:           id,
		CompanyID:    companyID,
		AppealReason: &req.Reason,
	})
	if err != nil {
		response.InternalError(c, "Failed to submit appeal")
		return
	}
	response.OK(c, action)
}

func (h *Handler) ResolveAppeal(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Status     string `json:"status" binding:"required"`
		Resolution string `json:"resolution"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	action, err := h.queries.ResolveAppeal(c.Request.Context(), store.ResolveAppealParams{
		ID:               id,
		CompanyID:        companyID,
		AppealStatus:     &req.Status,
		AppealResolution: &req.Resolution,
		AppealResolvedBy: &userID,
	})
	if err != nil {
		response.InternalError(c, "Failed to resolve appeal")
		return
	}
	response.OK(c, action)
}

func (h *Handler) GetEmployeeSummary(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	empID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	incSummary, err := h.queries.GetEmployeeDisciplinarySummary(c.Request.Context(), store.GetEmployeeDisciplinarySummaryParams{
		CompanyID:  companyID,
		EmployeeID: empID,
	})
	if err != nil {
		response.InternalError(c, "Failed to get summary")
		return
	}
	actCounts, err := h.queries.GetEmployeeActionCounts(c.Request.Context(), store.GetEmployeeActionCountsParams{
		CompanyID:  companyID,
		EmployeeID: empID,
	})
	if err != nil {
		response.InternalError(c, "Failed to get action counts")
		return
	}
	response.OK(c, gin.H{"incidents": incSummary, "actions": actCounts})
}
