package blindspot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Scorer detects manager blind spots by analyzing team patterns.
type Scorer struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// BlindSpot represents a single detected blind spot for a manager.
type BlindSpot struct {
	ManagerID   int64             `json:"manager_id"`
	ManagerName string            `json:"manager_name"`
	SpotType    string            `json:"spot_type"`
	Severity    string            `json:"severity"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Employees   []AffectedEmployee `json:"employees"`
}

// AffectedEmployee is an employee involved in a blind spot.
type AffectedEmployee struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Detail string `json:"detail"`
}

// NewScorer creates a new blind spot scorer.
func NewScorer(pool *pgxpool.Pool, logger *slog.Logger) *Scorer {
	return &Scorer{pool: pool, logger: logger}
}

// DetectAll finds blind spots for all managers in a company.
func (s *Scorer) DetectAll(ctx context.Context, companyID int64) ([]BlindSpot, error) {
	managers, err := s.loadManagers(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("load managers: %w", err)
	}
	if len(managers) == 0 {
		return nil, nil
	}

	// Load signals in bulk
	tardiness, err := s.loadChronicTardiness(ctx, companyID)
	if err != nil {
		s.logger.Warn("blind spot: failed to load tardiness", "error", err)
		tardiness = map[int64][]tardinessInfo{}
	}

	otConcentration, err := s.loadOvertimeConcentration(ctx, companyID)
	if err != nil {
		s.logger.Warn("blind spot: failed to load OT concentration", "error", err)
		otConcentration = map[int64][]otInfo{}
	}

	leaveImbalance, err := s.loadLeaveImbalance(ctx, companyID)
	if err != nil {
		s.logger.Warn("blind spot: failed to load leave imbalance", "error", err)
		leaveImbalance = map[int64][]leaveInfo{}
	}

	flightRisks, err := s.loadHighFlightRisk(ctx, companyID)
	if err != nil {
		s.logger.Warn("blind spot: failed to load flight risks", "error", err)
		flightRisks = map[int64][]riskInfo{}
	}

	noRecentFeedback, err := s.loadNoFeedback(ctx, companyID)
	if err != nil {
		s.logger.Warn("blind spot: failed to load feedback gaps", "error", err)
		noRecentFeedback = map[int64][]feedbackInfo{}
	}

	var spots []BlindSpot

	for _, mgr := range managers {
		// 1. Chronic tardiness unaddressed
		if emps, ok := tardiness[mgr.id]; ok && len(emps) > 0 {
			affected := make([]AffectedEmployee, 0, len(emps))
			for _, e := range emps {
				affected = append(affected, AffectedEmployee{
					ID:     e.employeeID,
					Name:   e.name,
					Detail: fmt.Sprintf("Late %d times in last 30 days, avg %.0f min", e.lateCount, e.avgMinutes),
				})
			}
			spots = append(spots, BlindSpot{
				ManagerID:   mgr.id,
				ManagerName: mgr.name,
				SpotType:    "chronic_tardiness",
				Severity:    severity(len(affected), 3, 5),
				Title:       fmt.Sprintf("%d team member(s) chronically late", len(affected)),
				Description: fmt.Sprintf("These employees have been consistently late but no disciplinary action or discussion has been recorded."),
				Employees:   affected,
			})
		}

		// 2. Overtime concentration (few people doing most OT)
		if emps, ok := otConcentration[mgr.id]; ok && len(emps) > 0 {
			affected := make([]AffectedEmployee, 0, len(emps))
			for _, e := range emps {
				affected = append(affected, AffectedEmployee{
					ID:     e.employeeID,
					Name:   e.name,
					Detail: fmt.Sprintf("%.1f OT hours in last 30 days (team avg: %.1f)", e.hours, e.teamAvg),
				})
			}
			spots = append(spots, BlindSpot{
				ManagerID:   mgr.id,
				ManagerName: mgr.name,
				SpotType:    "ot_concentration",
				Severity:    "medium",
				Title:       fmt.Sprintf("Overtime concentrated on %d employee(s)", len(affected)),
				Description: "A few team members carry disproportionate overtime load. Consider redistributing work.",
				Employees:   affected,
			})
		}

		// 3. Leave imbalance (some never take leave)
		if emps, ok := leaveImbalance[mgr.id]; ok && len(emps) > 0 {
			affected := make([]AffectedEmployee, 0, len(emps))
			for _, e := range emps {
				affected = append(affected, AffectedEmployee{
					ID:     e.employeeID,
					Name:   e.name,
					Detail: fmt.Sprintf("%.0f days remaining of %.0f earned, only %.0f used this year", e.remaining, e.earned, e.used),
				})
			}
			spots = append(spots, BlindSpot{
				ManagerID:   mgr.id,
				ManagerName: mgr.name,
				SpotType:    "leave_never_taken",
				Severity:    "low",
				Title:       fmt.Sprintf("%d team member(s) rarely use their leave", len(affected)),
				Description: "These employees have used less than 20% of their leave balance. This may indicate workload concerns or burnout risk.",
				Employees:   affected,
			})
		}

		// 4. High flight risk without manager action
		if emps, ok := flightRisks[mgr.id]; ok && len(emps) > 0 {
			affected := make([]AffectedEmployee, 0, len(emps))
			for _, e := range emps {
				affected = append(affected, AffectedEmployee{
					ID:     e.employeeID,
					Name:   e.name,
					Detail: fmt.Sprintf("Flight risk score: %d/100", e.score),
				})
			}
			spots = append(spots, BlindSpot{
				ManagerID:   mgr.id,
				ManagerName: mgr.name,
				SpotType:    "high_flight_risk",
				Severity:    "high",
				Title:       fmt.Sprintf("%d team member(s) have high flight risk", len(affected)),
				Description: "These employees scored 70+ on flight risk but no retention actions have been taken.",
				Employees:   affected,
			})
		}

		// 5. No recent feedback/review
		if emps, ok := noRecentFeedback[mgr.id]; ok && len(emps) > 0 {
			affected := make([]AffectedEmployee, 0, len(emps))
			for _, e := range emps {
				affected = append(affected, AffectedEmployee{
					ID:     e.employeeID,
					Name:   e.name,
					Detail: e.detail,
				})
			}
			spots = append(spots, BlindSpot{
				ManagerID:   mgr.id,
				ManagerName: mgr.name,
				SpotType:    "feedback_gap",
				Severity:    severity(len(affected), 2, 4),
				Title:       fmt.Sprintf("%d team member(s) have no recent performance feedback", len(affected)),
				Description: "These employees haven't received a performance review in the current cycle. Regular feedback improves engagement.",
				Employees:   affected,
			})
		}
	}

	return spots, nil
}

// UpsertSpots persists detected blind spots.
func (s *Scorer) UpsertSpots(ctx context.Context, companyID int64, spots []BlindSpot, weekDate time.Time) error {
	for _, spot := range spots {
		empsJSON, err := json.Marshal(spot.Employees)
		if err != nil {
			continue
		}

		_, err = s.pool.Exec(ctx, `
			INSERT INTO manager_blind_spots (company_id, manager_id, spot_type, severity, title, description, employees, week_date)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, companyID, spot.ManagerID, spot.SpotType, spot.Severity, spot.Title, spot.Description, empsJSON, weekDate)
		if err != nil {
			s.logger.Error("failed to insert blind spot",
				"manager_id", spot.ManagerID,
				"type", spot.SpotType,
				"error", err,
			)
		}
	}
	return nil
}

