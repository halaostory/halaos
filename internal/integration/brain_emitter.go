package integration

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/halaostory/halaos/internal/store"
)

// BrainEmitter builds AI Management Brain events and enqueues them for delivery.
type BrainEmitter struct {
	outbox  *BrainOutbox
	queries *store.Queries
	logger  *slog.Logger
}

// NewBrainEmitter creates a new emitter.
func NewBrainEmitter(queries *store.Queries, logger *slog.Logger) *BrainEmitter {
	return &BrainEmitter{
		outbox:  NewBrainOutbox(queries, logger),
		queries: queries,
		logger:  logger,
	}
}

// hasActiveLink returns true if the company has an active Brain integration link.
func (e *BrainEmitter) hasActiveLink(ctx context.Context, companyID int64) bool {
	_, err := e.queries.GetActiveBrainLink(ctx, companyID)
	return err == nil
}

// EmitRiskUpdated enqueues an hr.risk.updated event for an employee.
func (e *BrainEmitter) EmitRiskUpdated(
	ctx context.Context,
	companyID, employeeID int64,
	employeeNo, name, department string,
	riskScore int,
	factors []EventFactor,
	prevScore int,
) error {
	if !e.hasActiveLink(ctx, companyID) {
		return nil
	}

	event := RiskUpdatedEvent{
		EventID:      uuid.New().String(),
		EventType:    BrainEventRiskUpdated,
		EventVersion: 1,
		OccurredAt:   time.Now().UTC(),
		HRCompanyID:  companyID,
		EmployeeID:   employeeID,
		EmployeeNo:   employeeNo,
		Name:         name,
		Department:   department,
		RiskScore:    riskScore,
		Factors:      factors,
		PrevScore:    prevScore,
	}

	if err := e.outbox.Enqueue(ctx, companyID, BrainEventRiskUpdated, "employee", employeeID, event); err != nil {
		return fmt.Errorf("enqueue hr.risk.updated: %w", err)
	}

	e.logger.Debug("brain hr.risk.updated event emitted",
		"company_id", companyID,
		"employee_id", employeeID,
		"risk_score", riskScore,
	)
	return nil
}

// EmitBurnoutUpdated enqueues an hr.burnout.updated event for an employee.
func (e *BrainEmitter) EmitBurnoutUpdated(
	ctx context.Context,
	companyID, employeeID int64,
	employeeNo, name, department string,
	burnoutScore int,
	factors []EventFactor,
	prevScore int,
) error {
	if !e.hasActiveLink(ctx, companyID) {
		return nil
	}

	event := BurnoutUpdatedEvent{
		EventID:      uuid.New().String(),
		EventType:    BrainEventBurnoutUpdated,
		EventVersion: 1,
		OccurredAt:   time.Now().UTC(),
		HRCompanyID:  companyID,
		EmployeeID:   employeeID,
		EmployeeNo:   employeeNo,
		Name:         name,
		Department:   department,
		BurnoutScore: burnoutScore,
		Factors:      factors,
		PrevScore:    prevScore,
	}

	if err := e.outbox.Enqueue(ctx, companyID, BrainEventBurnoutUpdated, "employee", employeeID, event); err != nil {
		return fmt.Errorf("enqueue hr.burnout.updated: %w", err)
	}

	e.logger.Debug("brain hr.burnout.updated event emitted",
		"company_id", companyID,
		"employee_id", employeeID,
		"burnout_score", burnoutScore,
	)
	return nil
}

// EmitTeamHealthUpdated enqueues an hr.team_health.updated event for a department.
func (e *BrainEmitter) EmitTeamHealthUpdated(
	ctx context.Context,
	companyID, departmentID int64,
	departmentName string,
	healthScore int,
	factors []EventFactor,
	prevScore int,
) error {
	if !e.hasActiveLink(ctx, companyID) {
		return nil
	}

	event := TeamHealthUpdatedEvent{
		EventID:        uuid.New().String(),
		EventType:      BrainEventTeamHealthUpdated,
		EventVersion:   1,
		OccurredAt:     time.Now().UTC(),
		HRCompanyID:    companyID,
		DepartmentID:   departmentID,
		DepartmentName: departmentName,
		HealthScore:    healthScore,
		Factors:        factors,
		PrevScore:      prevScore,
	}

	if err := e.outbox.Enqueue(ctx, companyID, BrainEventTeamHealthUpdated, "department", departmentID, event); err != nil {
		return fmt.Errorf("enqueue hr.team_health.updated: %w", err)
	}

	e.logger.Debug("brain hr.team_health.updated event emitted",
		"company_id", companyID,
		"department_id", departmentID,
		"health_score", healthScore,
	)
	return nil
}

