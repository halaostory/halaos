package burnout

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Scorer calculates burnout risk scores for all active employees in a company.
type Scorer struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// BurnoutFactor describes a single contributing factor to an employee's burnout score.
type BurnoutFactor struct {
	Factor string `json:"factor"`
	Points int    `json:"points"`
	Detail string `json:"detail"`
}

// EmployeeBurnout holds the computed burnout score and contributing factors.
type EmployeeBurnout struct {
	EmployeeID   int64           `json:"employee_id"`
	EmployeeNo   string          `json:"employee_no"`
	Name         string          `json:"name"`
	Department   string          `json:"department"`
	BurnoutScore int             `json:"burnout_score"`
	Factors      []BurnoutFactor `json:"factors"`
}

// NewScorer creates a new burnout scorer.
func NewScorer(pool *pgxpool.Pool, logger *slog.Logger) *Scorer {
	return &Scorer{pool: pool, logger: logger}
}

// ScoreAll calculates burnout scores for all active employees in the given company.
func (s *Scorer) ScoreAll(ctx context.Context, companyID int64) ([]EmployeeBurnout, error) {
	employees, err := s.loadActiveEmployees(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("load active employees: %w", err)
	}
	if len(employees) == 0 {
		return nil, nil
	}

	// Load burnout signals
	otFrequency, err := s.loadOvertimeFrequency(ctx, companyID)
	if err != nil {
		s.logger.Warn("failed to load overtime frequency", "error", err)
		otFrequency = map[int64]otData{}
	}

	leaveAvoidance, err := s.loadLeaveAvoidance(ctx, companyID)
	if err != nil {
		s.logger.Warn("failed to load leave avoidance", "error", err)
		leaveAvoidance = map[int64]leaveAvoidData{}
	}

	longHours, err := s.loadLongWorkingHours(ctx, companyID)
	if err != nil {
		s.logger.Warn("failed to load long working hours", "error", err)
		longHours = map[int64]hoursData{}
	}

	weekendWork, err := s.loadWeekendWork(ctx, companyID)
	if err != nil {
		s.logger.Warn("failed to load weekend work", "error", err)
		weekendWork = map[int64]int64{}
	}

	irregularity, err := s.loadAttendanceIrregularity(ctx, companyID)
	if err != nil {
		s.logger.Warn("failed to load attendance irregularity", "error", err)
		irregularity = map[int64]float64{}
	}

	return computeScores(employees, otFrequency, leaveAvoidance, longHours, weekendWork, irregularity), nil
}