// severity returns low/medium/high based on thresholds.
func severity(count, mediumThreshold, highThreshold int) string {
	switch {
	case count >= highThreshold:
		return "high"
	case count >= mediumThreshold:
		return "medium"
	default:
		return "low"
	}
}

// -- internal types --

type managerInfo struct {
	id     int64
	name   string
	userID *int64
}

type tardinessInfo struct {
	employeeID int64
	name       string
	lateCount  int64
	avgMinutes float64
}

type otInfo struct {
	employeeID int64
	name       string
	hours      float64
	teamAvg    float64
}

type leaveInfo struct {
	employeeID int64
	name       string
	earned     float64
	used       float64
	remaining  float64
}

type riskInfo struct {
	employeeID int64
	name       string
	score      int
}

type feedbackInfo struct {
	employeeID int64
	name       string
	detail     string
}

// -- data loaders --

func (s *Scorer) loadManagers(ctx context.Context, companyID int64) ([]managerInfo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT e.id, e.first_name || ' ' || e.last_name as name, e.user_id
		FROM employees e
		WHERE e.company_id = $1
		  AND e.status = 'active'
		  AND EXISTS (
		    SELECT 1 FROM employees sub
		    WHERE sub.manager_id = e.id AND sub.company_id = $1 AND sub.status = 'active'
		  )
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []managerInfo
	for rows.Next() {
		var m managerInfo
		if err := rows.Scan(&m.id, &m.name, &m.userID); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// loadChronicTardiness finds employees who are late 5+ times in 30 days, grouped by manager.
func (s *Scorer) loadChronicTardiness(ctx context.Context, companyID int64) (map[int64][]tardinessInfo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.manager_id,
		       e.id as employee_id,
		       e.first_name || ' ' || e.last_name as name,
		       COUNT(*) as late_count,
		       AVG(ar.late_minutes) as avg_minutes
		FROM attendance_records ar
		JOIN employees e ON e.id = ar.employee_id AND e.company_id = ar.company_id
		WHERE ar.company_id = $1
		  AND ar.late_minutes > 0
		  AND ar.clock_in_at >= NOW() - INTERVAL '30 days'
		  AND e.manager_id IS NOT NULL
		  AND e.status = 'active'
		GROUP BY e.manager_id, e.id, e.first_name, e.last_name
		HAVING COUNT(*) >= 5
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]tardinessInfo)
	for rows.Next() {
		var mgrID int64
		var t tardinessInfo
		if err := rows.Scan(&mgrID, &t.employeeID, &t.name, &t.lateCount, &t.avgMinutes); err != nil {
			return nil, err
		}
		result[mgrID] = append(result[mgrID], t)
	}
	return result, rows.Err()
}

