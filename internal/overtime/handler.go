package overtime

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/notification"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/pagination"
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

func (h *Handler) CreateRequest(c *gin.Context) {
	var req struct {
		OTDate  string  `json:"ot_date" binding:"required"`
		StartAt string  `json:"start_at" binding:"required"`
		EndAt   string  `json:"end_at" binding:"required"`
		Hours   string  `json:"hours" binding:"required"`
		OTType  string  `json:"ot_type"`
		Reason  *string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Employee not found")
		return
	}

	otDate, _ := time.Parse("2006-01-02", req.OTDate)
	startAt, _ := time.Parse(time.RFC3339, req.StartAt)
	endAt, _ := time.Parse(time.RFC3339, req.EndAt)
	otType := req.OTType
	if otType == "" {
		otType = "regular"
	}

	var hours pgtype.Numeric
	_ = hours.Scan(req.Hours)

	ot, err := h.queries.CreateOvertimeRequest(c.Request.Context(), store.CreateOvertimeRequestParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
		OtDate:     otDate,
		StartAt:    startAt,
		EndAt:      endAt,
		Hours:      hours,
		OtType:     otType,
		Reason:     req.Reason,
	})
	if err != nil {
		h.logger.Error("failed to create overtime request", "error", err)
		response.InternalError(c, "Failed to create overtime request")
		return
	}

	// Emit overtime.requested event for agentic workflow
	idempKey := fmt.Sprintf("overtime.requested.%d", ot.ID)
	if _, err := h.queries.InsertHREvent(c.Request.Context(), store.InsertHREventParams{
		CompanyID:      companyID,
		AggregateType:  "overtime_request",
		AggregateID:    ot.ID,
		EventType:      "overtime.requested",
		EventVersion:   1,
		Payload:        json.RawMessage(`{}`),
		ActorUserID:    &userID,
		IdempotencyKey: &idempKey,
	}); err != nil {
		h.logger.Error("failed to emit overtime.requested event", "ot_id", ot.ID, "error", err)
	}

	response.Created(c, ot)
}

func (h *Handler) ListRequests(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	pg := pagination.Parse(c)

	var employeeIDVal int64
	if eid := c.Query("employee_id"); eid != "" {
		if id, err := strconv.ParseInt(eid, 10, 64); err == nil {
			employeeIDVal = id
		}
	}
	statusFilter := c.Query("status")

	requests, err := h.queries.ListOvertimeRequests(c.Request.Context(), store.ListOvertimeRequestsParams{
		CompanyID: companyID,
		Column2:   employeeIDVal,
		Column3:   statusFilter,
		Limit:     int32(pg.Limit),
		Offset:    int32(pg.Offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list requests")
		return
	}

	count, _ := h.queries.CountOvertimeRequests(c.Request.Context(), store.CountOvertimeRequestsParams{
		CompanyID: companyID,
		Column2:   employeeIDVal,
		Column3:   statusFilter,
	})

	response.Paginated(c, requests, count, pg.Page, pg.Limit)
}

func (h *Handler) ApproveRequest(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.BadRequest(c, "Employee profile not found")
		return
	}

	ot, err := h.queries.ApproveOvertimeRequest(c.Request.Context(), store.ApproveOvertimeRequestParams{
		ID:         id,
		CompanyID:  companyID,
		ApproverID: &emp.ID,
	})
	if err != nil {
		response.NotFound(c, "Overtime request not found or already processed")
		return
	}

	// Notify employee
	if reqEmp, err := h.queries.GetEmployeeByID(c.Request.Context(), store.GetEmployeeByIDParams{ID: ot.EmployeeID, CompanyID: companyID}); err == nil && reqEmp.UserID != nil {
		entityType := "overtime_request"
		notification.Notify(c.Request.Context(), h.queries, h.logger, companyID, *reqEmp.UserID,
			"Overtime Approved",
			fmt.Sprintf("Your overtime request for %s has been approved.", ot.OtDate.Format("Jan 2, 2006")),
			"approval", &entityType, &ot.ID)
	}

	response.OK(c, ot)
}

func (h *Handler) RejectRequest(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.BadRequest(c, "Employee profile not found")
		return
	}

	ot, err := h.queries.RejectOvertimeRequest(c.Request.Context(), store.RejectOvertimeRequestParams{
		ID:              id,
		CompanyID:       companyID,
		ApproverID:      &emp.ID,
		RejectionReason: &req.Reason,
	})
	if err != nil {
		response.NotFound(c, "Overtime request not found or already processed")
		return
	}

	// Notify employee
	if reqEmp, err := h.queries.GetEmployeeByID(c.Request.Context(), store.GetEmployeeByIDParams{ID: ot.EmployeeID, CompanyID: companyID}); err == nil && reqEmp.UserID != nil {
		entityType := "overtime_request"
		msg := fmt.Sprintf("Your overtime request for %s has been rejected.", ot.OtDate.Format("Jan 2, 2006"))
		if req.Reason != "" {
			msg += " Reason: " + req.Reason
		}
		notification.Notify(c.Request.Context(), h.queries, h.logger, companyID, *reqEmp.UserID,
			"Overtime Rejected", msg, "approval", &entityType, &ot.ID)
	}

	response.OK(c, ot)
}