// computeScores calculates burnout scores from pre-loaded data maps.
func computeScores(
	employees []employeeInfo,
	otFrequency map[int64]otData,
	leaveAvoidance map[int64]leaveAvoidData,
	longHours map[int64]hoursData,
	weekendWork map[int64]int64,
	irregularity map[int64]float64,
) []EmployeeBurnout {
	var results []EmployeeBurnout

	for _, emp := range employees {
		var factors []BurnoutFactor
		score := 0

		// 1. Overtime frequency (+25) — many OT days in last 30 days
		if ot, ok := otFrequency[emp.id]; ok {
			if ot.otDays >= 10 {
				points := 25
				factors = append(factors, BurnoutFactor{
					Factor: "overtime_frequency",
					Points: points,
					Detail: fmt.Sprintf("%d overtime days in last 30 days (%.1f avg hours)", ot.otDays, ot.avgHours),
				})
				score += points
			} else if ot.otDays >= 5 {
				points := 15
				factors = append(factors, BurnoutFactor{
					Factor: "overtime_frequency",
					Points: points,
					Detail: fmt.Sprintf("%d overtime days in last 30 days (%.1f avg hours)", ot.otDays, ot.avgHours),
				})
				score += points
			}
		}

		// 2. Leave avoidance (+20) — has leave balance but rarely uses it
		if la, ok := leaveAvoidance[emp.id]; ok {
			if la.earned > 5 && la.used < 1 {
				points := 20
				factors = append(factors, BurnoutFactor{
					Factor: "leave_avoidance",
					Points: points,
					Detail: fmt.Sprintf("%.0f days earned but only %.0f used this year", la.earned, la.used),
				})
				score += points
			} else if la.earned > 5 && la.used/la.earned < 0.2 {
				points := 12
				factors = append(factors, BurnoutFactor{
					Factor: "leave_avoidance",
					Points: points,
					Detail: fmt.Sprintf("Only %.0f%% of leave balance used (%.0f/%.0f)", la.used/la.earned*100, la.used, la.earned),
				})
				score += points
			}
		}

		// 3. Long working hours (+20) — average daily hours > threshold
		if h, ok := longHours[emp.id]; ok {
			if h.avgDaily >= 11 {
				points := 20
				factors = append(factors, BurnoutFactor{
					Factor: "long_hours",
					Points: points,
					Detail: fmt.Sprintf("Average %.1f hours/day over %d workdays", h.avgDaily, h.workdays),
				})
				score += points
			} else if h.avgDaily >= 10 {
				points := 12
				factors = append(factors, BurnoutFactor{
					Factor: "long_hours",
					Points: points,
					Detail: fmt.Sprintf("Average %.1f hours/day over %d workdays", h.avgDaily, h.workdays),
				})
				score += points
			}
		}

		// 4. Weekend/holiday work (+15)
		if wkDays, ok := weekendWork[emp.id]; ok && wkDays >= 3 {
			points := 15
			if wkDays >= 6 {
				points = 20 // cap bonus for extreme weekend work
			}
			if points > 15 {
				points = 15
			}
			factors = append(factors, BurnoutFactor{
				Factor: "weekend_work",
				Points: points,
				Detail: fmt.Sprintf("Worked %d weekend/holiday days in last 30 days", wkDays),
			})
			score += points
		}

		// 5. Attendance irregularity (+20) — high variance in clock-in times
		if stddev, ok := irregularity[emp.id]; ok {
			if stddev >= 60 { // > 60 min std dev in clock-in time
				points := 20
				factors = append(factors, BurnoutFactor{
					Factor: "attendance_irregularity",
					Points: points,
					Detail: fmt.Sprintf("Clock-in time variance: %.0f min std deviation (last 30 days)", stddev),
				})
				score += points
			} else if stddev >= 30 {
				points := 10
				factors = append(factors, BurnoutFactor{
					Factor: "attendance_irregularity",
					Points: points,
					Detail: fmt.Sprintf("Clock-in time variance: %.0f min std deviation (last 30 days)", stddev),
				})
				score += points
			}
		}

		if score > 100 {
			score = 100
		}

		if score > 0 {
			results = append(results, EmployeeBurnout{
				EmployeeID:   emp.id,
				EmployeeNo:   emp.employeeNo,
				Name:         emp.firstName + " " + emp.lastName,
				Department:   emp.department,
				BurnoutScore: score,
				Factors:      factors,
			})
		}
	}

	return results
}

// UpsertScores persists calculated burnout scores.
func (s *Scorer) UpsertScores(ctx context.Context, companyID int64, scores []EmployeeBurnout) error {
	for _, r := range scores {
		factorsJSON, err := json.Marshal(r.Factors)
		if err != nil {
			s.logger.Error("failed to marshal burnout factors", "employee_id", r.EmployeeID, "error", err)
			continue
		}

		_, err = s.pool.Exec(ctx, `
			INSERT INTO employee_burnout_scores (company_id, employee_id, burnout_score, factors, calculated_at)
			VALUES ($1, $2, $3, $4, NOW())
			ON CONFLICT (company_id, employee_id)
			DO UPDATE SET burnout_score = $3, factors = $4, calculated_at = NOW()
		`, companyID, r.EmployeeID, r.BurnoutScore, factorsJSON)
		if err != nil {
			s.logger.Error("failed to upsert burnout score", "employee_id", r.EmployeeID, "error", err)
		}
	}
	return nil
}

// -- internal data types --

type employeeInfo struct {
	id           int64
	employeeNo   string
	firstName    string
	lastName     string
	department   string
	departmentID int64
}

type otData struct {
	otDays   int64
	avgHours float64
}

type leaveAvoidData struct {
	earned float64
	used   float64
}

type hoursData struct {
	avgDaily float64
	workdays int64
}

// -- data loaders --

