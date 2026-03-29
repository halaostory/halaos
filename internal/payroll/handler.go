package payroll

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-pdf/fpdf"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/notification"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/pagination"
	"github.com/halaostory/halaos/pkg/response"
)

// AccountingEventEmitter is called after payroll approval to enqueue accounting events.
type AccountingEventEmitter interface {
	EmitPayrollApproved(ctx context.Context, companyID, cycleID int64) error
}

type Handler struct {
	queries    *store.Queries
	pool       *pgxpool.Pool
	logger     *slog.Logger
	accounting AccountingEventEmitter // nil if not configured
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger}
}

// SetAccountingEmitter configures the accounting event emitter (optional).
func (h *Handler) SetAccountingEmitter(emitter AccountingEventEmitter) {
	h.accounting = emitter
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

	periodStart, err := time.Parse("2006-01-02", req.PeriodStart)
	if err != nil {
		response.BadRequest(c, "Invalid period_start date format (expected YYYY-MM-DD)")
		return
	}
	periodEnd, err := time.Parse("2006-01-02", req.PeriodEnd)
	if err != nil {
		response.BadRequest(c, "Invalid period_end date format (expected YYYY-MM-DD)")
		return
	}
	payDate, err := time.Parse("2006-01-02", req.PayDate)
	if err != nil {
		response.BadRequest(c, "Invalid pay_date date format (expected YYYY-MM-DD)")
		return
	}
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

	// Check if cycle is locked
	locked, err := h.queries.IsPayrollCycleLocked(c.Request.Context(), store.IsPayrollCycleLockedParams{
		ID: req.CycleID, CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Payroll cycle not found")
		return
	}
	if locked {
		response.BadRequest(c, "Payroll cycle is locked and cannot be modified")
		return
	}

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

	// Dispatch to async worker via event outbox
	_, err = h.queries.InsertHREvent(c.Request.Context(), store.InsertHREventParams{
		CompanyID:     companyID,
		AggregateType: "payroll_run",
		AggregateID:   run.ID,
		EventType:     "payroll.run_requested",
		EventVersion:  1,
		Payload: func() json.RawMessage {
			b, _ := json.Marshal(map[string]any{"run_id": run.ID, "company_id": companyID, "run_type": runType})
			return b
		}(),
		ActorUserID:   &userID,
	})
	if err != nil {
		h.logger.Error("failed to enqueue payroll event", "run_id", run.ID, "error", err)
	}

	response.Created(c, run)
}

func (h *Handler) ListPayrollItems(c *gin.Context) {
	runID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid run ID")
		return
	}

	companyID := auth.GetCompanyID(c)

	// Verify the run belongs to this company
	run, err := h.queries.GetPayrollRun(c.Request.Context(), store.GetPayrollRunParams{
		ID: runID, CompanyID: companyID,
	})
	if err != nil || run.ID == 0 {
		response.NotFound(c, "Payroll run not found")
		return
	}

	items, err := h.queries.ListPayrollItems(c.Request.Context(), runID)
	if err != nil {
		response.InternalError(c, "Failed to list payroll items")
		return
	}
	response.OK(c, items)
}

