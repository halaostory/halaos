package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/store"
)

func (r *ToolRegistry) toolQueryAttendanceSummary(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	summary, err := r.queries.GetTodayAttendanceSummary(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("query attendance summary: %w", err)
	}
	return toJSON(summary)
}

func (r *ToolRegistry) toolGetMyAttendance(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	att, err := r.queries.GetOpenAttendance(ctx, store.GetOpenAttendanceParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err != nil {
		return toJSON(map[string]string{"status": "not_clocked_in"})
	}
	return toJSON(att)
}

func (r *ToolRegistry) toolClockIn(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	// Check if already clocked in
	_, err = r.queries.GetOpenAttendance(ctx, store.GetOpenAttendanceParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err == nil {
		return toJSON(map[string]any{
			"success": false,
			"message": "You are already clocked in. Please clock out first.",
		})
	}

	att, err := r.queries.ClockIn(ctx, store.ClockInParams{
		CompanyID:     companyID,
		EmployeeID:    emp.ID,
		ClockInSource: "ai",
		ClockInLat:    pgtype.Numeric{Valid: false},
		ClockInLng:    pgtype.Numeric{Valid: false},
	})
	if err != nil {
		return "", fmt.Errorf("clock in: %w", err)
	}

	return toJSON(map[string]any{
		"success":       true,
		"attendance_id": att.ID,
		"clock_in_at":   att.ClockInAt.Time.Format(time.RFC3339),
		"source":        "ai",
		"message":       "Successfully clocked in.",
	})
}

func (r *ToolRegistry) toolClockOut(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	// Find open attendance record
	open, err := r.queries.GetOpenAttendance(ctx, store.GetOpenAttendanceParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err != nil {
		return toJSON(map[string]any{
			"success": false,
			"message": "No open attendance record found. You need to clock in first.",
		})
	}

	source := "ai"
	att, err := r.queries.ClockOut(ctx, store.ClockOutParams{
		ID:             open.ID,
		EmployeeID:     emp.ID,
		ClockOutSource: &source,
		ClockOutLat:    pgtype.Numeric{Valid: false},
		ClockOutLng:    pgtype.Numeric{Valid: false},
	})
	if err != nil {
		return "", fmt.Errorf("clock out: %w", err)
	}

	return toJSON(map[string]any{
		"success":       true,
		"attendance_id": att.ID,
		"clock_in_at":   att.ClockInAt.Time.Format(time.RFC3339),
		"clock_out_at":  att.ClockOutAt.Time.Format(time.RFC3339),
		"source":        "ai",
		"message":       "Successfully clocked out.",
	})
}

func (r *ToolRegistry) toolGenerateAttendanceReport(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	startDateStr, _ := input["start_date"].(string)
	endDateStr, _ := input["end_date"].(string)

	if startDateStr == "" || endDateStr == "" {
		return "", fmt.Errorf("start_date and end_date are required")
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid start_date format, use YYYY-MM-DD")
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid end_date format, use YYYY-MM-DD")
	}

	if endDate.Before(startDate) {
		return "", fmt.Errorf("end_date must not be before start_date")
	}

	// Cap date range to 93 days (one quarter)
	if endDate.Sub(startDate) > 93*24*time.Hour {
		return "", fmt.Errorf("date range cannot exceed 93 days")
	}

	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	// Check permissions: non-managers can only query their own data
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	isManager := user.Role == "admin" || user.Role == "manager"

	// End date should be exclusive (next day start)
	endDateExclusive := endDate.Add(24 * time.Hour)

	startTS := pgtype.Timestamptz{Time: startDate, Valid: true}
	endTS := pgtype.Timestamptz{Time: endDateExclusive, Valid: true}

	summaryMap := make(map[int64]*attendanceSummaryEntry)

	if eid, ok := input["employee_id"].(float64); ok && eid > 0 {
		targetEmpID := int64(eid)
		// Non-managers can only query their own attendance
		if !isManager && targetEmpID != emp.ID {
			return "", fmt.Errorf("you can only view your own attendance report")
		}
		records, err := r.queries.GetDTR(ctx, store.GetDTRParams{
			CompanyID:   companyID,
			EmployeeID:  targetEmpID,
			ClockInAt:   startTS,
			ClockInAt_2: endTS,
		})
		if err != nil {
			return "", fmt.Errorf("get DTR: %w", err)
		}
		for _, rec := range records {
			s := getOrCreateSummary(summaryMap, rec.EmployeeID, rec.EmployeeNo, rec.FirstName+" "+rec.LastName, rec.DepartmentName)
			accumulateDTR(s, rec.WorkHours, rec.OvertimeHours, rec.LateMinutes)
		}
	} else if isManager {
		// Company-wide report: managers/admins only
		records, err := r.queries.GetDTRAllEmployees(ctx, store.GetDTRAllEmployeesParams{
			CompanyID:   companyID,
			ClockInAt:   startTS,
			ClockInAt_2: endTS,
		})
		if err != nil {
			return "", fmt.Errorf("get DTR all employees: %w", err)
		}
		for _, rec := range records {
			s := getOrCreateSummary(summaryMap, rec.EmployeeID, rec.EmployeeNo, rec.FirstName+" "+rec.LastName, rec.DepartmentName)
			accumulateDTR(s, rec.WorkHours, rec.OvertimeHours, rec.LateMinutes)
		}
	} else {
		// Non-manager without employee_id: default to own data
		records, err := r.queries.GetDTR(ctx, store.GetDTRParams{
			CompanyID:   companyID,
			EmployeeID:  emp.ID,
			ClockInAt:   startTS,
			ClockInAt_2: endTS,
		})
		if err != nil {
			return "", fmt.Errorf("get DTR: %w", err)
		}
		for _, rec := range records {
			s := getOrCreateSummary(summaryMap, rec.EmployeeID, rec.EmployeeNo, rec.FirstName+" "+rec.LastName, rec.DepartmentName)
			accumulateDTR(s, rec.WorkHours, rec.OvertimeHours, rec.LateMinutes)
		}
	}

	employees := make([]attendanceSummaryEntry, 0, len(summaryMap))
	totalPresent := 0
	totalLate := 0
	for _, s := range summaryMap {
		employees = append(employees, *s)
		totalPresent += s.PresentDays
		totalLate += s.LateDays
	}

	return toJSON(map[string]any{
		"period":               startDateStr + " to " + endDateStr,
		"total_employees":      len(summaryMap),
		"total_present_days":   totalPresent,
		"total_late_instances": totalLate,
		"employees":            employees,
	})
}

type attendanceSummaryEntry struct {
	EmployeeNo     string  `json:"employee_no"`
	Name           string  `json:"name"`
	Department     string  `json:"department"`
	PresentDays    int     `json:"present_days"`
	LateDays       int     `json:"late_days"`
	TotalWorkHours float64 `json:"total_work_hours"`
	TotalOTHours   float64 `json:"total_ot_hours"`
	TotalLateMin   int     `json:"total_late_minutes"`
}

func getOrCreateSummary(m map[int64]*attendanceSummaryEntry, empID int64, empNo, name, dept string) *attendanceSummaryEntry {
	if s, ok := m[empID]; ok {
		return s
	}
	s := &attendanceSummaryEntry{
		EmployeeNo: empNo,
		Name:       name,
		Department: dept,
	}
	m[empID] = s
	return s
}

func accumulateDTR(s *attendanceSummaryEntry, workHours, overtimeHours pgtype.Numeric, lateMinutes *int32) {
	s.PresentDays++
	if wh, err := workHours.Float64Value(); err == nil && wh.Valid {
		s.TotalWorkHours += wh.Float64
	}
	if oh, err := overtimeHours.Float64Value(); err == nil && oh.Valid {
		s.TotalOTHours += oh.Float64
	}
	if lateMinutes != nil && *lateMinutes > 0 {
		s.LateDays++
		s.TotalLateMin += int(*lateMinutes)
	}
}

// attendanceDefs returns tool definitions for attendance-related tools.
func attendanceDefs() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "query_attendance_summary",
			Description: "Get attendance summary for today or a date range. Shows present, absent, late counts for the company.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "get_my_attendance",
			Description: "Get the current user's attendance status for today - whether clocked in, clock-in time, work hours so far.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "clock_in",
			Description: "Clock in (start work) for the current user. Records attendance with source='ai'. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "clock_out",
			Description: "Clock out (end work) for the current user. Closes the current open attendance record. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "generate_attendance_report",
			Description: "Generate an attendance/DTR report for a date range. Returns summary with present days, late counts, overtime hours, etc. If no employee_id is specified, generates a company-wide report.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"start_date":  map[string]any{"type": "string", "description": "Report start date in YYYY-MM-DD format."},
					"end_date":    map[string]any{"type": "string", "description": "Report end date in YYYY-MM-DD format."},
					"employee_id": map[string]any{"type": "integer", "description": "Optional employee ID. Omit for company-wide report."},
				},
				"required": []string{"start_date", "end_date"},
			}),
		},
	}
}

// registerAttendanceTools registers attendance-related tool executors.
func (r *ToolRegistry) registerAttendanceTools() {
	r.tools["query_attendance_summary"] = r.toolQueryAttendanceSummary
	r.tools["get_my_attendance"] = r.toolGetMyAttendance
	r.tools["clock_in"] = r.toolClockIn
	r.tools["clock_out"] = r.toolClockOut
	r.tools["generate_attendance_report"] = r.toolGenerateAttendanceReport
}