func (s *Scorer) loadActiveEmployees(ctx context.Context, companyID int64) ([]employeeInfo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.employee_no, e.first_name, e.last_name,
		       COALESCE(d.name, '') as department,
		       COALESCE(e.department_id, 0) as department_id
		FROM employees e
		LEFT JOIN departments d ON d.id = e.department_id
		WHERE e.company_id = $1 AND e.status = 'active'
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []employeeInfo
	for rows.Next() {
		var emp employeeInfo
		if err := rows.Scan(&emp.id, &emp.employeeNo, &emp.firstName, &emp.lastName, &emp.department, &emp.departmentID); err != nil {
			return nil, fmt.Errorf("scan employee: %w", err)
		}
		result = append(result, emp)
	}
	return result, rows.Err()
}

func (s *Scorer) loadOvertimeFrequency(ctx context.Context, companyID int64) (map[int64]otData, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT employee_id,
		       COUNT(*) as ot_days,
		       AVG(ot_hours::float) as avg_hours
		FROM attendance_records
		WHERE company_id = $1
		  AND clock_in_at >= NOW() - INTERVAL '30 days'
		  AND ot_hours > 0
		GROUP BY employee_id
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]otData)
	for rows.Next() {
		var empID, days int64
		var avgH float64
		if err := rows.Scan(&empID, &days, &avgH); err != nil {
			return nil, err
		}
		result[empID] = otData{otDays: days, avgHours: avgH}
	}
	return result, rows.Err()
}

func (s *Scorer) loadLeaveAvoidance(ctx context.Context, companyID int64) (map[int64]leaveAvoidData, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT employee_id,
		       SUM(earned::float) as total_earned,
		       SUM(used::float) as total_used
		FROM leave_balances
		WHERE company_id = $1 AND year = EXTRACT(YEAR FROM NOW())::int
		GROUP BY employee_id
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]leaveAvoidData)
	for rows.Next() {
		var empID int64
		var earned, used float64
		if err := rows.Scan(&empID, &earned, &used); err != nil {
			return nil, err
		}
		result[empID] = leaveAvoidData{earned: earned, used: used}
	}
	return result, rows.Err()
}

func (s *Scorer) loadLongWorkingHours(ctx context.Context, companyID int64) (map[int64]hoursData, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT employee_id,
		       AVG(EXTRACT(EPOCH FROM (clock_out_at - clock_in_at)) / 3600) as avg_daily_hours,
		       COUNT(*) as workdays
		FROM attendance_records
		WHERE company_id = $1
		  AND clock_in_at >= NOW() - INTERVAL '30 days'
		  AND clock_out_at IS NOT NULL
		GROUP BY employee_id
		HAVING COUNT(*) >= 5
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]hoursData)
	for rows.Next() {
		var empID int64
		var avgH float64
		var days int64
		if err := rows.Scan(&empID, &avgH, &days); err != nil {
			return nil, err
		}
		result[empID] = hoursData{avgDaily: avgH, workdays: days}
	}
	return result, rows.Err()
}

func (s *Scorer) loadWeekendWork(ctx context.Context, companyID int64) (map[int64]int64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT employee_id, COUNT(*) as weekend_days
		FROM attendance_records
		WHERE company_id = $1
		  AND clock_in_at >= NOW() - INTERVAL '30 days'
		  AND EXTRACT(DOW FROM clock_in_at) IN (0, 6)
		GROUP BY employee_id
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]int64)
	for rows.Next() {
		var empID, count int64
		if err := rows.Scan(&empID, &count); err != nil {
			return nil, err
		}
		result[empID] = count
	}
	return result, rows.Err()
}

func (s *Scorer) loadAttendanceIrregularity(ctx context.Context, companyID int64) (map[int64]float64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT employee_id,
		       STDDEV(EXTRACT(EPOCH FROM clock_in_at::time) / 60) as clock_in_stddev_minutes
		FROM attendance_records
		WHERE company_id = $1
		  AND clock_in_at >= NOW() - INTERVAL '30 days'
		GROUP BY employee_id
		HAVING COUNT(*) >= 5
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]float64)
	for rows.Next() {
		var empID int64
		var stddev *float64
		if err := rows.Scan(&empID, &stddev); err != nil {
			return nil, err
		}
		if stddev != nil {
			result[empID] = *stddev
		}
	}
	return result, rows.Err()
}
