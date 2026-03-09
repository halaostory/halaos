package teamhealth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Scorer calculates team health scores per department.
type Scorer struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// Factor describes one contributing factor to a department's health score.
type Factor struct {
	Name   string `json:"name"`
	Score  int    `json:"score"`
	Detail string `json:"detail"`
}

// DepartmentHealth holds the computed health score for one department.
type DepartmentHealth struct {
	DepartmentID   int64    `json:"department_id"`
	DepartmentName string   `json:"department_name"`
	HealthScore    int      `json:"health_score"`
	Factors        []Factor `json:"factors"`
}

// NewScorer creates a new team health scorer.
func NewScorer(pool *pgxpool.Pool, logger *slog.Logger) *Scorer {
	return &Scorer{pool: pool, logger: logger}
}

// ScoreAll calculates health scores for all departments in the given company.
// Each factor contributes 0-20 points for a max of 100.
func (s *Scorer) ScoreAll(ctx context.Context, companyID int64) ([]DepartmentHealth, error) {
	depts, err := s.loadDepartments(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("load departments: %w", err)
	}
	if len(depts) == 0 {
		return nil, nil
	}

	attendance := s.loadAttendanceRates(ctx, companyID)
	leaveHealth := s.loadLeaveHealth(ctx, companyID)
	overtime := s.loadOvertimeHealth(ctx, companyID)
	turnover := s.loadTurnoverHealth(ctx, companyID)
	grievances := s.loadGrievanceHealth(ctx, companyID)

	return computeScores(depts, attendance, leaveHealth, overtime, turnover, grievances), nil
}

// computeScores calculates health scores from pre-loaded data maps.
func computeScores(
	depts []deptInfo,
	attendance map[int64]attendanceInfo,
	leaveHealth map[int64]float64,
	overtime map[int64]float64,
	turnover map[int64]int64,
	grievances map[int64]int64,
) []DepartmentHealth {
	var results []DepartmentHealth

	for _, dept := range depts {
		var factors []Factor
		total := 0

		// 1. Attendance rate (0-20)
		if att, ok := attendance[dept.id]; ok {
			score := int(att.rate * 20)
			if score > 20 {
				score = 20
			}
			factors = append(factors, Factor{
				Name:   "attendance",
				Score:  score,
				Detail: fmt.Sprintf("%.0f%% on-time rate (%d/%d)", att.rate*100, att.onTime, att.total),
			})
			total += score
		} else {
			factors = append(factors, Factor{Name: "attendance", Score: 15, Detail: "No attendance data"})
			total += 15
		}

		// 2. Leave balance health (0-20)
		if lb, ok := leaveHealth[dept.id]; ok {
			score := int(lb * 20)
			if score > 20 {
				score = 20
			}
			factors = append(factors, Factor{
				Name:   "leave_balance",
				Score:  score,
				Detail: fmt.Sprintf("%.0f%% average leave balance remaining", lb*100),
			})
			total += score
		} else {
			factors = append(factors, Factor{Name: "leave_balance", Score: 15, Detail: "No leave data"})
			total += 15
		}

		// 3. Overtime health (0-20)
		if otHours, ok := overtime[dept.id]; ok {
			score := 20 - int(otHours*2)
			if score < 0 {
				score = 0
			}
			if score > 20 {
				score = 20
			}
			factors = append(factors, Factor{
				Name:   "overtime",
				Score:  score,
				Detail: fmt.Sprintf("%.1f avg OT hours/employee in last 30 days", otHours),
			})
			total += score
		} else {
			factors = append(factors, Factor{Name: "overtime", Score: 18, Detail: "No overtime data"})
			total += 18
		}

		// 4. Turnover health (0-20)
		if sepCount, ok := turnover[dept.id]; ok {
			score := 20
			if sepCount >= 1 {
				score = 15
			}
			if sepCount >= 3 {
				score = 8
			}
			if sepCount >= 5 {
				score = 0
			}
			factors = append(factors, Factor{
				Name:   "turnover",
				Score:  score,
				Detail: fmt.Sprintf("%d separations in last 6 months", sepCount),
			})
			total += score
		} else {
			factors = append(factors, Factor{Name: "turnover", Score: 20, Detail: "No separations"})
			total += 20
		}

		// 5. Grievance health (0-20)
		if gCount, ok := grievances[dept.id]; ok {
			score := 20
			if gCount >= 1 {
				score = 14
			}
			if gCount >= 3 {
				score = 6
			}
			if gCount >= 5 {
				score = 0
			}
			factors = append(factors, Factor{
				Name:   "grievances",
				Score:  score,
				Detail: fmt.Sprintf("%d open grievances", gCount),
			})
			total += score
		} else {
			factors = append(factors, Factor{Name: "grievances", Score: 20, Detail: "No open grievances"})
			total += 20
		}

		if total > 100 {
			total = 100
		}

		results = append(results, DepartmentHealth{
			DepartmentID:   dept.id,
			DepartmentName: dept.name,
			HealthScore:    total,
			Factors:        factors,
		})
	}

	return results
}

// UpsertScores persists calculated health scores into team_health_scores table.
func (s *Scorer) UpsertScores(ctx context.Context, companyID int64, scores []DepartmentHealth) error {
	for _, dh := range scores {
		factorsJSON, err := json.Marshal(dh.Factors)
		if err != nil {
			s.logger.Error("failed to marshal factors", "department_id", dh.DepartmentID, "error", err)
			continue
		}

		_, err = s.pool.Exec(ctx, `
			INSERT INTO team_health_scores (company_id, department_id, department_name, health_score, factors, calculated_at)
			VALUES ($1, $2, $3, $4, $5, NOW())
			ON CONFLICT (company_id, department_id)
			DO UPDATE SET department_name = $3, health_score = $4, factors = $5, calculated_at = NOW()
		`, companyID, dh.DepartmentID, dh.DepartmentName, dh.HealthScore, factorsJSON)
		if err != nil {
			s.logger.Error("failed to upsert team health score",
				"department_id", dh.DepartmentID,
				"error", err,
			)
		}
	}
	return nil
}

