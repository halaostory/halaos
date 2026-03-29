package workflow

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/store"
)

// TriggerDispatcher routes events to matching triggers and executes actions.
type TriggerDispatcher struct {
	queries   *store.Queries
	evaluator *AgentEvaluator // nil if AI not enabled
	engine    *Engine
	pool      *pgxpool.Pool
	logger    *slog.Logger
}

// NewTriggerDispatcher creates a new trigger dispatcher.
func NewTriggerDispatcher(queries *store.Queries, evaluator *AgentEvaluator, engine *Engine, pool *pgxpool.Pool, logger *slog.Logger) *TriggerDispatcher {
	return &TriggerDispatcher{
		queries:   queries,
		evaluator: evaluator,
		engine:    engine,
		pool:      pool,
		logger:    logger,
	}
}

// DispatchForEvent finds and executes matching triggers for an event.
func (td *TriggerDispatcher) DispatchForEvent(ctx context.Context, companyID int64, triggerType, entityType string, entityID int64) error {
	triggers, err := td.queries.ListActiveTriggersForEvent(ctx, store.ListActiveTriggersForEventParams{
		CompanyID:   companyID,
		TriggerType: triggerType,
		EntityType:  entityType,
	})
	if err != nil {
		return fmt.Errorf("list triggers: %w", err)
	}

	if len(triggers) == 0 {
		td.logger.Debug("no active triggers found",
			"company_id", companyID,
			"trigger_type", triggerType,
			"entity_type", entityType,
		)
		return nil
	}

	for _, trigger := range triggers {
		td.logger.Info("executing trigger",
			"trigger_id", trigger.ID,
			"trigger_name", trigger.Name,
			"action_type", trigger.ActionType,
			"entity_type", entityType,
			"entity_id", entityID,
		)

		if err := td.executeTrigger(ctx, companyID, trigger, entityType, entityID); err != nil {
			td.logger.Error("trigger execution failed",
				"trigger_id", trigger.ID,
				"error", err,
			)
			// Continue with other triggers
		}
	}

	return nil
}

func (td *TriggerDispatcher) executeTrigger(ctx context.Context, companyID int64, trigger store.WorkflowTrigger, entityType string, entityID int64) error {
	switch trigger.ActionType {
	case "run_rules_then_agent":
		return td.runRulesThenAgent(ctx, companyID, trigger, entityType, entityID)

	case "auto_approve":
		return td.executeDirectAction(ctx, companyID, entityType, entityID, "auto_approved")

	case "auto_reject":
		return td.executeDirectAction(ctx, companyID, entityType, entityID, "auto_rejected")

	case "notify":
		td.logger.Info("trigger notify action (no-op for now)",
			"trigger_id", trigger.ID,
			"entity_type", entityType,
			"entity_id", entityID,
		)
		return nil

	default:
		return fmt.Errorf("unknown action_type: %s", trigger.ActionType)
	}
}

func (td *TriggerDispatcher) runRulesThenAgent(ctx context.Context, companyID int64, trigger store.WorkflowTrigger, entityType string, entityID int64) error {
	// Prevent duplicate evaluation
	hasDec, err := td.queries.HasDecisionForEntity(ctx, store.HasDecisionForEntityParams{
		EntityType: entityType,
		EntityID:   entityID,
	})
	if err != nil {
		return fmt.Errorf("check existing decision: %w", err)
	}
	if hasDec {
		td.logger.Info("skipping: decision already exists",
			"entity_type", entityType,
			"entity_id", entityID,
		)
		return nil
	}

	switch entityType {
	case "leave_request":
		return td.runRulesThenAgentLeave(ctx, companyID, trigger, entityID)
	case "overtime_request":
		return td.runRulesThenAgentOT(ctx, companyID, trigger, entityID)
	default:
		return fmt.Errorf("unsupported entity_type for rules+agent: %s", entityType)
	}
}

