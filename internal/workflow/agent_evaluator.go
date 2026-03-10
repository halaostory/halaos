package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/ai/agent"
	"github.com/tonypk/aigonhr/internal/store"
)

// DecisionOutput represents the AI agent's structured decision.
type DecisionOutput struct {
	Decision   string  `json:"decision"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
}

// AgentEvaluator uses the AI agent pipeline to evaluate workflow requests.
type AgentEvaluator struct {
	executor *agent.Executor
	queries  *store.Queries
	pool     *pgxpool.Pool
	logger   *slog.Logger
}

// NewAgentEvaluator creates a new agent evaluator.
func NewAgentEvaluator(executor *agent.Executor, queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *AgentEvaluator {
	return &AgentEvaluator{
		executor: executor,
		queries:  queries,
		pool:     pool,
		logger:   logger,
	}
}

// EvaluateLeave evaluates a leave request using the AI agent.
func (ae *AgentEvaluator) EvaluateLeave(ctx context.Context, companyID int64, triggerID *int64, req store.ListPendingLeaveRequestsForAutoApprovalRow) (store.WorkflowDecision, DecisionOutput, error) {
	prompt := ae.buildLeavePrompt(ctx, companyID, req)
	return ae.evaluate(ctx, companyID, triggerID, "leave_request", req.ID, prompt)
}

// EvaluateOT evaluates an overtime request using the AI agent.
func (ae *AgentEvaluator) EvaluateOT(ctx context.Context, companyID int64, triggerID *int64, req store.ListPendingOTRequestsForAutoApprovalRow) (store.WorkflowDecision, DecisionOutput, error) {
	prompt := ae.buildOTPrompt(ctx, companyID, req)
	return ae.evaluate(ctx, companyID, triggerID, "overtime_request", req.ID, prompt)
}

func (ae *AgentEvaluator) evaluate(ctx context.Context, companyID int64, triggerID *int64, entityType string, entityID int64, prompt string) (store.WorkflowDecision, DecisionOutput, error) {
	evalCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Call AI agent (userID=0 for system calls)
	resp, err := ae.executor.Chat(evalCtx, companyID, 0, "workflow", agent.ChatRequest{
		Message: prompt,
	})
	if err != nil {
		return store.WorkflowDecision{}, DecisionOutput{}, fmt.Errorf("agent chat: %w", err)
	}

	// Parse structured output
	output, err := parseDecisionOutput(resp.Message)
	if err != nil {
		ae.logger.Warn("failed to parse agent output, defaulting to escalate",
			"entity_type", entityType,
			"entity_id", entityID,
			"raw", resp.Message,
			"error", err,
		)
		output = DecisionOutput{
			Decision:   "escalate",
			Confidence: 0.0,
			Reasoning:  "Failed to parse AI response: " + err.Error(),
		}
	}

	// Build confidence as pgtype.Numeric
	var confidence pgtype.Numeric
	_ = confidence.Scan(fmt.Sprintf("%.3f", output.Confidence))

	// Build context snapshot
	contextSnapshot, _ := json.Marshal(map[string]any{
		"prompt":     prompt,
		"raw_output": resp.Message,
	})

	agentSlug := "workflow"
	reasoning := output.Reasoning

	decision, err := ae.queries.InsertWorkflowDecision(ctx, store.InsertWorkflowDecisionParams{
		CompanyID:       companyID,
		TriggerID:       triggerID,
		EntityType:      entityType,
		EntityID:        entityID,
		Decision:        output.Decision,
		Confidence:      confidence,
		Reasoning:       &reasoning,
		ContextSnapshot: contextSnapshot,
		AiAgentSlug:     &agentSlug,
		TokensUsed:      int32(resp.TokensUsed),
	})
	if err != nil {
		return store.WorkflowDecision{}, output, fmt.Errorf("insert decision: %w", err)
	}

	ae.logger.Info("AI decision recorded",
		"entity_type", entityType,
		"entity_id", entityID,
		"decision", output.Decision,
		"confidence", output.Confidence,
	)

	return decision, output, nil
}

func (ae *AgentEvaluator) buildLeavePrompt(ctx context.Context, companyID int64, req store.ListPendingLeaveRequestsForAutoApprovalRow) string {
	var sb strings.Builder

	days := numericToFloat64(req.Days)

	sb.WriteString("## Leave Request Evaluation\n\n")
	sb.WriteString(fmt.Sprintf("**Employee:** %s %s\n", req.FirstName, req.LastName))
	sb.WriteString(fmt.Sprintf("**Leave Type:** %s (%s)\n", req.LeaveTypeName, req.LeaveTypeCode))
	sb.WriteString(fmt.Sprintf("**Days:** %.1f\n", days))
	sb.WriteString(fmt.Sprintf("**Period:** %s to %s\n", req.StartDate.Format("2006-01-02"), req.EndDate.Format("2006-01-02")))
	if req.Reason != nil {
		sb.WriteString(fmt.Sprintf("**Reason:** %s\n", *req.Reason))
	}
	sb.WriteString(fmt.Sprintf("**Hire Date:** %s\n", req.HireDate.Format("2006-01-02")))
	sb.WriteString("\n")

	// Leave balances
	year := time.Now().Year()
	balances, err := ae.queries.ListLeaveBalances(ctx, store.ListLeaveBalancesParams{
		CompanyID:  companyID,
		EmployeeID: req.EmployeeID,
		Year:       int32(year),
	})
	if err == nil && len(balances) > 0 {
		sb.WriteString("### Leave Balances (Current Year)\n")
		for _, b := range balances {
			earned := numericToFloat64(b.Earned)
			used := numericToFloat64(b.Used)
			remaining := earned - used
			sb.WriteString(fmt.Sprintf("- %s: earned=%.1f, used=%.1f, remaining=%.1f\n",
				b.LeaveTypeName, earned, used, remaining))
		}
		sb.WriteString("\n")
	}

	// Team conflict check
	sb.WriteString("### Team Coverage\n")
	hasConflict := checkDepartmentConflict(ctx, ae.pool, companyID, req)
	if hasConflict {
		sb.WriteString("⚠️ Another team member in the same department has overlapping leave during this period.\n")
	} else {
		sb.WriteString("✅ No overlapping leave requests in the same department.\n")
	}
	sb.WriteString("\n")

	// Recent approval patterns
	ae.appendApprovalPatterns(&sb, ctx, companyID, "leave_request")

	sb.WriteString("\n### Instructions\n")
	sb.WriteString("Respond with ONLY a JSON object: {\"decision\":\"...\",\"confidence\":0.XX,\"reasoning\":\"...\"}\n")

	return sb.String()
}

func (ae *AgentEvaluator) buildOTPrompt(ctx context.Context, companyID int64, req store.ListPendingOTRequestsForAutoApprovalRow) string {
	var sb strings.Builder

	hours := numericToFloat64(req.Hours)

	sb.WriteString("## Overtime Request Evaluation\n\n")
	sb.WriteString(fmt.Sprintf("**Employee:** %s %s\n", req.FirstName, req.LastName))
	sb.WriteString(fmt.Sprintf("**OT Date:** %s\n", req.OtDate.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("**Hours:** %.1f\n", hours))
	sb.WriteString(fmt.Sprintf("**OT Type:** %s\n", req.OtType))
	if req.Reason != nil {
		sb.WriteString(fmt.Sprintf("**Reason:** %s\n", *req.Reason))
	}
	sb.WriteString("\n")

	// Recent approval patterns
	ae.appendApprovalPatterns(&sb, ctx, companyID, "overtime_request")

	sb.WriteString("\n### Instructions\n")
	sb.WriteString("Respond with ONLY a JSON object: {\"decision\":\"...\",\"confidence\":0.XX,\"reasoning\":\"...\"}\n")

	return sb.String()
}

func (ae *AgentEvaluator) appendApprovalPatterns(sb *strings.Builder, ctx context.Context, companyID int64, entityType string) {
	patterns, err := ae.queries.GetRecentApprovalPatterns(ctx, store.GetRecentApprovalPatternsParams{
		CompanyID:  companyID,
		EntityType: entityType,
	})
	if err != nil || len(patterns) == 0 {
		return
	}

	var approved, rejected, overridden int
	for _, p := range patterns {
		switch {
		case strings.HasSuffix(p.Decision, "_approve") || p.Decision == "auto_approve":
			approved++
		case strings.HasSuffix(p.Decision, "_reject") || p.Decision == "auto_reject":
			rejected++
		}
		if p.WasOverridden {
			overridden++
		}
	}

	sb.WriteString("### Recent Decision Patterns (last 50)\n")
	sb.WriteString(fmt.Sprintf("- Approved: %d, Rejected: %d, Overridden by manager: %d\n", approved, rejected, overridden))

	if len(patterns) > 0 {
		overrideRate := float64(overridden) / float64(len(patterns)) * 100
		if overrideRate > 20 {
			sb.WriteString(fmt.Sprintf("⚠️ High override rate (%.0f%%). Managers frequently change AI decisions. Prefer recommend_* over auto_* when uncertain.\n", overrideRate))
		}
	}
}

func parseDecisionOutput(raw string) (DecisionOutput, error) {
	// Try to extract JSON from the response
	raw = strings.TrimSpace(raw)

	// If wrapped in markdown code block, extract
	if idx := strings.Index(raw, "```json"); idx >= 0 {
		raw = raw[idx+7:]
		if end := strings.Index(raw, "```"); end >= 0 {
			raw = raw[:end]
		}
	} else if idx := strings.Index(raw, "```"); idx >= 0 {
		raw = raw[idx+3:]
		if end := strings.Index(raw, "```"); end >= 0 {
			raw = raw[:end]
		}
	}

	// Try to find JSON object
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}

	raw = strings.TrimSpace(raw)

	var output DecisionOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		return DecisionOutput{}, fmt.Errorf("unmarshal decision: %w (raw: %s)", err, raw)
	}

	// Validate decision
	validDecisions := map[string]bool{
		"auto_approve":      true,
		"auto_reject":       true,
		"recommend_approve": true,
		"recommend_reject":  true,
		"escalate":          true,
		"request_info":      true,
	}
	if !validDecisions[output.Decision] {
		return DecisionOutput{}, fmt.Errorf("invalid decision: %s", output.Decision)
	}

	// Clamp confidence
	if output.Confidence < 0 {
		output.Confidence = 0
	}
	if output.Confidence > 1 {
		output.Confidence = 1
	}

	return output, nil
}

func numericToFloat64(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

