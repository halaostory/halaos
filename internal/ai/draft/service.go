package draft

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/store"
)

// WriteTools is the set of tool names that require user confirmation before execution.
var WriteTools = map[string]bool{
	"create_leave_request":         true,
	"create_overtime_request":      true,
	"approve_leave_request":        true,
	"reject_leave_request":         true,
	"approve_overtime_request":     true,
	"reject_overtime_request":      true,
	"create_expense_claim":         true,
	"create_disciplinary_incident": true,
	"create_disciplinary_action":   true,
	"create_tax_filing_record":     true,
	"update_employee_profile":      true,
	"approve_loan":                 true,
	"reject_loan":                  true,
	"update_clearance_item":        true,
	"create_final_pay":             true,
	"create_goal":                  true,
	"approve_benefit_claim":        true,
	"reject_benefit_claim":         true,
	"approve_leave_encashment":     true,
	"create_employee":              true,
	"send_kudos":                   true,
}

// RiskLevel categorizes the potential impact of an action.
var toolRisk = map[string]string{
	"send_kudos":                   "low",
	"create_leave_request":         "low",
	"create_overtime_request":      "low",
	"create_expense_claim":         "low",
	"create_goal":                  "low",
	"approve_leave_request":        "medium",
	"reject_leave_request":         "medium",
	"approve_overtime_request":     "medium",
	"reject_overtime_request":      "medium",
	"approve_benefit_claim":        "medium",
	"reject_benefit_claim":         "medium",
	"approve_leave_encashment":     "medium",
	"approve_loan":                 "medium",
	"reject_loan":                  "medium",
	"update_employee_profile":      "medium",
	"create_disciplinary_incident": "high",
	"create_disciplinary_action":   "high",
	"create_tax_filing_record":     "high",
	"update_clearance_item":        "high",
	"create_final_pay":             "high",
	"create_employee":              "high",
}

// DraftResult is returned to the LLM when a write tool creates a draft.
type DraftResult struct {
	DraftID     string `json:"draft_id"`
	Status      string `json:"status"`
	ToolName    string `json:"tool_name"`
	RiskLevel   string `json:"risk_level"`
	Description string `json:"description"`
	Message     string `json:"message"`
}

// Service manages the lifecycle of action drafts.
type Service struct {
	queries *store.Queries
	logger  *slog.Logger
}

// NewService creates a draft service.
func NewService(queries *store.Queries, logger *slog.Logger) *Service {
	return &Service{queries: queries, logger: logger}
}

// IsWriteTool returns true if the tool requires confirmation.
func IsWriteTool(name string) bool {
	return WriteTools[name]
}

// RiskLevelFor returns the risk level for a tool.
func RiskLevelFor(toolName string) string {
	if r, ok := toolRisk[toolName]; ok {
		return r
	}
	return "medium"
}

// CreateDraft creates a pending action draft for a write tool invocation.
func (s *Service) CreateDraft(ctx context.Context, companyID, userID int64, sessionID string, toolName string, toolInput map[string]any) (*store.ActionDraft, error) {
	inputJSON, err := json.Marshal(toolInput)
	if err != nil {
		return nil, fmt.Errorf("marshal tool input: %w", err)
	}

	var sid pgtype.UUID
	if sessionID != "" {
		parsed, err := uuid.Parse(sessionID)
		if err == nil {
			sid = pgtype.UUID{Bytes: parsed, Valid: true}
		}
	}

	description := buildDescription(toolName, toolInput)

	draft, err := s.queries.CreateActionDraft(ctx, store.CreateActionDraftParams{
		CompanyID:   companyID,
		UserID:      userID,
		SessionID:   sid,
		ToolName:    toolName,
		ToolInput:   inputJSON,
		RiskLevel:   RiskLevelFor(toolName),
		Description: description,
	})
	if err != nil {
		return nil, fmt.Errorf("create draft: %w", err)
	}

	s.logger.Info("action draft created",
		"draft_id", draft.ID,
		"tool", toolName,
		"risk", draft.RiskLevel,
		"company_id", companyID,
		"user_id", userID,
	)

	return &draft, nil
}

// Confirm marks a draft as confirmed and returns it for execution.
func (s *Service) Confirm(ctx context.Context, companyID, userID int64, draftID uuid.UUID) (*store.ActionDraft, error) {
	draft, err := s.queries.ConfirmActionDraft(ctx, store.ConfirmActionDraftParams{
		ID:        draftID,
		CompanyID: companyID,
		UserID:    userID,
	})
	if err != nil {
		return nil, fmt.Errorf("confirm draft: %w", err)
	}
	return &draft, nil
}

// Reject marks a draft as rejected.
func (s *Service) Reject(ctx context.Context, companyID, userID int64, draftID uuid.UUID) error {
	return s.queries.RejectActionDraft(ctx, store.RejectActionDraftParams{
		ID:        draftID,
		CompanyID: companyID,
		UserID:    userID,
	})
}

// MarkExecuted records the result of a confirmed draft execution.
func (s *Service) MarkExecuted(ctx context.Context, draftID uuid.UUID, result string) error {
	resultJSON, _ := json.Marshal(map[string]string{"result": result})
	return s.queries.ExecuteActionDraft(ctx, store.ExecuteActionDraftParams{
		ID:     draftID,
		Result: resultJSON,
	})
}

// MarkFailed records an execution failure.
func (s *Service) MarkFailed(ctx context.Context, draftID uuid.UUID, errMsg string) error {
	return s.queries.FailActionDraft(ctx, store.FailActionDraftParams{
		ID:           draftID,
		ErrorMessage: &errMsg,
	})
}

// Get retrieves a draft by ID.
func (s *Service) Get(ctx context.Context, companyID, userID int64, draftID uuid.UUID) (*store.ActionDraft, error) {
	draft, err := s.queries.GetActionDraft(ctx, store.GetActionDraftParams{
		ID:        draftID,
		CompanyID: companyID,
		UserID:    userID,
	})
	if err != nil {
		return nil, err
	}
	return &draft, nil
}

// ListPending returns all pending drafts for a user.
func (s *Service) ListPending(ctx context.Context, companyID, userID int64) ([]store.ActionDraft, error) {
	return s.queries.ListPendingDrafts(ctx, store.ListPendingDraftsParams{
		CompanyID: companyID,
		UserID:    userID,
	})
}

// buildDescription generates a human-readable summary of the draft action.
func buildDescription(toolName string, input map[string]any) string {
	switch toolName {
	case "create_leave_request":
		days, _ := input["days"].(float64)
		start, _ := input["start_date"].(string)
		end, _ := input["end_date"].(string)
		return fmt.Sprintf("Create leave request: %.1f days (%s to %s)", days, start, end)
	case "create_overtime_request":
		date, _ := input["ot_date"].(string)
		hours, _ := input["hours"].(float64)
		return fmt.Sprintf("Create overtime request: %.1f hours on %s", hours, date)
	case "approve_leave_request", "reject_leave_request":
		id, _ := input["request_id"].(float64)
		action := "Approve"
		if toolName == "reject_leave_request" {
			action = "Reject"
		}
		return fmt.Sprintf("%s leave request #%d", action, int64(id))
	case "create_expense_claim":
		amount, _ := input["amount"].(float64)
		return fmt.Sprintf("Create expense claim: PHP %.2f", amount)
	case "update_employee_profile":
		return "Update employee profile"
	default:
		return fmt.Sprintf("Execute %s", toolName)
	}
}
