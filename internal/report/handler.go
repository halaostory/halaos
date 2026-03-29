package report

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/pdf"
	"github.com/halaostory/halaos/internal/store"
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

func (h *Handler) GetDTR(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	startStr := c.Query("start")
	endStr := c.Query("end")
	employeeIDStr := c.Query("employee_id")

	if startStr == "" || endStr == "" {
		response.BadRequest(c, "start and end dates are required")
		return
	}
	startDate, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		response.BadRequest(c, "Invalid start date")
		return
	}
	endDate, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		response.BadRequest(c, "Invalid end date")
		return
	}
	endDate = endDate.AddDate(0, 0, 1)
	startTz := pgtype.Timestamptz{Time: startDate, Valid: true}
	endTz := pgtype.Timestamptz{Time: endDate, Valid: true}

	if employeeIDStr != "" {
		empID, _ := strconv.ParseInt(employeeIDStr, 10, 64)
		records, err := h.queries.GetDTR(c.Request.Context(), store.GetDTRParams{
			CompanyID:   companyID,
			EmployeeID:  empID,
			ClockInAt:   startTz,
			ClockInAt_2: endTz,
		})
		if err != nil {
			response.InternalError(c, "Failed to get DTR")
			return
		}
		response.OK(c, records)
	} else {
		records, err := h.queries.GetDTRAllEmployees(c.Request.Context(), store.GetDTRAllEmployeesParams{
			CompanyID:   companyID,
			ClockInAt:   startTz,
			ClockInAt_2: endTz,
		})
		if err != nil {
			response.InternalError(c, "Failed to get DTR")
			return
		}
		response.OK(c, records)
	}
}

func (h *Handler) GetDTRCSV(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	startStr := c.Query("start")
	endStr := c.Query("end")
	if startStr == "" || endStr == "" {
		response.BadRequest(c, "start and end dates are required")
		return
	}
	startDate, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		response.BadRequest(c, "Invalid start date")
		return
	}
	endDate, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		response.BadRequest(c, "Invalid end date")
		return
	}
	endDate = endDate.AddDate(0, 0, 1)
	startTz := pgtype.Timestamptz{Time: startDate, Valid: true}
	endTz := pgtype.Timestamptz{Time: endDate, Valid: true}

	records, err := h.queries.GetDTRAllEmployees(c.Request.Context(), store.GetDTRAllEmployeesParams{
		CompanyID:   companyID,
		ClockInAt:   startTz,
		ClockInAt_2: endTz,
	})
	if err != nil {
		response.InternalError(c, "Failed to get DTR")
		return
	}

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=dtr_%s_%s.csv", startStr, c.Query("end")))

	var buf bytes.Buffer
	buf.WriteString("Employee No,Name,Department,Position,Date,Clock In,Clock Out,Work Hours,OT Hours,Late (min),Undertime (min),Status\n")
	for _, r := range records {
		clockIn := ""
		date := ""
		if r.ClockInAt.Valid {
			clockIn = r.ClockInAt.Time.Format("15:04")
			date = r.ClockInAt.Time.Format("2006-01-02")
		}
		clockOut := ""
		if r.ClockOutAt.Valid {
			clockOut = r.ClockOutAt.Time.Format("15:04")
		}
		wh, _ := r.WorkHours.Float64Value()
		oh, _ := r.OvertimeHours.Float64Value()
		var late, ut int32
		if r.LateMinutes != nil {
			late = *r.LateMinutes
		}
		if r.UndertimeMinutes != nil {
			ut = *r.UndertimeMinutes
		}
		buf.WriteString(fmt.Sprintf("%s,\"%s %s\",%s,%s,%s,%s,%s,%.2f,%.2f,%d,%d,%s\n",
			r.EmployeeNo, r.FirstName, r.LastName,
			r.DepartmentName, r.PositionName,
			date, clockIn, clockOut,
			wh.Float64, oh.Float64,
			late, ut,
			r.Status,
		))
	}
	c.Data(http.StatusOK, "text/csv", buf.Bytes())
}

func (h *Handler) GetDOLERegister(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	comp, err := h.queries.GetCompanyByID(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get company")
		return
	}
	emps, err := h.queries.ListEmployeesForDOLERegister(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list employees")
		return
	}
	pdfBytes, err := pdf.GenerateDOLERegister(comp, emps)
	if err != nil {
		h.logger.Error("failed to generate DOLE register PDF", "error", err)
		response.InternalError(c, "Failed to generate PDF")
		return
	}
	fileName := fmt.Sprintf("DOLE_Register_%s.pdf", time.Now().Format("20060102"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Data(200, "application/pdf", pdfBytes)
}
