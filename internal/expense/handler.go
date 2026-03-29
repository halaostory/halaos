package expense

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/auth"
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

func (h *Handler) ListCategories(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	cats, err := h.queries.ListExpenseCategories(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list expense categories")
		return
	}
	response.OK(c, cats)
}

func (h *Handler) CreateCategory(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	var req struct {
		Name            string  `json:"name" binding:"required"`
		Description     *string `json:"description"`
		MaxAmount       float64 `json:"max_amount"`
		RequiresReceipt bool    `json:"requires_receipt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	var maxAmt pgtype.Numeric
	if req.MaxAmount > 0 {
		_ = maxAmt.Scan(fmt.Sprintf("%.2f", req.MaxAmount))
	}
	cat, err := h.queries.CreateExpenseCategory(c.Request.Context(), store.CreateExpenseCategoryParams{
		CompanyID:       companyID,
		Name:            req.Name,
		Description:     req.Description,
		MaxAmount:       maxAmt,
		RequiresReceipt: req.RequiresReceipt,
	})
	if err != nil {
		response.InternalError(c, "Failed to create expense category")
		return
	}
	response.Created(c, cat)
}

func (h *Handler) UpdateCategory(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid category ID")
		return
	}
	var req struct {
		Name            string  `json:"name"`
		Description     *string `json:"description"`
		MaxAmount       float64 `json:"max_amount"`
		RequiresReceipt bool    `json:"requires_receipt"`
		IsActive        bool    `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	var maxAmt pgtype.Numeric
	if req.MaxAmount > 0 {
		_ = maxAmt.Scan(fmt.Sprintf("%.2f", req.MaxAmount))
	}
	cat, err := h.queries.UpdateExpenseCategory(c.Request.Context(), store.UpdateExpenseCategoryParams{
		ID:              id,
		CompanyID:       companyID,
		Name:            req.Name,
		Description:     req.Description,
		MaxAmount:       maxAmt,
		RequiresReceipt: req.RequiresReceipt,
		IsActive:        req.IsActive,
	})
	if err != nil {
		response.InternalError(c, "Failed to update expense category")
		return
	}
	response.OK(c, cat)
}

func (h *Handler) GetSummary(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	summary, err := h.queries.GetExpenseSummary(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get expense summary")
		return
	}
	response.OK(c, summary)
}

func (h *Handler) List(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	pg := pagination.Parse(c)
	statusFilter := c.DefaultQuery("status", "")
	var employeeIDFilter int64
	if eid := c.Query("employee_id"); eid != "" {
		employeeIDFilter, _ = strconv.ParseInt(eid, 10, 64)
	}
	claims, err := h.queries.ListExpenseClaims(c.Request.Context(), store.ListExpenseClaimsParams{
		CompanyID:  companyID,
		Status:     statusFilter,
		EmployeeID: employeeIDFilter,
		Lim:        int32(pg.Limit),
		Off:        int32(pg.Offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list expense claims")
		return
	}
	count, _ := h.queries.CountExpenseClaims(c.Request.Context(), store.CountExpenseClaimsParams{
		CompanyID:  companyID,
		Status:     statusFilter,
		EmployeeID: employeeIDFilter,
	})
	response.Paginated(c, claims, count, pg.Page, pg.Limit)
}

func (h *Handler) ListMy(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		CompanyID: companyID,
		UserID:    &userID,
	})
	if err != nil {
		response.OK(c, []any{})
		return
	}
	claims, err := h.queries.ListMyExpenseClaims(c.Request.Context(), store.ListMyExpenseClaimsParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
	})
	if err != nil {
		response.InternalError(c, "Failed to list expense claims")
		return
	}
	response.OK(c, claims)
}

func (h *Handler) Get(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid expense claim ID")
		return
	}
	claim, err := h.queries.GetExpenseClaim(c.Request.Context(), store.GetExpenseClaimParams{
		ID:        id,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Expense claim not found")
		return
	}
	response.OK(c, claim)
}