// loadOvertimeConcentration finds employees doing 2x+ team average OT.
func (s *Scorer) loadOvertimeConcentration(ctx context.Context, companyID int64) (map[int64][]otInfo, error) {
	rows, err := s.pool.Query(ctx, `
		WITH team_ot AS (
			SELECT e.manager_id,
			       e.id as employee_id,
			       e.first_name || ' ' || e.last_name as name,
			       COALESCE(SUM(otr.hours), 0) as total_hours
			FROM employees e
			LEFT JOIN overtime_requests otr ON otr.employee_id = e.id
			  AND otr.company_id = e.company_id
			  AND otr.status = 'approved'
			  AND otr.ot_date >= NOW() - INTERVAL '30 days'
			WHERE e.company_id = $1
			  AND e.manager_id IS NOT NULL
			  AND e.status = 'active'
			GROUP BY e.manager_id, e.id, e.first_name, e.last_name
		),
		team_avg AS (
			SELECT manager_id, AVG(total_hours) as avg_hours
			FROM team_ot
			GROUP BY manager_id
		)
		SELECT t.manager_id, t.employee_id, t.name, t.total_hours, ta.avg_hours
		FROM team_ot t
		JOIN team_avg ta ON ta.manager_id = t.manager_id
		WHERE t.total_hours > 0
		  AND ta.avg_hours > 0
		  AND t.total_hours >= ta.avg_hours * 2
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]otInfo)
	for rows.Next() {
		var mgrID int64
		var o otInfo
		if err := rows.Scan(&mgrID, &o.employeeID, &o.name, &o.hours, &o.teamAvg); err != nil {
			return nil, err
		}
		result[mgrID] = append(result[mgrID], o)
	}
	return result, rows.Err()
}

// loadLeaveImbalance finds employees who used <20% of earned leave (tenured 6+ months).
func (s *Scorer) loadLeaveImbalance(ctx context.Context, companyID int64) (map[int64][]leaveInfo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.manager_id,
		       e.id as employee_id,
		       e.first_name || ' ' || e.last_name as name,
		       SUM(lb.earned::float) as total_earned,
		       SUM(lb.used::float) as total_used,
		       SUM((lb.earned - lb.used)::float) as remaining
		FROM leave_balances lb
		JOIN employees e ON e.id = lb.employee_id AND e.company_id = lb.company_id
		WHERE lb.company_id = $1
		  AND lb.year = EXTRACT(YEAR FROM NOW())::int
		  AND e.manager_id IS NOT NULL
		  AND e.status = 'active'
		  AND e.hire_date <= NOW() - INTERVAL '6 months'
		GROUP BY e.manager_id, e.id, e.first_name, e.last_name
		HAVING SUM(lb.earned::float) >= 5
		   AND SUM(lb.used::float) / NULLIF(SUM(lb.earned::float), 0) < 0.2
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]leaveInfo)
	for rows.Next() {
		var mgrID int64
		var l leaveInfo
		if err := rows.Scan(&mgrID, &l.employeeID, &l.name, &l.earned, &l.used, &l.remaining); err != nil {
			return nil, err
		}
		result[mgrID] = append(result[mgrID], l)
	}
	return result, rows.Err()
}

// loadHighFlightRisk finds employees with risk score >= 70, grouped by manager.
func (s *Scorer) loadHighFlightRisk(ctx context.Context, companyID int64) (map[int64][]riskInfo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.manager_id,
		       e.id as employee_id,
		       e.first_name || ' ' || e.last_name as name,
		       ers.risk_score
		FROM employee_risk_scores ers
		JOIN employees e ON e.id = ers.employee_id AND e.company_id = ers.company_id
		WHERE ers.company_id = $1
		  AND ers.risk_score >= 70
		  AND e.manager_id IS NOT NULL
		  AND e.status = 'active'
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]riskInfo)
	for rows.Next() {
		var mgrID int64
		var r riskInfo
		if err := rows.Scan(&mgrID, &r.employeeID, &r.name, &r.score); err != nil {
			return nil, err
		}
		result[mgrID] = append(result[mgrID], r)
	}
	return result, rows.Err()
}

// loadNoFeedback finds employees with no performance review in the current year.
func (s *Scorer) loadNoFeedback(ctx context.Context, companyID int64) (map[int64][]feedbackInfo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.manager_id,
		       e.id as employee_id,
		       e.first_name || ' ' || e.last_name as name,
		       e.hire_date
		FROM employees e
		WHERE e.company_id = $1
		  AND e.manager_id IS NOT NULL
		  AND e.status = 'active'
		  AND e.hire_date <= NOW() - INTERVAL '3 months'
		  AND NOT EXISTS (
		    SELECT 1 FROM performance_reviews pr
		    WHERE pr.employee_id = e.id
		      AND pr.company_id = e.company_id
		      AND pr.created_at >= date_trunc('year', NOW())
		  )
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]feedbackInfo)
	for rows.Next() {
		var mgrID int64
		var empID int64
		var name string
		var hireDate time.Time
		if err := rows.Scan(&mgrID, &empID, &name, &hireDate); err != nil {
			return nil, err
		}
		months := int(time.Since(hireDate).Hours() / 24 / 30)
		result[mgrID] = append(result[mgrID], feedbackInfo{
			employeeID: empID,
			name:       name,
			detail:     fmt.Sprintf("Hired %d months ago, no review this year", months),
		})
	}
	return result, rows.Err()
}
