package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/store"
)

// ProvisioningService creates provisioning jobs from HR events.
type ProvisioningService struct {
	queries *store.Queries
	logger  *slog.Logger
}

// NewProvisioningService creates a provisioning service.
func NewProvisioningService(queries *store.Queries, logger *slog.Logger) *ProvisioningService {
	return &ProvisioningService{queries: queries, logger: logger}
}

// ScheduleFromEvent finds matching templates and creates provisioning jobs.
func (s *ProvisioningService) ScheduleFromEvent(ctx context.Context, ev store.HrEvent) error {
	templates, err := s.queries.ListActiveTemplatesForEvent(ctx, store.ListActiveTemplatesForEventParams{
		CompanyID:    ev.CompanyID,
		EventTrigger: ev.EventType,
	})
	if err != nil {
		return fmt.Errorf("list templates for event: %w", err)
	}

	if len(templates) == 0 {
		s.logger.Info("no provisioning templates match event",
			"event_type", ev.EventType,
			"company_id", ev.CompanyID,
		)
		return nil
	}

	// Parse employee ID from aggregate
	employeeID := ev.AggregateID

	// Parse event payload for employee details
	var payload map[string]any
	if ev.Payload != nil {
		json.Unmarshal(ev.Payload, &payload)
	}

	for _, tmpl := range templates {
		// Check department filter
		if tmpl.FilterDepartmentID != nil {
			empDeptID, _ := payload["department_id"].(float64)
			if int64(empDeptID) != *tmpl.FilterDepartmentID {
				continue
			}
		}

		// Check employment type filter
		if tmpl.FilterEmploymentType != nil && *tmpl.FilterEmploymentType != "" {
			empType, _ := payload["employment_type"].(string)
			if empType != *tmpl.FilterEmploymentType {
				continue
			}
		}

		// Resolve params from template + payload
		resolvedParams := s.resolveParams(tmpl.Params, payload)

		status := "pending"
		if tmpl.RequiresApproval {
			status = "requires_approval"
		}

		job, err := s.queries.CreateProvisioningJob(ctx, store.CreateProvisioningJobParams{
			CompanyID:      ev.CompanyID,
			EmployeeID:     employeeID,
			ConnectionID:   tmpl.ConnectionID,
			TemplateID:     pgtype.UUID{Bytes: tmpl.ID, Valid: true},
			Provider:       tmpl.Provider,
			ActionType:     tmpl.ActionType,
			TriggerEventID: &ev.ID,
			ResolvedParams: resolvedParams,
			Status:         status,
			ScheduledAt:    time.Now(),
		})
		if err != nil {
			s.logger.Error("failed to create provisioning job",
				"template_id", tmpl.ID,
				"employee_id", employeeID,
				"error", err,
			)
			continue
		}

		s.logger.Info("provisioning job created",
			"job_id", job.ID,
			"template_id", tmpl.ID,
			"provider", tmpl.Provider,
			"action", tmpl.ActionType,
			"status", status,
			"employee_id", employeeID,
		)
	}

	return nil
}

func (s *ProvisioningService) resolveParams(templateParams json.RawMessage, eventPayload map[string]any) json.RawMessage {
	var params map[string]any
	json.Unmarshal(templateParams, &params)
	if params == nil {
		params = make(map[string]any)
	}

	// Merge event payload fields into params
	for _, field := range []string{"email", "first_name", "last_name", "department", "position", "employment_type"} {
		if v, ok := eventPayload[field]; ok {
			if _, exists := params[field]; !exists {
				params[field] = v
			}
		}
	}

	resolved, _ := json.Marshal(params)
	return resolved
}

// TemplateIDFromUUID is a helper to convert UUID to pointer.
func TemplateIDFromUUID(id uuid.UUID) *uuid.UUID {
	return &id
}