// EmitBlindspotDetected enqueues an hr.blindspot.detected event for a manager.
func (e *BrainEmitter) EmitBlindspotDetected(
	ctx context.Context,
	companyID, managerID int64,
	managerName, spotType, severity, title, description string,
	employees []BlindspotEmployee,
) error {
	if !e.hasActiveLink(ctx, companyID) {
		return nil
	}

	event := BlindspotDetectedEvent{
		EventID:      uuid.New().String(),
		EventType:    BrainEventBlindspotDetected,
		EventVersion: 1,
		OccurredAt:   time.Now().UTC(),
		HRCompanyID:  companyID,
		ManagerID:    managerID,
		ManagerName:  managerName,
		SpotType:     spotType,
		Severity:     severity,
		Title:        title,
		Description:  description,
		Employees:    employees,
	}

	if err := e.outbox.Enqueue(ctx, companyID, BrainEventBlindspotDetected, "manager", managerID, event); err != nil {
		return fmt.Errorf("enqueue hr.blindspot.detected: %w", err)
	}

	e.logger.Debug("brain hr.blindspot.detected event emitted",
		"company_id", companyID,
		"manager_id", managerID,
		"spot_type", spotType,
		"severity", severity,
	)
	return nil
}

// EmitAttendanceAnomaly enqueues an hr.attendance.anomaly event for a given date.
func (e *BrainEmitter) EmitAttendanceAnomaly(
	ctx context.Context,
	companyID int64,
	date string,
	anomalies []AttendanceAnomalyItem,
) error {
	if !e.hasActiveLink(ctx, companyID) {
		return nil
	}

	event := AttendanceAnomalyEvent{
		EventID:      uuid.New().String(),
		EventType:    BrainEventAttendanceAnomaly,
		EventVersion: 1,
		OccurredAt:   time.Now().UTC(),
		HRCompanyID:  companyID,
		Date:         date,
		Anomalies:    anomalies,
	}

	if err := e.outbox.Enqueue(ctx, companyID, BrainEventAttendanceAnomaly, "company", companyID, event); err != nil {
		return fmt.Errorf("enqueue hr.attendance.anomaly: %w", err)
	}

	e.logger.Debug("brain hr.attendance.anomaly event emitted",
		"company_id", companyID,
		"date", date,
		"anomaly_count", len(anomalies),
	)
	return nil
}

// EmitOrgSnapshot enqueues an hr.org_snapshot.weekly event for the company.
func (e *BrainEmitter) EmitOrgSnapshot(
	ctx context.Context,
	companyID int64,
	avgFlightRisk, avgBurnout, avgTeamHealth float64,
	highRiskCount, highBurnoutCount, headcount, lowHealthDeptCount int,
) error {
	if !e.hasActiveLink(ctx, companyID) {
		return nil
	}

	event := OrgSnapshotEvent{
		EventID:            uuid.New().String(),
		EventType:          BrainEventOrgSnapshot,
		EventVersion:       1,
		OccurredAt:         time.Now().UTC(),
		HRCompanyID:        companyID,
		AvgFlightRisk:      avgFlightRisk,
		AvgBurnout:         avgBurnout,
		AvgTeamHealth:      avgTeamHealth,
		HighRiskCount:      highRiskCount,
		HighBurnoutCount:   highBurnoutCount,
		Headcount:          headcount,
		LowHealthDeptCount: lowHealthDeptCount,
	}

	if err := e.outbox.Enqueue(ctx, companyID, BrainEventOrgSnapshot, "company", companyID, event); err != nil {
		return fmt.Errorf("enqueue hr.org_snapshot.weekly: %w", err)
	}

	e.logger.Debug("brain hr.org_snapshot.weekly event emitted",
		"company_id", companyID,
		"headcount", headcount,
		"avg_flight_risk", avgFlightRisk,
		"avg_burnout", avgBurnout,
		"avg_team_health", avgTeamHealth,
	)
	return nil
}
