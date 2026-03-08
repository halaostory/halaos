package flightrisk

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Scorer calculates flight risk scores for all active employees in a company.
type Scorer struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// RiskFactor describes a single contributing factor to an employee's risk score.
type RiskFactor struct {
	Factor string `json:"factor"`
	Points int    `json:"points"`
	Detail string `json:"detail"`
}

// EmployeeRisk holds the computed risk score and contributing factors for one employee.
type EmployeeRisk struct {
	EmployeeID   int64        `json:"employee_id"`
	EmployeeNo   string       `json:"employee_no"`
	Name         string       `json:"name"`
	Department   string       `json:"department"`
	RiskScore    int          `json:"risk_score"`
	Factors      []RiskFactor `json:"factors"`
}

// NewScorer creates a new flight risk scorer.
func NewScorer(pool *pgxpool.Pool, logger *slog.Logger) *Scorer {
	return &Scorer{pool: pool, logger: logger}
}

// ScoreAll calculates flight risk scores for all active employees in the given company.
func (s *Scorer) ScoreAll(ctx context.Context, companyID int64) ([]EmployeeRisk, error) {
	// Load all active employees
	employees, err := s.loadActiveEmployees(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("load active employees: %w", err)
	}
	if len(employees) == 0 {
		return nil, nil
	}

	// Load risk signals via SQL
	attendanceRisk, err := s.loadAttendanceDeteriorations(ctx, companyID)
	if err != nil {
		s.logger.Warn("failed to load attendance deteriorations", "error", err)
		attendanceRisk = map[int64]attendanceData{}
	}

	leaveRisk, err := s.loadLeaveExhaustion(ctx, companyID)
	if err != nil {
		s.logger.Warn("failed to load leave exhaustion", "error", err)
		leaveRisk = map[int64]leaveData{}
	}

	salaryStagnant, err := s.loadSalaryStagnation(ctx, companyID)
	if err != nil {
		s.logger.Warn("failed to load salary stagnation", "error", err)
		salaryStagnant = map[int64]time.Time{}
	}

	deptTurnover, err := s.loadDepartmentTurnover(ctx, companyID)
	if err != nil {
		s.logger.Warn("failed to load department turnover", "error", err)
		deptTurnover = map[int64]int64{}
	}

	// Compute risk scores
	now := time.Now()
	var results []EmployeeRisk

	for _, emp := range employees {
		var factors []RiskFactor
		score := 0

		// 1. Attendance deterioration (+20)
		if ad, ok := attendanceRisk[emp.id]; ok {
			// Compare recent 30-day late rate vs previous 60-day late rate
			// older covers 60 days (day 31..90), recent covers 30 days (day 1..30)
			// Normalize: recent rate = recent/30, older rate = older/60
			// If recent rate > 1.3 * older rate → flag
			if ad.older > 0 {
				recentRate := float64(ad.recent) / 30.0
				olderRate := float64(ad.older) / 60.0
				if recentRate > 1.3*olderRate {
					factors = append(factors, RiskFactor{
						Factor: "attendance_deterioration",
						Points: 20,
						Detail: fmt.Sprintf("Late %d times in last 30 days vs %d in previous 60 days", ad.recent, ad.older),
					})
					score += 20
				}
			} else if ad.recent >= 3 {
				// No prior history but frequent recent lates
				factors = append(factors, RiskFactor{
					Factor: "attendance_deterioration",
					Points: 20,
					Detail: fmt.Sprintf("Late %d times in last 30 days with no prior pattern", ad.recent),
				})
				score += 20
			}
		}

		// 2. Leave exhaustion (+15)
		if ld, ok := leaveRisk[emp.id]; ok {
			if ld.totalEarned > 0 && now.Month() < 10 {
				ratio := ld.totalUsed / ld.totalEarned
				if ratio > 0.8 {
					factors = append(factors, RiskFactor{
						Factor: "leave_exhaustion",
						Points: 15,
						Detail: fmt.Sprintf("Used %.0f%% of leave balance before October", ratio*100),
					})
					score += 15
				}
			}
		}

		// 3. Salary stagnation (+15)
		if lastChange, ok := salaryStagnant[emp.id]; ok {
			monthsSince := int(now.Sub(lastChange).Hours() / 24 / 30)
			factors = append(factors, RiskFactor{
				Factor: "salary_stagnation",
				Points: 15,
				Detail: fmt.Sprintf("No salary change in %d months (last: %s)", monthsSince, lastChange.Format("2006-01-02")),
			})
			score += 15
		}

		// 4. High-risk tenure (+15)
		if !emp.hireDate.IsZero() {
			months := int(now.Sub(emp.hireDate).Hours() / 24 / 30)
			if (months >= 11 && months <= 14) || (months >= 23 && months <= 26) {
				factors = append(factors, RiskFactor{
					Factor: "high_risk_tenure",
					Points: 15,
					Detail: fmt.Sprintf("Tenure at %d months (common departure window)", months),
				})
				score += 15
			}
		}

		// 5. Department turnover (+15)
		if emp.departmentID > 0 {
			if sepCount, ok := deptTurnover[emp.departmentID]; ok && sepCount >= 3 {
				factors = append(factors, RiskFactor{
					Factor: "department_turnover",
					Points: 15,
					Detail: fmt.Sprintf("%d separations in department in last 6 months", sepCount),
				})
				score += 15
			}
		}

		// Cap at 100
		if score > 100 {
			score = 100
		}

		if score > 0 {
			results = append(results, EmployeeRisk{
				EmployeeID: emp.id,
				EmployeeNo: emp.employeeNo,
				Name:       emp.firstName + " " + emp.lastName,
				Department: emp.department,
				RiskScore:  score,
				Factors:    factors,
			})
		}
	}

	return results, nil
}