// -- internal types --

type deptInfo struct {
	id   int64
	name string
}

type attendanceInfo struct {
	onTime int64
	total  int64
	rate   float64
}

func (s *Scorer) loadDepartments(ctx context.Context, companyID int64) ([]deptInfo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name FROM departments
		WHERE company_id = $1 AND is_active = true
		ORDER BY name
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []deptInfo
	for rows.Next() {
		var d deptInfo
		if err := rows.Scan(&d.id, &d.name); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (s *Scorer) loadAttendanceRates(ctx context.Context, companyID int64) map[int64]attendanceInfo {
	rows, err := s.pool.Query(ctx, `
		SELECT e.department_id,
		       COUNT(*) as total,
		       COUNT(*) FILTER (WHERE ar.late_minutes = 0 OR ar.late_minutes IS NULL) as on_time
		FROM attendance_records ar
		JOIN employees e ON e.id = ar.employee_id
		WHERE ar.company_id = $1
		  AND ar.clock_in_at >= NOW() - INTERVAL '30 days'
		  AND e.department_id IS NOT NULL
		GROUP BY e.department_id
	`, companyID)
	if err != nil {
		s.logger.Warn("team health: failed to load attendance rates", "error", err)
		return map[int64]attendanceInfo{}
	}
	defer rows.Close()

	result := make(map[int64]attendanceInfo)
	for rows.Next() {
		var deptID, total, onTime int64
		if err := rows.Scan(&deptID, &total, &onTime); err != nil {
			continue
		}
		rate := float64(0)
		if total > 0 {
			rate = float64(onTime) / float64(total)
		}
		result[deptID] = attendanceInfo{onTime: onTime, total: total, rate: rate}
	}
	return result
}

func (s *Scorer) loadLeaveHealth(ctx context.Context, companyID int64) map[int64]float64 {
	rows, err := s.pool.Query(ctx, `
		SELECT e.department_id,
		       AVG(CASE WHEN lb.earned::float > 0 THEN (lb.earned::float - lb.used::float) / lb.earned::float ELSE 1 END) as avg_remaining
		FROM leave_balances lb
		JOIN employees e ON e.id = lb.employee_id
		WHERE lb.company_id = $1
		  AND lb.year = EXTRACT(YEAR FROM NOW())::int
		  AND e.department_id IS NOT NULL
		GROUP BY e.department_id
	`, companyID)
	if err != nil {
		s.logger.Warn("team health: failed to load leave health", "error", err)
		return map[int64]float64{}
	}
	defer rows.Close()

	result := make(map[int64]float64)
	for rows.Next() {
		var deptID int64
		var avg float64
		if err := rows.Scan(&deptID, &avg); err != nil {
			continue
		}
		if avg < 0 {
			avg = 0
		}
		result[deptID] = avg
	}
	return result
}

func (s *Scorer) loadOvertimeHealth(ctx context.Context, companyID int64) map[int64]float64 {
	rows, err := s.pool.Query(ctx, `
		SELECT e.department_id,
		       AVG(otr.hours::float) as avg_hours
		FROM overtime_requests otr
		JOIN employees e ON e.id = otr.employee_id
		WHERE otr.company_id = $1
		  AND otr.status = 'approved'
		  AND otr.created_at >= NOW() - INTERVAL '30 days'
		  AND e.department_id IS NOT NULL
		GROUP BY e.department_id
	`, companyID)
	if err != nil {
		s.logger.Warn("team health: failed to load overtime health", "error", err)
		return map[int64]float64{}
	}
	defer rows.Close()

	result := make(map[int64]float64)
	for rows.Next() {
		var deptID int64
		var avg float64
		if err := rows.Scan(&deptID, &avg); err != nil {
			continue
		}
		result[deptID] = avg
	}
	return result
}

func (s *Scorer) loadTurnoverHealth(ctx context.Context, companyID int64) map[int64]int64 {
	rows, err := s.pool.Query(ctx, `
		SELECT department_id, COUNT(*) as sep_count
		FROM employees
		WHERE company_id = $1
		  AND status = 'separated'
		  AND updated_at >= NOW() - INTERVAL '6 months'
		  AND department_id IS NOT NULL
		GROUP BY department_id
	`, companyID)
	if err != nil {
		s.logger.Warn("team health: failed to load turnover", "error", err)
		return map[int64]int64{}
	}
	defer rows.Close()

	result := make(map[int64]int64)
	for rows.Next() {
		var deptID, count int64
		if err := rows.Scan(&deptID, &count); err != nil {
			continue
		}
		result[deptID] = count
	}
	return result
}

func (s *Scorer) loadGrievanceHealth(ctx context.Context, companyID int64) map[int64]int64 {
	rows, err := s.pool.Query(ctx, `
		SELECT e.department_id, COUNT(*) as open_count
		FROM grievances g
		JOIN employees e ON e.id = g.filed_by
		WHERE g.company_id = $1
		  AND g.status IN ('open', 'under_investigation')
		  AND e.department_id IS NOT NULL
		GROUP BY e.department_id
	`, companyID)
	if err != nil {
		s.logger.Warn("team health: failed to load grievances", "error", err)
		return map[int64]int64{}
	}
	defer rows.Close()

	result := make(map[int64]int64)
	for rows.Next() {
		var deptID, count int64
		if err := rows.Scan(&deptID, &count); err != nil {
			continue
		}
		result[deptID] = count
	}
	return result
}
