package importexport

import (
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

var dateFormats = []string{
	"2006-01-02",  // YYYY-MM-DD
	"1/2/2006",    // M/D/YYYY
	"01/02/2006",  // MM/DD/YYYY
	"2006/01/02",  // YYYY/MM/DD
	"Jan 2, 2006", // Mon D, YYYY
	"2-Jan-2006",  // D-Mon-YYYY
}

func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range dateFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date format: %s", s)
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger}
}

// ImportEmployeesCSV parses a CSV file and bulk creates employees.
// Expected columns: employee_no, first_name, last_name, middle_name, email, phone, gender, birth_date, hire_date, employment_type
func (h *Handler) ImportEmployeesCSV(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "CSV file is required")
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true

	// Read header row
	header, err := reader.Read()
	if err != nil {
		response.BadRequest(c, "Failed to read CSV header")
		return
	}

	// Map column names to indices
	colIdx := make(map[string]int)
	for i, col := range header {
		colIdx[strings.TrimSpace(strings.ToLower(col))] = i
	}

	// Required columns
	requiredCols := []string{"employee_no", "first_name", "last_name", "hire_date", "employment_type"}
	for _, col := range requiredCols {
		if _, ok := colIdx[col]; !ok {
			response.BadRequest(c, fmt.Sprintf("Missing required column: %s", col))
			return
		}
	}

	var imported, skipped int
	var errors []string
	lineNum := 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			lineNum++
			skipped++
			errors = append(errors, fmt.Sprintf("Line %d: parse error", lineNum))
			continue
		}
		lineNum++

		getCol := func(name string) string {
			idx, ok := colIdx[name]
			if !ok || idx >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[idx])
		}

		empNo := getCol("employee_no")
		firstName := getCol("first_name")
		lastName := getCol("last_name")
		hireDateStr := getCol("hire_date")
		employmentType := getCol("employment_type")

		if empNo == "" || firstName == "" || lastName == "" || hireDateStr == "" {
			skipped++
			errors = append(errors, fmt.Sprintf("Line %d: missing required fields", lineNum))
			continue
		}

		hireDate, err := parseDate(hireDateStr)
		if err != nil {
			skipped++
			errors = append(errors, fmt.Sprintf("Line %d: invalid hire_date format (accepted: YYYY-MM-DD, M/D/YYYY, MM/DD/YYYY)", lineNum))
			continue
		}

		if employmentType == "" {
			employmentType = "regular"
		}

		var birthDate pgtype.Date
		if bd := getCol("birth_date"); bd != "" {
			parsed, err := parseDate(bd)
			if err == nil {
				birthDate = pgtype.Date{Time: parsed, Valid: true}
			}
		}

		params := store.CreateEmployeeParams{
			CompanyID:      companyID,
			EmployeeNo:     empNo,
			FirstName:      firstName,
			LastName:        lastName,
			HireDate:        hireDate,
			EmploymentType:  employmentType,
			BirthDate:       birthDate,
		}

		if v := getCol("middle_name"); v != "" {
			params.MiddleName = &v
		}
		if v := getCol("email"); v != "" {
			params.Email = &v
		}
		if v := getCol("phone"); v != "" {
			params.Phone = &v
		}
		if v := getCol("gender"); v != "" {
			params.Gender = &v
		}

		// Resolve department by name
		if deptName := getCol("department"); deptName != "" {
			dept, err := h.queries.GetDepartmentByName(c.Request.Context(), store.GetDepartmentByNameParams{
				CompanyID: companyID,
				Lower:     deptName,
			})
			if err == nil {
				params.DepartmentID = &dept.ID
			}
		}

		// Resolve position by title
		if posTitle := getCol("position"); posTitle != "" {
			pos, err := h.queries.GetPositionByTitle(c.Request.Context(), store.GetPositionByTitleParams{
				CompanyID: companyID,
				Lower:     posTitle,
			})
			if err == nil {
				params.PositionID = &pos.ID
			}
		}

		_, err = h.queries.CreateEmployee(c.Request.Context(), params)
		if err != nil {
			skipped++
			errors = append(errors, fmt.Sprintf("Line %d: %s", lineNum, err.Error()))
			continue
		}
		imported++
	}

	response.OK(c, gin.H{
		"imported": imported,
		"skipped":  skipped,
		"errors":   errors,
	})
}

