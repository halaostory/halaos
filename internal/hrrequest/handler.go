package hrrequest

import (
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

// CreateRequest creates a new HR service request.
func (h *Handler) CreateRequest(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	var req struct {
		RequestType string  `json:"request_type" binding:"required"`
		Subject     string  `json:"subject" binding:"required"`
		Description string  `json:"description"`
		Priority    string  `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	ctx := c.Request.Context()

	emp, err := h.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.BadRequest(c, "Employee not found")
		return
	}

	priority := req.Priority
	if priority == "" {
		priority = "normal"
	}

	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}

	hr, err := h.queries.CreateHRRequest(ctx, store.CreateHRRequestParams{
		CompanyID:   companyID,
		EmployeeID:  emp.ID,
		RequestType: req.RequestType,
		Subject:     req.Subject,
		Description: desc,
		Priority:    priority,
	})
	if err != nil {
		h.logger.Error("failed to create HR request", "error", err)
		response.InternalError(c, "Failed to create request")
		return
	}

	response.Created(c, hr)
}

// ListMyRequests returns the current employee's requests.
func (h *Handler) ListMyRequests(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	ctx := c.Request.Context()

	emp, err := h.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.OK(c, []any{})
		return
	}

	limit := int32(20)
	offset := int32(0)
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = int32(v)
		}
	}
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = int32(v)
		}
	}

	reqs, err := h.queries.ListHRRequestsByEmployee(ctx, store.ListHRRequestsByEmployeeParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		response.InternalError(c, "Failed to list requests")
		return
	}
	response.OK(c, reqs)
}

// ListAllRequests returns all HR requests (admin/HR view).
func (h *Handler) ListAllRequests(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	status := c.DefaultQuery("status", "")
	requestType := c.DefaultQuery("request_type", "")
	limit := int32(30)
	offset := int32(0)
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = int32(v)
		}
	}
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = int32(v)
		}
	}

	reqs, err := h.queries.ListHRRequests(c.Request.Context(), store.ListHRRequestsParams{
		CompanyID: companyID,
		Column2:   status,
		Column3:   requestType,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		h.logger.Error("failed to list HR requests", "error", err)
		response.InternalError(c, "Failed to list requests")
		return
	}
	response.OK(c, reqs)
}

// GetRequest returns a single HR request.
func (h *Handler) GetRequest(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid request ID")
		return
	}

	hr, err := h.queries.GetHRRequest(c.Request.Context(), store.GetHRRequestParams{
		ID:        id,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Request not found")
		return
	}
	response.OK(c, hr)
}

// UpdateStatus updates the status of an HR request (admin/HR).
func (h *Handler) UpdateStatus(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid request ID")
		return
	}

	var req struct {
		Status         string  `json:"status" binding:"required"`
		ResolutionNote string  `json:"resolution_note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var note *string
	if req.ResolutionNote != "" {
		note = &req.ResolutionNote
	}

	hr, err := h.queries.UpdateHRRequestStatus(c.Request.Context(), store.UpdateHRRequestStatusParams{
		ID:             id,
		CompanyID:      companyID,
		Status:         req.Status,
		AssignedTo:     &userID,
		ResolutionNote: note,
	})
	if err != nil {
		h.logger.Error("failed to update HR request", "error", err)
		response.InternalError(c, "Failed to update request")
		return
	}
	response.OK(c, hr)
}

// GetStats returns request statistics.
func (h *Handler) GetStats(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	ctx := c.Request.Context()

	statusCounts, err := h.queries.CountHRRequestsByStatus(ctx, companyID)
	if err != nil {
		response.InternalError(c, "Failed to get stats")
		return
	}

	openCount, _ := h.queries.CountOpenHRRequests(ctx, companyID)

	response.OK(c, gin.H{
		"by_status":  statusCounts,
		"open_count": openCount,
	})
}
