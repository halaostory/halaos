package integration

import "time"

// Event type constants for AI Management Brain integration.
const (
	BrainEventRiskUpdated       = "hr.risk.updated"
	BrainEventBurnoutUpdated    = "hr.burnout.updated"
	BrainEventTeamHealthUpdated = "hr.team_health.updated"
	BrainEventBlindspotDetected = "hr.blindspot.detected"
	BrainEventAttendanceAnomaly = "hr.attendance.anomaly"
	BrainEventOrgSnapshot       = "hr.org_snapshot.weekly"
)

// EventFactor is a scored contributing factor within a risk or health event.
type EventFactor struct {
	Factor string `json:"factor"`
	Points int    `json:"points"`
	Detail string `json:"detail"`
}

// BlindspotEmployee is an employee referenced in a blindspot detection event.
type BlindspotEmployee struct {
	ID         int64  `json:"id"`
	EmployeeNo string `json:"employee_no"`
	Name       string `json:"name"`
	Detail     string `json:"detail"`
}

// AttendanceAnomalyItem is a single anomalous attendance record.
type AttendanceAnomalyItem struct {
	EmployeeID int64  `json:"employee_id"`
	EmployeeNo string `json:"employee_no"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Detail     string `json:"detail"`
}

// RiskUpdatedEvent is emitted when an employee's flight-risk score changes.
type RiskUpdatedEvent struct {
	EventID      string    `json:"event_id"`
	EventType    string    `json:"event_type"`
	EventVersion int       `json:"event_version"`
	OccurredAt   time.Time `json:"occurred_at"`
	HRCompanyID  int64     `json:"hr_company_id"`
	EmployeeID   int64     `json:"employee_id"`
	EmployeeNo   string    `json:"employee_no"`
	Name         string    `json:"name"`
	Department   string    `json:"department"`
	RiskScore    int       `json:"risk_score"`
	Factors      []EventFactor `json:"factors"`
	PrevScore    int       `json:"prev_score"`
}

// BurnoutUpdatedEvent is emitted when an employee's burnout score changes.
type BurnoutUpdatedEvent struct {
	EventID      string    `json:"event_id"`
	EventType    string    `json:"event_type"`
	EventVersion int       `json:"event_version"`
	OccurredAt   time.Time `json:"occurred_at"`
	HRCompanyID  int64     `json:"hr_company_id"`
	EmployeeID   int64     `json:"employee_id"`
	EmployeeNo   string    `json:"employee_no"`
	Name         string    `json:"name"`
	Department   string    `json:"department"`
	BurnoutScore int       `json:"burnout_score"`
	Factors      []EventFactor `json:"factors"`
	PrevScore    int       `json:"prev_score"`
}

// TeamHealthUpdatedEvent is emitted when a department's team health score changes.
type TeamHealthUpdatedEvent struct {
	EventID        string    `json:"event_id"`
	EventType      string    `json:"event_type"`
	EventVersion   int       `json:"event_version"`
	OccurredAt     time.Time `json:"occurred_at"`
	HRCompanyID    int64     `json:"hr_company_id"`
	DepartmentID   int64     `json:"department_id"`
	DepartmentName string    `json:"department_name"`
	HealthScore    int       `json:"health_score"`
	Factors        []EventFactor `json:"factors"`
	PrevScore      int       `json:"prev_score"`
}

// BlindspotDetectedEvent is emitted when an AI analysis detects a managerial blindspot.
type BlindspotDetectedEvent struct {
	EventID      string              `json:"event_id"`
	EventType    string              `json:"event_type"`
	EventVersion int                 `json:"event_version"`
	OccurredAt   time.Time           `json:"occurred_at"`
	HRCompanyID  int64               `json:"hr_company_id"`
	ManagerID    int64               `json:"manager_id"`
	ManagerName  string              `json:"manager_name"`
	SpotType     string              `json:"spot_type"`
	Severity     string              `json:"severity"`
	Title        string              `json:"title"`
	Description  string              `json:"description"`
	Employees    []BlindspotEmployee `json:"employees"`
}

// AttendanceAnomalyEvent is emitted when attendance anomalies are detected for a given date.
type AttendanceAnomalyEvent struct {
	EventID      string                  `json:"event_id"`
	EventType    string                  `json:"event_type"`
	EventVersion int                     `json:"event_version"`
	OccurredAt   time.Time               `json:"occurred_at"`
	HRCompanyID  int64                   `json:"hr_company_id"`
	Date         string                  `json:"date"`
	Anomalies    []AttendanceAnomalyItem `json:"anomalies"`
}

// OrgSnapshotEvent is emitted on the weekly organisational health snapshot.
type OrgSnapshotEvent struct {
	EventID            string    `json:"event_id"`
	EventType          string    `json:"event_type"`
	EventVersion       int       `json:"event_version"`
	OccurredAt         time.Time `json:"occurred_at"`
	HRCompanyID        int64     `json:"hr_company_id"`
	AvgFlightRisk      float64   `json:"avg_flight_risk"`
	AvgBurnout         float64   `json:"avg_burnout"`
	AvgTeamHealth      float64   `json:"avg_team_health"`
	HighRiskCount      int       `json:"high_risk_count"`
	HighBurnoutCount   int       `json:"high_burnout_count"`
	Headcount          int       `json:"headcount"`
	LowHealthDeptCount int       `json:"low_health_dept_count"`
}
