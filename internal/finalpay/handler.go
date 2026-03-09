package finalpay

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

// toNum converts a float64 to pgtype.Numeric with 2 decimal places.
func toNum(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%.2f", v))
	return n
}

// List returns paginated final pay records.
func (h *Handler) List(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	items, err := h.queries.ListFinalPays(c.Request.Context(), store.ListFinalPaysParams{
		CompanyID: companyID,
		Limit:     int32(limit),
		Offset:    int32((page - 1) * limit),
	})
	if err != nil {
		response.InternalError(c, "Failed to list final pays")
		return
	}
	response.OK(c, items)
}

// Get returns the final pay record for a specific employee.
func (h *Handler) Get(c *gin.Context) {
	empID, err := strconv.ParseInt(c.Param("employee_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	companyID := auth.GetCompanyID(c)
	fp, err := h.queries.GetFinalPay(c.Request.Context(), store.GetFinalPayParams{
		CompanyID:  companyID,
		EmployeeID: empID,
	})
	if err != nil {
		response.NotFound(c, "No final pay record found")
		return
	}
	response.OK(c, fp)
}

// Create creates a new final pay record.
func (h *Handler) Create(c *gin.Context) {
	var req struct {
		EmployeeID            int64   `json:"employee_id" binding:"required"`
		SeparationDate        string  `json:"separation_date" binding:"required"`
		SeparationReason      string  `json:"separation_reason" binding:"required"`
		UnpaidSalary          float64 `json:"unpaid_salary"`
		Prorated13th          float64 `json:"prorated_13th"`
		UnusedLeaveConversion float64 `json:"unused_leave_conversion"`
		SeparationPay         float64 `json:"separation_pay"`
		TaxRefund             float64 `json:"tax_refund"`
		OtherDeductions       float64 `json:"other_deductions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	companyID := auth.GetCompanyID(c)
	sepDate, _ := time.Parse("2006-01-02", req.SeparationDate)

	total := req.UnpaidSalary + req.Prorated13th + req.UnusedLeaveConversion +
		req.SeparationPay + req.TaxRefund - req.OtherDeductions

	fp, err := h.queries.CreateFinalPay(c.Request.Context(), store.CreateFinalPayParams{
		CompanyID:             companyID,
		EmployeeID:            req.EmployeeID,
		SeparationDate:        sepDate,
		SeparationReason:      req.SeparationReason,
		UnpaidSalary:          toNum(req.UnpaidSalary),
		Prorated13th:          toNum(req.Prorated13th),
		UnusedLeaveConversion: toNum(req.UnusedLeaveConversion),
		SeparationPay:         toNum(req.SeparationPay),
		TaxRefund:             toNum(req.TaxRefund),
		OtherDeductions:       toNum(req.OtherDeductions),
		TotalFinalPay:         toNum(total),
		Payload:               []byte("{}"),
	})
	if err != nil {
		h.logger.Error("failed to create final pay", "error", err)
		response.InternalError(c, "Failed to create final pay")
		return
	}
	response.Created(c, fp)
}

// UpdateStatus updates the status of a final pay record.
func (h *Handler) UpdateStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	companyID := auth.GetCompanyID(c)
	fp, err := h.queries.UpdateFinalPayStatus(c.Request.Context(), store.UpdateFinalPayStatusParams{
		ID: id, CompanyID: companyID, Status: req.Status,
	})
	if err != nil {
		response.InternalError(c, "Failed to update final pay status")
		return
	}
	response.OK(c, fp)
}
