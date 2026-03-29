package breaks

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/xuri/excelize/v2"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// MonthlyReport generates and returns the monthly break report as an Excel file.
func (h *Handler) MonthlyReport(c *gin.Context) {
	yearStr := c.Query("year")
	monthStr := c.Query("month")
	if yearStr == "" || monthStr == "" {
		response.BadRequest(c, "year and month query parameters are required")
		return
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 2000 || year > 2100 {
		response.BadRequest(c, "Invalid year. Must be between 2000 and 2100")
		return
	}

	month, err := strconv.Atoi(monthStr)
	if err != nil || month < 1 || month > 12 {
		response.BadRequest(c, "Invalid month. Must be between 1 and 12")
		return
	}

	companyID := auth.GetCompanyID(c)
	ctx := c.Request.Context()

	// Build date range
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	startTz := pgtype.Timestamptz{Time: start, Valid: true}
	endTz := pgtype.Timestamptz{Time: end, Valid: true}

	// Query attendance report
	attendanceRows, err := h.queries.GetAttendanceReport(ctx, store.GetAttendanceReportParams{
		CompanyID:   companyID,
		ClockInAt:   startTz,
		ClockInAt_2: endTz,
	})
	if err != nil {
		h.logger.Error("failed to get attendance report", "error", err)
		response.InternalError(c, "Failed to get attendance report")
		return
	}

	// Query break summary
	breakRows, err := h.queries.GetMonthlyBreakSummary(ctx, store.GetMonthlyBreakSummaryParams{
		CompanyID: companyID,
		StartAt:   start,
		StartAt_2: end,
	})
	if err != nil {
		h.logger.Error("failed to get break summary", "error", err)
		response.InternalError(c, "Failed to get break summary")
		return
	}

	// Query bot user links for Telegram IDs
	botLinks, err := h.queries.ListBotUserLinksByCompany(ctx, companyID)
	if err != nil {
		h.logger.Error("failed to list bot user links", "error", err)
		// Non-fatal: continue without bot links
		botLinks = []store.BotUserLink{}
	}

	// Query active employees to get employee_id -> user_id mapping
	employees, err := h.queries.ListActiveEmployees(ctx, companyID)
	if err != nil {
		h.logger.Error("failed to list employees", "error", err)
		response.InternalError(c, "Failed to list employees")
		return
	}

	// Build employee_id -> user_id map
	empUserMap := make(map[int64]int64, len(employees))
	for _, emp := range employees {
		if emp.UserID != nil {
			empUserMap[emp.ID] = *emp.UserID
		}
	}

	// Build user_id -> platform_user_id map from bot links
	userBotMap := make(map[int64]string, len(botLinks))
	for _, link := range botLinks {
		if link.PlatformUserID != nil {
			userBotMap[link.UserID] = *link.PlatformUserID
		}
	}

	// Build break data lookup: employee_id -> break_type -> summary
	breakMap := make(map[int64]map[string]*breakInfo)
	for _, br := range breakRows {
		if _, ok := breakMap[br.EmployeeID]; !ok {
			breakMap[br.EmployeeID] = make(map[string]*breakInfo)
		}
		breakMap[br.EmployeeID][br.BreakType] = &breakInfo{
			count:           br.TotalCount,
			totalMinutes:    br.TotalMinutes,
			overtimeMinutes: br.TotalOvertimeMinutes,
			overtimeCount:   br.OvertimeCount,
		}
	}

	// Generate Excel
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"

	// Write headers
	headers := []string{
		"用户昵称", "用户标识", "工作天数", "工作时间总计", "纯工作时间总计",
		"迟到天数", "迟到总时长", "早退天数", "早退总时长",
		"吃饭次数", "吃饭用时", "吃饭超时",
		"上厕所次数", "上厕所用时", "上厕所超时",
		"休息次数", "休息用时",
		"中途离岗次数", "中途离岗用时",
		"所有次数总计", "所有用时总计", "所有超时总计",
		"手动惩罚",
	}
	for i, h := range headers {
		col := string(rune('A' + i))
		f.SetCellValue(sheet, col+"1", h)
	}

	// Write data rows
	for rowIdx, att := range attendanceRows {
		row := rowIdx + 2 // 1-indexed, skip header
		empID := att.EmployeeID

		// A: 用户昵称
		name := att.FirstName + " " + att.LastName
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), name)

		// B: 用户标识 (Telegram ID)
		botID := dashStr
		if userID, ok := empUserMap[empID]; ok {
			if pid, ok2 := userBotMap[userID]; ok2 {
				botID = pid
			}
		}
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), botID)

		// C: 工作天数
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), fmtIntOrDash(att.DaysWorked))

		// D: 工作时间总计
		totalWorkHours := toFloat64(att.TotalWorkHours)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), fmtHoursOrDash(totalWorkHours))

		// Calculate total break time for this employee
		empBreaks := breakMap[empID]
		var totalBreakMinutes int32
		if empBreaks != nil {
			for _, bi := range empBreaks {
				totalBreakMinutes += bi.totalMinutes
			}
		}

		// E: 纯工作时间总计 (work hours minus break time)
		breakHours := float64(totalBreakMinutes) / 60.0
		netWorkHours := totalWorkHours - breakHours
		if netWorkHours < 0 {
			netWorkHours = 0
		}
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), fmtHoursOrDash(netWorkHours))

		// F: 迟到天数
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row), fmtIntOrDash(att.LateCount))

		// G: 迟到总时长
		totalLateMin := toFloat64(att.TotalLateMinutes)
		f.SetCellValue(sheet, fmt.Sprintf("G%d", row), fmtMinutesOrDash(int32(totalLateMin)))

		// H: 早退天数
		f.SetCellValue(sheet, fmt.Sprintf("H%d", row), fmtIntOrDash(att.UndertimeCount))

		// I: 早退总时长
		totalUndertimeMin := toFloat64(att.TotalUndertimeMinutes)
		f.SetCellValue(sheet, fmt.Sprintf("I%d", row), fmtMinutesOrDash(int32(totalUndertimeMin)))

		// J-L: 吃饭 (meal)
		writeBreakCols(f, sheet, row, "J", empBreaks, "meal", true)

		// M-O: 上厕所 (bathroom)
		writeBreakCols(f, sheet, row, "M", empBreaks, "bathroom", true)

		// P-Q: 休息 (rest) — no overtime column
		writeBreakCols(f, sheet, row, "P", empBreaks, "rest", false)

		// R-S: 中途离岗 (leave_post) — no overtime column
		writeBreakCols(f, sheet, row, "R", empBreaks, "leave_post", false)

		// T: 所有次数总计
		var allCount int32
		if empBreaks != nil {
			for _, bi := range empBreaks {
				allCount += bi.count
			}
		}
		f.SetCellValue(sheet, fmt.Sprintf("T%d", row), fmtInt32OrDash(allCount))

		// U: 所有用时总计
		f.SetCellValue(sheet, fmt.Sprintf("U%d", row), fmtHoursOrDash(float64(totalBreakMinutes)/60.0))

		// V: 所有超时总计
		var allOvertime int32
		if empBreaks != nil {
			if m, ok := empBreaks["meal"]; ok {
				allOvertime += m.overtimeMinutes
			}
			if b, ok := empBreaks["bathroom"]; ok {
				allOvertime += b.overtimeMinutes
			}
		}
		f.SetCellValue(sheet, fmt.Sprintf("V%d", row), fmtMinutesOrDash(allOvertime))

		// W: 手动惩罚
		f.SetCellValue(sheet, fmt.Sprintf("W%d", row), dashStr)
	}

	// Write to response
	filename := fmt.Sprintf("break_report_%d_%02d.xlsx", year, month)
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	if err := f.Write(c.Writer); err != nil {
		h.logger.Error("failed to write excel", "error", err)
		return
	}
}

