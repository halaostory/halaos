package training

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

func (h *Handler) ListTrainings(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 { page = 1 }
	if limit < 1 { limit = 50 }
	items, err := h.queries.ListTrainings(c.Request.Context(), store.ListTrainingsParams{
		CompanyID: companyID,
		Limit:     int32(limit),
		Offset:    int32((page - 1) * limit),
	})
	if err != nil {
		response.InternalError(c, "Failed to list trainings")
		return
	}
	response.OK(c, items)
}

func (h *Handler) CreateTraining(c *gin.Context) {
	var req struct {
		Title           string  `json:"title" binding:"required"`
		Description     *string `json:"description"`
		Trainer         *string `json:"trainer"`
		TrainingType    string  `json:"training_type"`
		StartDate       string  `json:"start_date" binding:"required"`
		EndDate         *string `json:"end_date"`
		MaxParticipants *int32  `json:"max_participants"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	startDate, _ := time.Parse("2006-01-02", req.StartDate)
	var endDate pgtype.Date
	if req.EndDate != nil {
		parsed, _ := time.Parse("2006-01-02", *req.EndDate)
		endDate = pgtype.Date{Time: parsed, Valid: true}
	}
	if req.TrainingType == "" { req.TrainingType = "internal" }
	training, err := h.queries.CreateTraining(c.Request.Context(), store.CreateTrainingParams{
		CompanyID:       companyID,
		Title:           req.Title,
		Description:     req.Description,
		Trainer:         req.Trainer,
		TrainingType:    req.TrainingType,
		StartDate:       startDate,
		EndDate:         endDate,
		MaxParticipants: req.MaxParticipants,
		CreatedBy:       &userID,
	})
	if err != nil {
		response.InternalError(c, "Failed to create training")
		return
	}
	response.Created(c, training)
}

func (h *Handler) UpdateTrainingStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	var req struct { Status string `json:"status" binding:"required"` }
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	companyID := auth.GetCompanyID(c)
	training, err := h.queries.UpdateTrainingStatus(c.Request.Context(), store.UpdateTrainingStatusParams{
		ID: id, CompanyID: companyID, Status: req.Status,
	})
	if err != nil {
		response.InternalError(c, "Failed to update training")
		return
	}
	response.OK(c, training)
}

func (h *Handler) ListParticipants(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	companyID := auth.GetCompanyID(c)
	items, err := h.queries.ListTrainingParticipants(c.Request.Context(), store.ListTrainingParticipantsParams{
		TrainingID: id,
		CompanyID:  companyID,
	})
	if err != nil {
		response.InternalError(c, "Failed to list participants")
		return
	}
	response.OK(c, items)
}

func (h *Handler) AddParticipant(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	var req struct { EmployeeID int64 `json:"employee_id" binding:"required"` }
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	p, err := h.queries.AddTrainingParticipant(c.Request.Context(), store.AddTrainingParticipantParams{
		TrainingID: id, EmployeeID: req.EmployeeID,
	})
	if err != nil {
		response.InternalError(c, "Failed to add participant")
		return
	}
	response.Created(c, p)
}

func (h *Handler) ListCertifications(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 { page = 1 }
	if limit < 1 { limit = 50 }
	var empID int64
	if eid := c.Query("employee_id"); eid != "" {
		empID, _ = strconv.ParseInt(eid, 10, 64)
	}
	items, err := h.queries.ListCertifications(c.Request.Context(), store.ListCertificationsParams{
		CompanyID: companyID,
		Column2:   empID,
		Limit:     int32(limit),
		Offset:    int32((page - 1) * limit),
	})
	if err != nil {
		response.InternalError(c, "Failed to list certifications")
		return
	}
	response.OK(c, items)
}

func (h *Handler) CreateCertification(c *gin.Context) {
	var req struct {
		EmployeeID   int64   `json:"employee_id" binding:"required"`
		Name         string  `json:"name" binding:"required"`
		IssuingBody  *string `json:"issuing_body"`
		CredentialID *string `json:"credential_id"`
		IssueDate    string  `json:"issue_date" binding:"required"`
		ExpiryDate   *string `json:"expiry_date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	companyID := auth.GetCompanyID(c)
	issueDate, _ := time.Parse("2006-01-02", req.IssueDate)
	var expiryDate pgtype.Date
	if req.ExpiryDate != nil {
		parsed, _ := time.Parse("2006-01-02", *req.ExpiryDate)
		expiryDate = pgtype.Date{Time: parsed, Valid: true}
	}
	cert, err := h.queries.CreateCertification(c.Request.Context(), store.CreateCertificationParams{
		CompanyID:    companyID,
		EmployeeID:   req.EmployeeID,
		Name:         req.Name,
		IssuingBody:  req.IssuingBody,
		CredentialID: req.CredentialID,
		IssueDate:    issueDate,
		ExpiryDate:   expiryDate,
	})
	if err != nil {
		response.InternalError(c, "Failed to create certification")
		return
	}
	response.Created(c, cert)
}

func (h *Handler) DeleteCertification(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	companyID := auth.GetCompanyID(c)
	if err := h.queries.DeleteCertification(c.Request.Context(), store.DeleteCertificationParams{
		ID: id, CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to delete certification")
		return
	}
	response.OK(c, nil)
}

func (h *Handler) ListExpiringCertifications(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	items, err := h.queries.ListExpiringCertifications(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list expiring certifications")
		return
	}
	response.OK(c, items)
}