// PreviewImportCSV parses a CSV file and returns a preview of what would be imported.
func (h *Handler) PreviewImportCSV(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "CSV file is required")
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		response.BadRequest(c, "Failed to read CSV header")
		return
	}

	colIdx := make(map[string]int)
	for i, col := range header {
		colIdx[strings.TrimSpace(strings.ToLower(col))] = i
	}

	requiredCols := []string{"employee_no", "first_name", "last_name", "hire_date", "employment_type"}
	var missingCols []string
	for _, col := range requiredCols {
		if _, ok := colIdx[col]; !ok {
			missingCols = append(missingCols, col)
		}
	}
	if len(missingCols) > 0 {
		response.BadRequest(c, fmt.Sprintf("Missing required columns: %s", strings.Join(missingCols, ", ")))
		return
	}

	// Load existing employee numbers for duplicate detection
	existingEmps, _ := h.queries.ListActiveEmployees(c.Request.Context(), companyID)
	existingNos := make(map[string]bool)
	for _, e := range existingEmps {
		existingNos[e.EmployeeNo] = true
	}

	type PreviewRow struct {
		Line           int      `json:"line"`
		EmployeeNo     string   `json:"employee_no"`
		FirstName      string   `json:"first_name"`
		LastName       string   `json:"last_name"`
		Email          string   `json:"email"`
		HireDate       string   `json:"hire_date"`
		EmploymentType string   `json:"employment_type"`
		Department     string   `json:"department"`
		Position       string   `json:"position"`
		Valid          bool     `json:"valid"`
		Errors         []string `json:"errors"`
	}

	var rows []PreviewRow
	lineNum := 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		lineNum++
		if err != nil {
			rows = append(rows, PreviewRow{Line: lineNum, Valid: false, Errors: []string{"Parse error"}})
			continue
		}

		getCol := func(name string) string {
			idx, ok := colIdx[name]
			if !ok || idx >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[idx])
		}

		row := PreviewRow{
			Line:           lineNum,
			EmployeeNo:     getCol("employee_no"),
			FirstName:      getCol("first_name"),
			LastName:        getCol("last_name"),
			Email:          getCol("email"),
			HireDate:       getCol("hire_date"),
			EmploymentType: getCol("employment_type"),
			Department:     getCol("department"),
			Position:       getCol("position"),
			Valid:          true,
		}

		// Validate
		if row.EmployeeNo == "" {
			row.Errors = append(row.Errors, "Missing employee_no")
			row.Valid = false
		} else if existingNos[row.EmployeeNo] {
			row.Errors = append(row.Errors, "Duplicate employee_no")
			row.Valid = false
		}
		if row.FirstName == "" {
			row.Errors = append(row.Errors, "Missing first_name")
			row.Valid = false
		}
		if row.LastName == "" {
			row.Errors = append(row.Errors, "Missing last_name")
			row.Valid = false
		}
		if row.HireDate == "" {
			row.Errors = append(row.Errors, "Missing hire_date")
			row.Valid = false
		} else if _, err := parseDate(row.HireDate); err != nil {
			row.Errors = append(row.Errors, "Invalid hire_date (accepted: YYYY-MM-DD, M/D/YYYY, MM/DD/YYYY)")
			row.Valid = false
		}
		if row.EmploymentType == "" {
			row.EmploymentType = "regular"
		}

		rows = append(rows, row)
	}

	validCount := 0
	invalidCount := 0
	for _, r := range rows {
		if r.Valid {
			validCount++
		} else {
			invalidCount++
		}
	}

	response.OK(c, gin.H{
		"columns": header,
		"rows":    rows,
		"summary": gin.H{
			"total":   len(rows),
			"valid":   validCount,
			"invalid": invalidCount,
		},
	})
}

