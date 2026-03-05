package payroll

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/pagination"
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

func (h *Handler) ListCycles(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	pg := pagination.Parse(c)

	cycles, err := h.queries.ListPayrollCycles(c.Request.Context(), store.ListPayrollCyclesParams{
		CompanyID: companyID,
		Limit:     int32(pg.Limit),
		Offset:    int32(pg.Offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list payroll cycles")
		return
	}
	response.OK(c, cycles)
}

func (h *Handler) CreateCycle(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		PeriodStart string `json:"period_start" binding:"required"`
		PeriodEnd   string `json:"period_end" binding:"required"`
		PayDate     string `json:"pay_date" binding:"required"`
		CycleType   string `json:"cycle_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	periodStart, _ := time.Parse("2006-01-02", req.PeriodStart)
	periodEnd, _ := time.Parse("2006-01-02", req.PeriodEnd)
	payDate, _ := time.Parse("2006-01-02", req.PayDate)
	cycleType := req.CycleType
	if cycleType == "" {
		cycleType = "regular"
	}

	cycle, err := h.queries.CreatePayrollCycle(c.Request.Context(), store.CreatePayrollCycleParams{
		CompanyID:   companyID,
		Name:        req.Name,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		PayDate:     payDate,
		CycleType:   cycleType,
		CreatedBy:   &userID,
	})
	if err != nil {
		response.InternalError(c, "Failed to create payroll cycle")
		return
	}
	response.Created(c, cycle)
}

func (h *Handler) RunPayroll(c *gin.Context) {
	var req struct {
		CycleID int64  `json:"cycle_id" binding:"required"`
		RunType string `json:"run_type"` // regular, simulation
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	runType := req.RunType
	if runType == "" {
		runType = "regular"
	}

	run, err := h.queries.CreatePayrollRun(c.Request.Context(), store.CreatePayrollRunParams{
		CompanyID:   companyID,
		CycleID:     req.CycleID,
		RunType:     runType,
		InitiatedBy: &userID,
	})
	if err != nil {
		response.InternalError(c, "Failed to create payroll run")
		return
	}

	// TODO: Trigger async payroll calculation via event/worker
	// For now, mark as pending
	response.Created(c, run)
}

func (h *Handler) ListPayrollItems(c *gin.Context) {
	runID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	items, err := h.queries.ListPayrollItems(c.Request.Context(), runID)
	if err != nil {
		response.InternalError(c, "Failed to list payroll items")
		return
	}
	response.OK(c, items)
}

func (h *Handler) ApproveCycle(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	err := h.queries.ApprovePayrollCycle(c.Request.Context(), store.ApprovePayrollCycleParams{
		ID:         id,
		CompanyID:  companyID,
		ApprovedBy: &userID,
	})
	if err != nil {
		response.NotFound(c, "Payroll cycle not found")
		return
	}
	response.OK(c, gin.H{"message": "Payroll cycle approved"})
}

func (h *Handler) ListPayslips(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	pg := pagination.Parse(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Employee not found")
		return
	}

	payslips, err := h.queries.ListPayslips(c.Request.Context(), store.ListPayslipsParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
		Limit:      int32(pg.Limit),
		Offset:     int32(pg.Offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list payslips")
		return
	}
	response.OK(c, payslips)
}

func (h *Handler) GetPayslip(c *gin.Context) {
	// TODO: parse UUID param
	response.OK(c, gin.H{"message": "payslip detail placeholder"})
}