func (h *Handler) Create(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		CompanyID: companyID,
		UserID:    &userID,
	})
	if err != nil {
		response.BadRequest(c, "Employee profile not found")
		return
	}
	var req struct {
		CategoryID  int64   `json:"category_id" binding:"required"`
		Description string  `json:"description" binding:"required"`
		Amount      float64 `json:"amount" binding:"required"`
		Currency    string  `json:"currency"`
		ExpenseDate string  `json:"expense_date" binding:"required"`
		ReceiptPath *string `json:"receipt_path"`
		Notes       *string `json:"notes"`
		Submit      bool    `json:"submit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	expDate, err := time.Parse("2006-01-02", req.ExpenseDate)
	if err != nil {
		response.BadRequest(c, "Invalid expense_date format")
		return
	}
	nextNum, _ := h.queries.NextExpenseClaimNumber(c.Request.Context(), companyID)
	claimNumber := fmt.Sprintf("EXP-%06d", nextNum)
	currency := req.Currency
	if currency == "" {
		currency = "PHP"
	}
	status := "draft"
	if req.Submit {
		status = "submitted"
	}
	var amount pgtype.Numeric
	_ = amount.Scan(fmt.Sprintf("%.2f", req.Amount))
	claim, err := h.queries.CreateExpenseClaim(c.Request.Context(), store.CreateExpenseClaimParams{
		CompanyID:   companyID,
		EmployeeID:  emp.ID,
		ClaimNumber: claimNumber,
		CategoryID:  req.CategoryID,
		Description: req.Description,
		Amount:      amount,
		Currency:    currency,
		ExpenseDate: expDate,
		ReceiptPath: req.ReceiptPath,
		Status:      status,
		Notes:       req.Notes,
	})
	if err != nil {
		h.logger.Error("failed to create expense claim", "error", err)
		response.InternalError(c, "Failed to create expense claim")
		return
	}
	response.Created(c, claim)
}

func (h *Handler) Submit(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid expense claim ID")
		return
	}
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		CompanyID: companyID,
		UserID:    &userID,
	})
	if err != nil {
		response.BadRequest(c, "Employee profile not found")
		return
	}
	claim, err := h.queries.SubmitExpenseClaim(c.Request.Context(), store.SubmitExpenseClaimParams{
		ID:         id,
		CompanyID:  companyID,
		EmployeeID: emp.ID,
	})
	if err != nil {
		response.BadRequest(c, "Failed to submit expense claim")
		return
	}
	response.OK(c, claim)
}

func (h *Handler) Approve(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid expense claim ID")
		return
	}
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		CompanyID: companyID,
		UserID:    &userID,
	})
	if err != nil {
		response.BadRequest(c, "Employee profile not found")
		return
	}
	claim, err := h.queries.ApproveExpenseClaim(c.Request.Context(), store.ApproveExpenseClaimParams{
		ID:         id,
		CompanyID:  companyID,
		ApproverID: &emp.ID,
	})
	if err != nil {
		response.BadRequest(c, "Failed to approve expense claim")
		return
	}
	response.OK(c, claim)
}

func (h *Handler) Reject(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid expense claim ID")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		CompanyID: companyID,
		UserID:    &userID,
	})
	if err != nil {
		response.BadRequest(c, "Employee profile not found")
		return
	}
	claim, err := h.queries.RejectExpenseClaim(c.Request.Context(), store.RejectExpenseClaimParams{
		ID:              id,
		CompanyID:       companyID,
		ApproverID:      &emp.ID,
		RejectionReason: &req.Reason,
	})
	if err != nil {
		response.BadRequest(c, "Failed to reject expense claim")
		return
	}
	response.OK(c, claim)
}

func (h *Handler) MarkPaid(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid expense claim ID")
		return
	}
	var req struct {
		Reference string `json:"reference"`
	}
	_ = c.ShouldBindJSON(&req)
	claim, err := h.queries.MarkExpenseClaimPaid(c.Request.Context(), store.MarkExpenseClaimPaidParams{
		ID:            id,
		CompanyID:     companyID,
		PaidReference: &req.Reference,
	})
	if err != nil {
		response.BadRequest(c, "Failed to mark expense as paid")
		return
	}
	response.OK(c, claim)
}
