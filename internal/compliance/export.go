package compliance

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/pkg/response"
)

func numStr(n pgtype.Numeric) string {
	if !n.Valid {
		return "0.00"
	}
	f, _ := n.Float64Value()
	if !f.Valid {
		return "0.00"
	}
	return fmt.Sprintf("%.2f", f.Float64)
}

// ExportPayrollCSV exports payroll items for a cycle as CSV.
func (h *Handler) ExportPayrollCSV(c *gin.Context) {
	cycleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid cycle ID")
		return
	}

	// Find latest run for this cycle
	var runID int64
	row := h.pool.QueryRow(c.Request.Context(),
		"SELECT id FROM payroll_runs WHERE cycle_id = $1 AND status = 'completed' ORDER BY created_at DESC LIMIT 1", cycleID)
	if err := row.Scan(&runID); err != nil {
		response.NotFound(c, "No completed payroll run found for this cycle")
		return
	}

	items, err := h.queries.ListPayrollItems(c.Request.Context(), runID)
	if err != nil {
		response.InternalError(c, "Failed to list payroll items")
		return
	}

	filename := fmt.Sprintf("payroll_cycle_%d_%s.csv", cycleID, time.Now().Format("20060102"))
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	w := csv.NewWriter(c.Writer)
	defer w.Flush()

	// Header
	_ = w.Write([]string{
		"Employee No", "First Name", "Last Name",
		"Basic Pay", "Gross Pay", "Taxable Income",
		"SSS EE", "SSS ER", "PhilHealth EE", "PhilHealth ER",
		"PagIBIG EE", "PagIBIG ER", "Withholding Tax",
		"Total Deductions", "Net Pay",
		"Work Days", "OT Hours",
	})

	for _, item := range items {
		_ = w.Write([]string{
			item.EmployeeNo, item.FirstName, item.LastName,
			numStr(item.BasicPay), numStr(item.GrossPay), numStr(item.TaxableIncome),
			numStr(item.SssEe), numStr(item.SssEr), numStr(item.PhilhealthEe), numStr(item.PhilhealthEr),
			numStr(item.PagibigEe), numStr(item.PagibigEr), numStr(item.WithholdingTax),
			numStr(item.TotalDeductions), numStr(item.NetPay),
			numStr(item.WorkDays), numStr(item.OtHours),
		})
	}

	c.Status(http.StatusOK)
}

// ExportEmployeesCSV exports employee list as CSV.
func (h *Handler) ExportEmployeesCSV(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	employees, err := h.queries.ListActiveEmployees(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list employees")
		return
	}

	filename := fmt.Sprintf("employees_%s.csv", time.Now().Format("20060102"))
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	w := csv.NewWriter(c.Writer)
	defer w.Flush()

	_ = w.Write([]string{
		"Employee No", "First Name", "Last Name", "Middle Name",
		"Email", "Phone", "Gender", "Birth Date",
		"Hire Date", "Employment Type", "Status",
	})

	for _, emp := range employees {
		_ = w.Write([]string{
			emp.EmployeeNo, emp.FirstName, emp.LastName,
			ptrOr(emp.MiddleName, ""),
			ptrOr(emp.Email, ""), ptrOr(emp.Phone, ""),
			ptrOr(emp.Gender, ""),
			formatDate(emp.BirthDate),
			emp.HireDate.Format("2006-01-02"),
			emp.EmploymentType, emp.Status,
		})
	}

	c.Status(http.StatusOK)
}

// ExportPayrollBankFile exports payroll data as a bank file (CSV format compatible
// with common Philippine banks: UnionBank, BDO, Landbank, BPI, Metrobank).
func (h *Handler) ExportPayrollBankFile(c *gin.Context) {
	cycleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid cycle ID")
		return
	}

	bankFormat := c.DefaultQuery("format", "generic")

	var runID int64
	row := h.pool.QueryRow(c.Request.Context(),
		"SELECT id FROM payroll_runs WHERE cycle_id = $1 AND status = 'completed' ORDER BY created_at DESC LIMIT 1", cycleID)
	if err := row.Scan(&runID); err != nil {
		response.NotFound(c, "No completed payroll run found for this cycle")
		return
	}

	items, err := h.queries.ListPayrollItemsWithBank(c.Request.Context(), runID)
	if err != nil {
		response.InternalError(c, "Failed to list payroll items")
		return
	}

	filename := fmt.Sprintf("bank_file_%s_cycle_%d_%s.csv", bankFormat, cycleID, time.Now().Format("20060102"))
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	w := csv.NewWriter(c.Writer)
	defer w.Flush()

	switch bankFormat {
	case "unionbank":
		_ = w.Write([]string{"Account Number", "Account Name", "Amount", "Remarks"})
		for _, item := range items {
			_ = w.Write([]string{
				ptrOr(item.BankAccountNo, ""),
				ptrOr(item.BankAccountName, item.LastName+", "+item.FirstName),
				numStr(item.NetPay),
				"Salary " + item.EmployeeNo,
			})
		}
	case "bdo":
		_ = w.Write([]string{"Account Number", "Amount", "Employee Name", "Employee No"})
		for _, item := range items {
			_ = w.Write([]string{
				ptrOr(item.BankAccountNo, ""),
				numStr(item.NetPay),
				item.LastName + ", " + item.FirstName,
				item.EmployeeNo,
			})
		}
	case "landbank":
		_ = w.Write([]string{"Account No", "Employee Name", "Net Pay"})
		for _, item := range items {
			_ = w.Write([]string{
				ptrOr(item.BankAccountNo, ""),
				item.LastName + ", " + item.FirstName,
				numStr(item.NetPay),
			})
		}
	default: // generic
		_ = w.Write([]string{
			"Employee No", "Last Name", "First Name",
			"Bank Name", "Account No", "Account Name",
			"Net Pay",
		})
		for _, item := range items {
			_ = w.Write([]string{
				item.EmployeeNo, item.LastName, item.FirstName,
				ptrOr(item.BankName, ""),
				ptrOr(item.BankAccountNo, ""),
				ptrOr(item.BankAccountName, ""),
				numStr(item.NetPay),
			})
		}
	}

	c.Status(http.StatusOK)
}

func formatDate(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format("2006-01-02")
}

// GenerateFormHandler handles form generation API requests.
func (h *Handler) GenerateFormHandler(c *gin.Context) {
	var req struct {
		FormType string `json:"form_type" binding:"required"`
		TaxYear  int32  `json:"tax_year" binding:"required"`
		Month    int    `json:"month"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	fg := NewFormGenerator(h.queries)

	form, err := fg.GenerateAndStore(c.Request.Context(), companyID, req.FormType, req.TaxYear, req.Month)
	if err != nil {
		h.logger.Error("failed to generate form", "form_type", req.FormType, "error", err)
		response.InternalError(c, fmt.Sprintf("Failed to generate form: %s", err.Error()))
		return
	}

	response.Created(c, form)
}