// ExportAttendanceCSV exports attendance records for a date range.
func (h *Handler) ExportAttendanceCSV(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	startStr := c.Query("start")
	endStr := c.Query("end")
	if startStr == "" || endStr == "" {
		response.BadRequest(c, "start and end query parameters are required (YYYY-MM-DD)")
		return
	}

	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		response.BadRequest(c, "Invalid start date format")
		return
	}
	end, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		response.BadRequest(c, "Invalid end date format")
		return
	}
	end = end.AddDate(0, 0, 1) // exclusive end

	records, err := h.queries.ExportAttendanceLogs(c.Request.Context(), store.ExportAttendanceLogsParams{
		CompanyID:   companyID,
		ClockInAt:   pgtype.Timestamptz{Time: start, Valid: true},
		ClockInAt_2: pgtype.Timestamptz{Time: end, Valid: true},
	})
	if err != nil {
		response.InternalError(c, "Failed to export attendance records")
		return
	}

	filename := fmt.Sprintf("attendance_%s_%s.csv", startStr, endStr)
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	w := csv.NewWriter(c.Writer)
	defer w.Flush()

	_ = w.Write([]string{
		"Employee No", "First Name", "Last Name",
		"Clock In", "Clock Out",
		"Work Hours", "OT Hours", "Late (min)", "Undertime (min)", "Status",
	})

	for _, r := range records {
		clockIn := ""
		if r.ClockInAt.Valid {
			clockIn = r.ClockInAt.Time.Format("2006-01-02 15:04:05")
		}
		clockOut := ""
		if r.ClockOutAt.Valid {
			clockOut = r.ClockOutAt.Time.Format("2006-01-02 15:04:05")
		}

		_ = w.Write([]string{
			r.EmployeeNo, r.FirstName, r.LastName,
			clockIn, clockOut,
			numStr(r.WorkHours), numStr(r.OvertimeHours),
			intPtrStr(r.LateMinutes), intPtrStr(r.UndertimeMinutes),
			r.Status,
		})
	}

	c.Status(http.StatusOK)
}

// ExportLeaveBalancesCSV exports leave balances for all employees for a given year.
func (h *Handler) ExportLeaveBalancesCSV(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	yearStr := c.DefaultQuery("year", fmt.Sprintf("%d", time.Now().Year()))
	year, err := strconv.ParseInt(yearStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid year parameter")
		return
	}

	balances, err := h.queries.ExportLeaveBalances(c.Request.Context(), store.ExportLeaveBalancesParams{
		CompanyID: companyID,
		Year:      int32(year),
	})
	if err != nil {
		response.InternalError(c, "Failed to export leave balances")
		return
	}

	filename := fmt.Sprintf("leave_balances_%d.csv", year)
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	w := csv.NewWriter(c.Writer)
	defer w.Flush()

	_ = w.Write([]string{
		"Employee No", "First Name", "Last Name",
		"Leave Type", "Earned", "Used", "Carried", "Adjusted", "Remaining",
	})

	for _, b := range balances {
		earned := numFloat(b.Earned)
		used := numFloat(b.Used)
		carried := numFloat(b.Carried)
		adjusted := numFloat(b.Adjusted)
		remaining := earned + carried + adjusted - used

		_ = w.Write([]string{
			b.EmployeeNo, b.FirstName, b.LastName,
			b.LeaveTypeName,
			numStr(b.Earned), numStr(b.Used), numStr(b.Carried), numStr(b.Adjusted),
			fmt.Sprintf("%.2f", remaining),
		})
	}

	c.Status(http.StatusOK)
}

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

func numFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, _ := n.Float64Value()
	if !f.Valid {
		return 0
	}
	return f.Float64
}

func intPtrStr(p *int32) string {
	if p == nil {
		return "0"
	}
	return strconv.Itoa(int(*p))
}