// UpsertScores persists calculated risk scores into employee_risk_scores table.
func (s *Scorer) UpsertScores(ctx context.Context, companyID int64, risks []EmployeeRisk) error {
	for _, r := range risks {
		factorsJSON, err := json.Marshal(r.Factors)
		if err != nil {
			s.logger.Error("failed to marshal factors", "employee_id", r.EmployeeID, "error", err)
			continue
		}

		_, err = s.pool.Exec(ctx, `
			INSERT INTO employee_risk_scores (company_id, employee_id, risk_score, factors, calculated_at)
			VALUES ($1, $2, $3, $4, NOW())
			ON CONFLICT (company_id, employee_id)
			DO UPDATE SET risk_score = $3, factors = $4, calculated_at = NOW()
		`, companyID, r.EmployeeID, r.RiskScore, factorsJSON)
		if err != nil {
			s.logger.Error("failed to upsert risk score",
				"employee_id", r.EmployeeID,
				"error", err,
			)
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
	hireDate     time.Time
}

type attendanceData struct {
	recent int64
	older  int64
}

type leaveData struct {
	totalUsed   float64
	totalEarned float64
}

// -- data loaders --

func (s *Scorer) loadActiveEmployees(ctx context.Context, companyID int64) ([]employeeInfo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.employee_no, e.first_name, e.last_name,
		       COALESCE(d.name, '') as department,
		       COALESCE(e.department_id, 0) as department_id,
		       e.hire_date
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
		if err := rows.Scan(
			&emp.id, &emp.employeeNo, &emp.firstName, &emp.lastName,
			&emp.department, &emp.departmentID, &emp.hireDate,
		); err != nil {
			return nil, fmt.Errorf("scan employee: %w", err)
		}
		result = append(result, emp)
	}
	return result, rows.Err()
}

func (s *Scorer) loadAttendanceDeteriorations(ctx context.Context, companyID int64) (map[int64]attendanceData, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT employee_id,
		       COUNT(*) FILTER (WHERE late_minutes > 0 AND clock_in_at >= NOW() - INTERVAL '30 days') as recent,
		       COUNT(*) FILTER (WHERE late_minutes > 0 AND clock_in_at >= NOW() - INTERVAL '90 days' AND clock_in_at < NOW() - INTERVAL '30 days') as older
		FROM attendance_records
		WHERE company_id = $1
		GROUP BY employee_id
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]attendanceData)
	for rows.Next() {
		var empID, recent, older int64
		if err := rows.Scan(&empID, &recent, &older); err != nil {
			return nil, err
		}
		result[empID] = attendanceData{recent: recent, older: older}
	}
	return result, rows.Err()
}

func (s *Scorer) loadLeaveExhaustion(ctx context.Context, companyID int64) (map[int64]leaveData, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT employee_id,
		       SUM(used::float) as total_used,
		       SUM(earned::float) as total_earned
		FROM leave_balances
		WHERE company_id = $1 AND year = EXTRACT(YEAR FROM NOW())::int
		GROUP BY employee_id
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]leaveData)
	for rows.Next() {
		var empID int64
		var used, earned float64
		if err := rows.Scan(&empID, &used, &earned); err != nil {
			return nil, err
		}
		result[empID] = leaveData{totalUsed: used, totalEarned: earned}
	}
	return result, rows.Err()
}

func (s *Scorer) loadSalaryStagnation(ctx context.Context, companyID int64) (map[int64]time.Time, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT employee_id, MAX(effective_from) as last_change
		FROM employee_salaries
		WHERE company_id = $1
		GROUP BY employee_id
		HAVING MAX(effective_from) < NOW() - INTERVAL '18 months'
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]time.Time)
	for rows.Next() {
		var empID int64
		var lastChange time.Time
		if err := rows.Scan(&empID, &lastChange); err != nil {
			return nil, err
		}
		result[empID] = lastChange
	}
	return result, rows.Err()
}

func (s *Scorer) loadDepartmentTurnover(ctx context.Context, companyID int64) (map[int64]int64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT department_id, COUNT(*) as sep_count
		FROM employees
		WHERE company_id = $1
		  AND status = 'separated'
		  AND updated_at >= NOW() - INTERVAL '6 months'
		  AND department_id IS NOT NULL
		GROUP BY department_id
		HAVING COUNT(*) >= 3
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]int64)
	for rows.Next() {
		var deptID, count int64
		if err := rows.Scan(&deptID, &count); err != nil {
			return nil, err
		}
		result[deptID] = count
	}
	return result, rows.Err()
}
