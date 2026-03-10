package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/store"
)

// EvaluationResult represents the outcome of rule evaluation.
type EvaluationResult struct {
	Matched    bool            `json:"matched"`
	RuleID     int64           `json:"rule_id,omitempty"`
	RuleName   string          `json:"rule_name,omitempty"`
	Action     string          `json:"action,omitempty"` // auto_approved, auto_rejected
	Reason     string          `json:"reason,omitempty"`
	Conditions json.RawMessage `json:"conditions,omitempty"`
}

// Engine evaluates workflow rules against entities.
type Engine struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

// NewEngine creates a new workflow rules engine.
func NewEngine(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Engine {
	return &Engine{queries: queries, pool: pool, logger: logger}
}

// EvaluateLeaveRequest loads active rules and returns the first matching result.
func (e *Engine) EvaluateLeaveRequest(ctx context.Context, companyID int64, req store.ListPendingLeaveRequestsForAutoApprovalRow) (EvaluationResult, error) {
	rules, err := e.queries.ListActiveRulesForEntityType(ctx, store.ListActiveRulesForEntityTypeParams{
		CompanyID:  companyID,
		EntityType: "leave_request",
	})
	if err != nil {
		return EvaluationResult{}, fmt.Errorf("list rules: %w", err)
	}

	for _, rule := range rules {
		matched, reason, evaluated := evaluateLeaveConditions(ctx, e.queries, e.pool, companyID, req, rule.Conditions)
		if matched {
			return EvaluationResult{
				Matched:    true,
				RuleID:     rule.ID,
				RuleName:   rule.Name,
				Action:     rule.RuleType + "d", // auto_approve → auto_approved
				Reason:     reason,
				Conditions: evaluated,
			}, nil
		}
	}

	return EvaluationResult{Matched: false}, nil
}

// EvaluateOTRequest loads active rules and returns the first matching result.
func (e *Engine) EvaluateOTRequest(ctx context.Context, companyID int64, req store.ListPendingOTRequestsForAutoApprovalRow) (EvaluationResult, error) {
	rules, err := e.queries.ListActiveRulesForEntityType(ctx, store.ListActiveRulesForEntityTypeParams{
		CompanyID:  companyID,
		EntityType: "overtime_request",
	})
	if err != nil {
		return EvaluationResult{}, fmt.Errorf("list rules: %w", err)
	}

	for _, rule := range rules {
		matched, reason, evaluated := evaluateOTConditions(req, rule.Conditions)
		if matched {
			return EvaluationResult{
				Matched:    true,
				RuleID:     rule.ID,
				RuleName:   rule.Name,
				Action:     rule.RuleType + "d",
				Reason:     reason,
				Conditions: evaluated,
			}, nil
		}
	}

	return EvaluationResult{Matched: false}, nil
}