func (h *Handler) ApproveCycle(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	// Check if cycle is locked
	locked, err := h.queries.IsPayrollCycleLocked(c.Request.Context(), store.IsPayrollCycleLockedParams{
		ID: id, CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Payroll cycle not found")
		return
	}
	if locked {
		response.BadRequest(c, "Payroll cycle is locked and cannot be modified")
		return
	}

	err = h.queries.ApprovePayrollCycle(c.Request.Context(), store.ApprovePayrollCycleParams{
		ID:         id,
		CompanyID:  companyID,
		ApprovedBy: &userID,
	})
	if err != nil {
		response.NotFound(c, "Payroll cycle not found")
		return
	}

	// Notify all employees that payslips are available
	go func() {
		ctx := context.Background()
		cycle, err := h.queries.GetPayrollCycle(ctx, store.GetPayrollCycleParams{ID: id, CompanyID: companyID})
		if err != nil {
			return
		}
		emps, _ := h.queries.ListActiveEmployees(ctx, companyID)
		entityType := "payroll"
		for _, e := range emps {
			if e.UserID != nil {
				notification.Notify(ctx, h.queries, h.logger, companyID, *e.UserID,
					"Payslip Available",
					fmt.Sprintf("Your payslip for %s is now available.", cycle.Name),
					"payroll", &entityType, &id)
			}
		}
	}()

	// Emit accounting event (async, non-blocking)
	if h.accounting != nil {
		go func() {
			if err := h.accounting.EmitPayrollApproved(context.Background(), companyID, id); err != nil {
				h.logger.Error("failed to emit accounting event", "cycle_id", id, "error", err)
			}
		}()
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
		// Admin/manager users may not have an employee record — return empty list
		response.OK(c, []any{})
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
	payslipID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid payslip ID")
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

	payslip, err := h.queries.GetPayslip(c.Request.Context(), store.GetPayslipParams{
		ID:         payslipID,
		EmployeeID: emp.ID,
	})
	if err != nil {
		response.NotFound(c, "Payslip not found")
		return
	}
	response.OK(c, payslip)
}

func (h *Handler) DownloadPayslipPDF(c *gin.Context) {
	payslipID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid payslip ID")
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

	payslip, err := h.queries.GetPayslip(c.Request.Context(), store.GetPayslipParams{
		ID:         payslipID,
		EmployeeID: emp.ID,
	})
	if err != nil {
		response.NotFound(c, "Payslip not found")
		return
	}

	company, err := h.queries.GetCompanyByID(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get company info")
		return
	}

	pdfBytes, err := generatePayslipPDF(company, emp, payslip)
	if err != nil {
		h.logger.Error("failed to generate payslip PDF", "error", err)
		response.InternalError(c, "Failed to generate PDF")
		return
	}

	fileName := fmt.Sprintf("payslip_%s_%s.pdf", emp.EmployeeNo, payslip.PayDate.Format("2006-01-02"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Data(200, "application/pdf", pdfBytes)
}

func generatePayslipPDF(company store.Company, emp store.Employee, payslip store.Payslip) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetMargins(15, 15, 15)

	// Company header
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(180, 10, company.Name, "", 1, "C", false, 0, "")

	if company.Address != nil {
		pdf.SetFont("Arial", "", 9)
		addr := *company.Address
		if company.City != nil {
			addr += ", " + *company.City
		}
		if company.Province != nil {
			addr += ", " + *company.Province
		}
		pdf.CellFormat(180, 5, addr, "", 1, "C", false, 0, "")
	}

	if company.Tin != nil {
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(180, 5, "TIN: "+*company.Tin, "", 1, "C", false, 0, "")
	}

	pdf.Ln(5)
	pdf.SetFont("Arial", "B", 13)
	pdf.CellFormat(180, 8, "PAYSLIP", "", 1, "C", false, 0, "")
	pdf.Ln(3)

	// Period info
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(90, 6, "Period: "+payslip.PeriodStart.Format("Jan 02, 2006")+" - "+payslip.PeriodEnd.Format("Jan 02, 2006"), "", 0, "L", false, 0, "")
	pdf.CellFormat(90, 6, "Pay Date: "+payslip.PayDate.Format("Jan 02, 2006"), "", 1, "R", false, 0, "")

	// Employee info
	empName := emp.FirstName + " " + emp.LastName
	pdf.CellFormat(90, 6, "Employee: "+empName, "", 0, "L", false, 0, "")
	pdf.CellFormat(90, 6, "Employee No: "+emp.EmployeeNo, "", 1, "R", false, 0, "")

	pdf.Ln(5)

	// Parse payload
	var payload map[string]interface{}
	if len(payslip.Payload) > 0 {
		_ = json.Unmarshal(payslip.Payload, &payload)
	}

	// Table header
	pdf.SetFillColor(240, 240, 240)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(120, 7, "Description", "1", 0, "L", true, 0, "")
	pdf.CellFormat(60, 7, "Amount (PHP)", "1", 1, "R", true, 0, "")

	pdf.SetFont("Arial", "", 10)

	// Earnings section
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(180, 7, "EARNINGS", "LR", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)

	addRow := func(label string, amount interface{}) {
		pdf.CellFormat(120, 6, "  "+label, "LR", 0, "L", false, 0, "")
		pdf.CellFormat(60, 6, formatAmount(amount), "LR", 1, "R", false, 0, "")
	}

	if payload != nil {
		if basic, ok := payload["basic_pay"]; ok {
			addRow("Basic Pay", basic)
		}
		if ot, ok := payload["overtime_pay"]; ok {
			addRow("Overtime Pay", ot)
		}
		if hp, ok := payload["holiday_pay"]; ok {
			addRow("Holiday Pay", hp)
		}
		if nd, ok := payload["night_diff"]; ok {
			addRow("Night Differential", nd)
		}
		if allowances, ok := payload["allowances"].([]interface{}); ok {
			for _, a := range allowances {
				if m, ok := a.(map[string]interface{}); ok {
					addRow(fmt.Sprintf("%v", m["name"]), m["amount"])
				}
			}
		}
	}

	if gross, ok := payload["gross_pay"]; ok {
		pdf.SetFont("Arial", "B", 10)
		pdf.CellFormat(120, 7, "  Gross Pay", "1", 0, "L", true, 0, "")
		pdf.CellFormat(60, 7, formatAmount(gross), "1", 1, "R", true, 0, "")
		pdf.SetFont("Arial", "", 10)
	}

	// Deductions section
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(180, 7, "DEDUCTIONS", "LR", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)

	if payload != nil {
		if v, ok := payload["sss_ee"]; ok {
			addRow("SSS (Employee)", v)
		}
		if v, ok := payload["philhealth_ee"]; ok {
			addRow("PhilHealth (Employee)", v)
		}
		if v, ok := payload["pagibig_ee"]; ok {
			addRow("Pag-IBIG (Employee)", v)
		}
		if v, ok := payload["withholding_tax"]; ok {
			addRow("Withholding Tax", v)
		}
		if v, ok := payload["late_deduction"]; ok {
			addRow("Late Deduction", v)
		}
		if v, ok := payload["undertime_deduction"]; ok {
			addRow("Undertime Deduction", v)
		}
		if deductions, ok := payload["other_deductions"].([]interface{}); ok {
			for _, d := range deductions {
				if m, ok := d.(map[string]interface{}); ok {
					addRow(fmt.Sprintf("%v", m["name"]), m["amount"])
				}
			}
		}
	}

	if totalDed, ok := payload["total_deductions"]; ok {
		pdf.SetFont("Arial", "B", 10)
		pdf.CellFormat(120, 7, "  Total Deductions", "1", 0, "L", true, 0, "")
		pdf.CellFormat(60, 7, formatAmount(totalDed), "1", 1, "R", true, 0, "")
	}

	// Net Pay
	pdf.Ln(3)
	pdf.SetFont("Arial", "B", 12)
	pdf.SetFillColor(24, 160, 88)
	pdf.SetTextColor(255, 255, 255)
	if netPay, ok := payload["net_pay"]; ok {
		pdf.CellFormat(120, 9, "  NET PAY", "1", 0, "L", true, 0, "")
		pdf.CellFormat(60, 9, formatAmount(netPay), "1", 1, "R", true, 0, "")
	}
	pdf.SetTextColor(0, 0, 0)

	// Footer
	pdf.Ln(15)
	pdf.SetFont("Arial", "I", 8)
	pdf.CellFormat(180, 5, "This is a system-generated payslip. No signature is required.", "", 1, "C", false, 0, "")
	pdf.CellFormat(180, 5, "Generated on: "+time.Now().Format("January 02, 2006 3:04 PM"), "", 1, "C", false, 0, "")

	// Branding footer
	_, pageH := pdf.GetPageSize()
	pdf.SetY(pageH - 10)
	pdf.SetFont("Arial", "", 7)
	pdf.SetTextColor(160, 160, 160)
	pdf.CellFormat(0, 4, "Powered by HalaOS | halaos.com", "", 0, "C", false, 0, "https://halaos.com")
	pdf.SetTextColor(0, 0, 0)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func formatAmount(v interface{}) string {
	switch val := v.(type) {
	case float64:
		return fmt.Sprintf("%.2f", val)
	case string:
		return val
	case json.Number:
		f, err := val.Float64()
		if err == nil {
			return fmt.Sprintf("%.2f", f)
		}
		return string(val)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// === Auto-Payroll Config ===

func (h *Handler) GetAutoConfig(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	cfg, err := h.queries.GetPayrollAutoConfig(c.Request.Context(), companyID)
	if err != nil {
		// Return defaults if no config exists
		response.OK(c, gin.H{
			"auto_run_enabled":        false,
			"days_before_pay":         2,
			"auto_approve_enabled":    false,
			"max_auto_approve_amount": 0,
			"notify_on_auto":          true,
		})
		return
	}
	response.OK(c, cfg)
}

func (h *Handler) UpdateAutoConfig(c *gin.Context) {
	var req struct {
		AutoRunEnabled       bool    `json:"auto_run_enabled"`
		DaysBeforePay        int32   `json:"days_before_pay"`
		AutoApproveEnabled   bool    `json:"auto_approve_enabled"`
		MaxAutoApproveAmount float64 `json:"max_auto_approve_amount"`
		NotifyOnAuto         bool    `json:"notify_on_auto"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)

	if req.DaysBeforePay < 1 {
		req.DaysBeforePay = 2
	}
	if req.DaysBeforePay > 14 {
		req.DaysBeforePay = 14
	}

	// Convert float64 to pgtype.Numeric
	var maxAmount pgtype.Numeric
	maxAmount.Valid = true
	maxAmount.Int = big.NewInt(int64(req.MaxAutoApproveAmount * 100))
	maxAmount.Exp = -2

	cfg, err := h.queries.UpsertPayrollAutoConfig(c.Request.Context(), store.UpsertPayrollAutoConfigParams{
		CompanyID:            companyID,
		AutoRunEnabled:       req.AutoRunEnabled,
		DaysBeforePay:        req.DaysBeforePay,
		AutoApproveEnabled:   req.AutoApproveEnabled,
		MaxAutoApproveAmount: maxAmount,
		NotifyOnAuto:         req.NotifyOnAuto,
	})
	if err != nil {
		response.InternalError(c, "Failed to update auto-payroll config")
		return
	}
	response.OK(c, cfg)
}

func (h *Handler) ListAutoLogs(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	pg := pagination.Parse(c)

	logs, err := h.queries.ListPayrollAutoLogs(c.Request.Context(), store.ListPayrollAutoLogsParams{
		CompanyID: companyID,
		Limit:     int32(pg.Limit),
		Offset:    int32(pg.Offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list auto-payroll logs")
		return
	}
	response.OK(c, logs)
}
