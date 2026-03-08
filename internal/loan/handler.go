package loan

import (
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/email"
	"github.com/tonypk/aigonhr/internal/notification"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
	email   *email.Sender
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger, emailSender *email.Sender) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger, email: emailSender}
}

// --- Loan Types ---

func (h *Handler) ListLoanTypes(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	types, err := h.queries.ListLoanTypes(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list loan types")
		return
	}
	response.OK(c, types)
}

func (h *Handler) CreateLoanType(c *gin.Context) {
	var req struct {
		Name             string   `json:"name" binding:"required"`
		Code             string   `json:"code" binding:"required"`
		Provider         string   `json:"provider"`
		MaxTermMonths    int32    `json:"max_term_months"`
		InterestRate     float64  `json:"interest_rate"`
		MaxAmount        *float64 `json:"max_amount"`
		RequiresApproval *bool    `json:"requires_approval"`
		AutoDeduct       *bool    `json:"auto_deduct"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)

	provider := req.Provider
	if provider == "" {
		provider = "company"
	}
	maxTerm := req.MaxTermMonths
	if maxTerm == 0 {
		maxTerm = 24
	}

	var rate pgtype.Numeric
	_ = rate.Scan(strconv.FormatFloat(req.InterestRate, 'f', 4, 64))

	var maxAmt pgtype.Numeric
	if req.MaxAmount != nil {
		_ = maxAmt.Scan(strconv.FormatFloat(*req.MaxAmount, 'f', 2, 64))
	}

	requiresApproval := true
	if req.RequiresApproval != nil {
		requiresApproval = *req.RequiresApproval
	}
	autoDeduct := true
	if req.AutoDeduct != nil {
		autoDeduct = *req.AutoDeduct
	}

	lt, err := h.queries.CreateLoanType(c.Request.Context(), store.CreateLoanTypeParams{
		CompanyID:        companyID,
		Name:             req.Name,
		Code:             req.Code,
		Provider:         provider,
		MaxTermMonths:    maxTerm,
		InterestRate:     rate,
		MaxAmount:        maxAmt,
		RequiresApproval: requiresApproval,
		AutoDeduct:       autoDeduct,
	})
	if err != nil {
		h.logger.Error("failed to create loan type", "error", err)
		response.InternalError(c, "Failed to create loan type")
		return
	}
	response.Created(c, lt)
}

// --- Loans ---

func (h *Handler) ListLoans(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	loans, err := h.queries.ListLoans(c.Request.Context(), store.ListLoansParams{
		CompanyID: companyID,
		Limit:     100,
		Offset:    0,
	})
	if err != nil {
		response.InternalError(c, "Failed to list loans")
		return
	}
	response.OK(c, loans)
}

func (h *Handler) ListMyLoans(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.OK(c, []any{})
		return
	}

	loans, err := h.queries.ListMyLoans(c.Request.Context(), emp.ID)
	if err != nil {
		response.InternalError(c, "Failed to list loans")
		return
	}
	response.OK(c, loans)
}

func (h *Handler) ApplyLoan(c *gin.Context) {
	var req struct {
		LoanTypeID  int64   `json:"loan_type_id" binding:"required"`
		Amount      float64 `json:"amount" binding:"required"`
		TermMonths  int32   `json:"term_months" binding:"required"`
		ReferenceNo *string `json:"reference_no"`
		StartDate   string  `json:"start_date" binding:"required"`
		Remarks     *string `json:"remarks"`
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
		response.BadRequest(c, "Employee not found")
		return
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		response.BadRequest(c, "Invalid start date")
		return
	}
	endDate := startDate.AddDate(0, int(req.TermMonths), 0)

	// Calculate amortization (simple interest for now)
	// For government loans, interest is usually already factored into the total
	// Monthly rate * principal * term = total interest
	monthlyRate := 0.01 // default 1% if not specified from loan type
	totalInterest := req.Amount * monthlyRate * float64(req.TermMonths)
	totalAmount := req.Amount + totalInterest
	monthlyAmort := math.Ceil(totalAmount/float64(req.TermMonths)*100) / 100

	var principal, rate, amort, total, balance pgtype.Numeric
	_ = principal.Scan(strconv.FormatFloat(req.Amount, 'f', 2, 64))
	_ = rate.Scan(strconv.FormatFloat(monthlyRate, 'f', 4, 64))
	_ = amort.Scan(strconv.FormatFloat(monthlyAmort, 'f', 2, 64))
	_ = total.Scan(strconv.FormatFloat(totalAmount, 'f', 2, 64))
	_ = balance.Scan(strconv.FormatFloat(totalAmount, 'f', 2, 64))

	loan, err := h.queries.CreateLoan(c.Request.Context(), store.CreateLoanParams{
		CompanyID:           companyID,
		EmployeeID:          emp.ID,
		LoanTypeID:          req.LoanTypeID,
		ReferenceNo:         req.ReferenceNo,
		PrincipalAmount:     principal,
		InterestRate:        rate,
		TermMonths:          req.TermMonths,
		MonthlyAmortization: amort,
		TotalAmount:         total,
		RemainingBalance:    balance,
		StartDate:           startDate,
		EndDate:             endDate,
		Remarks:             req.Remarks,
	})
	if err != nil {
		h.logger.Error("failed to create loan", "error", err)
		response.InternalError(c, "Failed to create loan application")
		return
	}
	response.Created(c, loan)
}

func (h *Handler) GetLoan(c *gin.Context) {
	loanID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid loan ID")
		return
	}

	companyID := auth.GetCompanyID(c)

	loan, err := h.queries.GetLoan(c.Request.Context(), store.GetLoanParams{
		ID:        loanID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Loan not found")
		return
	}

	payments, _ := h.queries.ListLoanPayments(c.Request.Context(), loanID)

	response.OK(c, gin.H{
		"loan":     loan,
		"payments": payments,
	})
}

func (h *Handler) ApproveLoan(c *gin.Context) {
	loanID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid loan ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	emp, _ := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})

	loan, err := h.queries.ApproveLoan(c.Request.Context(), store.ApproveLoanParams{
		ID:         loanID,
		CompanyID:  companyID,
		ApprovedBy: &emp.ID,
	})
	if err != nil {
		response.NotFound(c, "Loan not found or already processed")
		return
	}

	// Auto-activate after approval
	activated, activateErr := h.queries.ActivateLoan(c.Request.Context(), store.ActivateLoanParams{
		ID:        loanID,
		CompanyID: companyID,
	})

	// Notify employee
	if reqEmp, err := h.queries.GetEmployeeByID(c.Request.Context(), store.GetEmployeeByIDParams{ID: loan.EmployeeID, CompanyID: companyID}); err == nil && reqEmp.UserID != nil {
		entityType := "loan"
		notification.Notify(c.Request.Context(), h.queries, h.logger, companyID, *reqEmp.UserID,
			"Loan Approved", "Your loan application has been approved.",
			"loan", &entityType, &loan.ID)

		// Email notification
		if reqEmp.Email != nil && *reqEmp.Email != "" {
			empName := reqEmp.FirstName + " " + reqEmp.LastName
			loanTypeName := "Loan"
			if types, err := h.queries.ListLoanTypes(c.Request.Context(), companyID); err == nil {
				for _, lt := range types {
					if lt.ID == loan.LoanTypeID {
						loanTypeName = lt.Name
						break
					}
				}
			}
			amount := "N/A"
			if f, err := loan.PrincipalAmount.Float64Value(); err == nil && f.Valid {
				amount = fmt.Sprintf("₱%.2f", f.Float64)
			}
			subj, body := email.LoanApprovedEmail(empName, loanTypeName, amount)
			h.email.SendAsync(*reqEmp.Email, subj, body)
		}
	}

	if activateErr == nil {
		response.OK(c, activated)
		return
	}

	response.OK(c, loan)
}

func (h *Handler) CancelLoan(c *gin.Context) {
	loanID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid loan ID")
		return
	}

	companyID := auth.GetCompanyID(c)

	loan, err := h.queries.CancelLoan(c.Request.Context(), store.CancelLoanParams{
		ID:        loanID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Loan not found or cannot be cancelled")
		return
	}
	response.OK(c, loan)
}

func (h *Handler) RecordPayment(c *gin.Context) {
	loanID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid loan ID")
		return
	}

	var req struct {
		Amount      float64 `json:"amount" binding:"required"`
		PaymentDate string  `json:"payment_date" binding:"required"`
		PaymentType string  `json:"payment_type"`
		Remarks     *string `json:"remarks"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)

	payDate, err := time.Parse("2006-01-02", req.PaymentDate)
	if err != nil {
		response.BadRequest(c, "Invalid payment date")
		return
	}

	payType := req.PaymentType
	if payType == "" {
		payType = "manual"
	}

	var amount, principalPortion, interestPortion pgtype.Numeric
	_ = amount.Scan(strconv.FormatFloat(req.Amount, 'f', 2, 64))
	_ = principalPortion.Scan(strconv.FormatFloat(req.Amount*0.9, 'f', 2, 64)) // approximate split
	_ = interestPortion.Scan(strconv.FormatFloat(req.Amount*0.1, 'f', 2, 64))

	payment, err := h.queries.CreateLoanPayment(c.Request.Context(), store.CreateLoanPaymentParams{
		LoanID:           loanID,
		PaymentDate:      payDate,
		Amount:           amount,
		PrincipalPortion: principalPortion,
		InterestPortion:  interestPortion,
		PaymentType:      payType,
		Remarks:          req.Remarks,
	})
	if err != nil {
		response.InternalError(c, "Failed to record payment")
		return
	}

	// Update loan balance
	var amtNum pgtype.Numeric
	_ = amtNum.Scan(strconv.FormatFloat(req.Amount, 'f', 2, 64))
	_, _ = h.queries.UpdateLoanBalance(c.Request.Context(), store.UpdateLoanBalanceParams{
		ID:        loanID,
		CompanyID: companyID,
		TotalPaid: amtNum,
	})

	response.Created(c, payment)
}