func (td *TriggerDispatcher) runRulesThenAgentLeave(ctx context.Context, companyID int64, trigger store.WorkflowTrigger, entityID int64) error {
	// Fetch the leave request
	reqs, err := td.queries.ListPendingLeaveRequestsForAutoApproval(ctx, companyID)
	if err != nil {
		return fmt.Errorf("list pending leaves: %w", err)
	}

	var req *store.ListPendingLeaveRequestsForAutoApprovalRow
	for i := range reqs {
		if reqs[i].ID == entityID {
			req = &reqs[i]
			break
		}
	}
	if req == nil {
		td.logger.Info("leave request not found or not pending",
			"entity_id", entityID,
		)
		return nil
	}

	// Step 1: Try rules engine
	result, err := td.engine.EvaluateLeaveRequest(ctx, companyID, *req)
	if err != nil {
		td.logger.Error("rules evaluation failed, falling through to agent",
			"entity_id", entityID,
			"error", err,
		)
	}

	if err == nil && result.Matched {
		// Rule matched — execute and log
		if result.Action == "auto_approved" {
			if execErr := executeLeaveAutoApprovalFromDispatcher(ctx, td.queries, td.pool, companyID, *req, result, td.logger); execErr != nil {
				return fmt.Errorf("execute rule action: %w", execErr)
			}
		}

		reason := result.Reason
		_, _ = td.queries.InsertRuleExecution(ctx, store.InsertRuleExecutionParams{
			CompanyID:           companyID,
			RuleID:              result.RuleID,
			EntityType:          "leave_request",
			EntityID:            entityID,
			Action:              result.Action,
			Reason:              &reason,
			EvaluatedConditions: result.Conditions,
		})

		td.logger.Info("leave request handled by rules engine",
			"entity_id", entityID,
			"rule", result.RuleName,
			"action", result.Action,
		)
		return nil
	}

	// Step 2: No rule matched — try AI agent
	if td.evaluator == nil {
		td.logger.Info("no matching rule and AI not enabled, skipping",
			"entity_id", entityID,
		)
		return nil
	}

	triggerID := trigger.ID
	decision, output, err := td.evaluator.EvaluateLeave(ctx, companyID, &triggerID, *req)
	if err != nil {
		return fmt.Errorf("agent evaluation: %w", err)
	}

	return routeDecision(ctx, td.queries, td.pool, companyID, decision, output, "leave_request", entityID, *req, trigger, td.logger)
}

func (td *TriggerDispatcher) runRulesThenAgentOT(ctx context.Context, companyID int64, trigger store.WorkflowTrigger, entityID int64) error {
	reqs, err := td.queries.ListPendingOTRequestsForAutoApproval(ctx, companyID)
	if err != nil {
		return fmt.Errorf("list pending OT: %w", err)
	}

	var req *store.ListPendingOTRequestsForAutoApprovalRow
	for i := range reqs {
		if reqs[i].ID == entityID {
			req = &reqs[i]
			break
		}
	}
	if req == nil {
		td.logger.Info("OT request not found or not pending",
			"entity_id", entityID,
		)
		return nil
	}

	// Try rules
	result, err := td.engine.EvaluateOTRequest(ctx, companyID, *req)
	if err != nil {
		td.logger.Error("OT rules evaluation failed", "entity_id", entityID, "error", err)
	}

	if err == nil && result.Matched {
		if result.Action == "auto_approved" {
			if execErr := executeOTAutoApprovalFromDispatcher(ctx, td.queries, companyID, *req, result, td.logger); execErr != nil {
				return fmt.Errorf("execute OT rule action: %w", execErr)
			}
		}

		reason := result.Reason
		_, _ = td.queries.InsertRuleExecution(ctx, store.InsertRuleExecutionParams{
			CompanyID:           companyID,
			RuleID:              result.RuleID,
			EntityType:          "overtime_request",
			EntityID:            entityID,
			Action:              result.Action,
			Reason:              &reason,
			EvaluatedConditions: result.Conditions,
		})

		td.logger.Info("OT request handled by rules engine",
			"entity_id", entityID,
			"rule", result.RuleName,
		)
		return nil
	}

	// AI agent
	if td.evaluator == nil {
		return nil
	}

	triggerID := trigger.ID
	decision, output, err := td.evaluator.EvaluateOT(ctx, companyID, &triggerID, *req)
	if err != nil {
		return fmt.Errorf("agent OT evaluation: %w", err)
	}

	return routeOTDecision(ctx, td.queries, companyID, decision, output, *req, trigger, td.logger)
}

func (td *TriggerDispatcher) executeDirectAction(ctx context.Context, companyID int64, entityType string, entityID int64, action string) error {
	switch entityType {
	case "leave_request":
		if action == "auto_approved" {
			_, err := td.queries.ApproveLeaveRequest(ctx, store.ApproveLeaveRequestParams{
				ID:        entityID,
				CompanyID: companyID,
			})
			return err
		}
	case "overtime_request":
		if action == "auto_approved" {
			_, err := td.queries.ApproveOvertimeRequest(ctx, store.ApproveOvertimeRequestParams{
				ID:        entityID,
				CompanyID: companyID,
			})
			return err
		}
	}
	return nil
}