const dashStr = "\u2014\u2014"

type breakInfo struct {
	count           int32
	totalMinutes    int32
	overtimeMinutes int32
	overtimeCount   int32
}

// writeBreakCols writes 2 or 3 columns for a break type starting at the given column.
func writeBreakCols(f *excelize.File, sheet string, row int, startCol string, empBreaks map[string]*breakInfo, breakType string, hasOvertime bool) {
	colIdx := int(startCol[0] - 'A')

	var bi *breakInfo
	if empBreaks != nil {
		bi = empBreaks[breakType]
	}

	// Column 1: count
	col1 := string(rune('A' + colIdx))
	if bi != nil && bi.count > 0 {
		f.SetCellValue(sheet, fmt.Sprintf("%s%d", col1, row), bi.count)
	} else {
		f.SetCellValue(sheet, fmt.Sprintf("%s%d", col1, row), dashStr)
	}

	// Column 2: total time in hours
	col2 := string(rune('A' + colIdx + 1))
	if bi != nil && bi.totalMinutes > 0 {
		f.SetCellValue(sheet, fmt.Sprintf("%s%d", col2, row), fmtHoursOrDash(float64(bi.totalMinutes)/60.0))
	} else {
		f.SetCellValue(sheet, fmt.Sprintf("%s%d", col2, row), dashStr)
	}

	// Column 3: overtime (only for meal and bathroom)
	if hasOvertime {
		col3 := string(rune('A' + colIdx + 2))
		if bi != nil && bi.overtimeMinutes > 0 {
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", col3, row), fmtOvertimeWithCount(bi.overtimeMinutes, bi.overtimeCount))
		} else {
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", col3, row), dashStr)
		}
	}
}

// fmtHoursOrDash formats hours as "X.X 小时" or returns dash.
func fmtHoursOrDash(hours float64) string {
	if hours <= 0 {
		return dashStr
	}
	return fmt.Sprintf("%.1f 小时", hours)
}

// fmtMinutesOrDash formats minutes as "X 分钟" or returns dash.
func fmtMinutesOrDash(minutes int32) string {
	if minutes <= 0 {
		return dashStr
	}
	return fmt.Sprintf("%d 分钟", minutes)
}

// fmtOvertimeWithCount formats overtime as "X 分钟（共 N 次）".
func fmtOvertimeWithCount(minutes, count int32) string {
	return fmt.Sprintf("%d 分钟（共 %d 次）", minutes, count)
}

// fmtIntOrDash returns the int64 value or dash string if zero.
func fmtIntOrDash(v int64) string {
	if v == 0 {
		return dashStr
	}
	return strconv.FormatInt(v, 10)
}

// fmtInt32OrDash returns the int32 value or dash string if zero.
func fmtInt32OrDash(v int32) string {
	if v == 0 {
		return dashStr
	}
	return strconv.FormatInt(int64(v), 10)
}

// toFloat64 converts an interface{} (from sqlc aggregate queries) to float64.
func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case []byte:
		f, _ := strconv.ParseFloat(string(val), 64)
		return f
	default:
		return 0
	}
}
