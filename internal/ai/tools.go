package ai

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/store"
)

// ToolRegistry maps tool names to executors.
type ToolRegistry struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	tools   map[string]ToolExecutor
}

// ToolExecutor runs a tool and returns the result as a string.
type ToolExecutor func(ctx context.Context, companyID, userID int64, input map[string]any) (string, error)

// NewToolRegistry creates and registers all HR tools.
func NewToolRegistry(queries *store.Queries, pool *pgxpool.Pool) *ToolRegistry {
	r := &ToolRegistry{
		queries: queries,
		pool:    pool,
		tools:   make(map[string]ToolExecutor),
	}
	r.registerTools()
	return r
}

// DefinitionsForAgent returns tool definitions filtered by the allowed tool names.
func (r *ToolRegistry) DefinitionsForAgent(allowedTools []string) []provider.ToolDefinition {
	if len(allowedTools) == 0 {
		return r.Definitions()
	}
	allowed := make(map[string]bool, len(allowedTools))
	for _, t := range allowedTools {
		allowed[t] = true
	}
	all := r.Definitions()
	filtered := make([]provider.ToolDefinition, 0, len(allowedTools))
	for _, d := range all {
		if allowed[d.Name] {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// Definitions returns tool definitions for the LLM.
func (r *ToolRegistry) Definitions() []provider.ToolDefinition {
	var defs []provider.ToolDefinition
	defs = append(defs, leaveDefs()...)
	defs = append(defs, attendanceDefs()...)
	defs = append(defs, payrollDefs()...)
	defs = append(defs, knowledgeDefs()...)
	defs = append(defs, employeeDefs()...)
	defs = append(defs, expenseDefs()...)
	defs = append(defs, approvalDefs()...)
	defs = append(defs, salarySimDefs()...)
	defs = append(defs, loanDefs()...)
	defs = append(defs, benefitDefs()...)
	defs = append(defs, performanceDefs()...)
	defs = append(defs, trainingDefs()...)
	defs = append(defs, disciplinaryDefs()...)
	defs = append(defs, analyticsDefs()...)
	defs = append(defs, orgIntelDefs()...)
	defs = append(defs, clearanceDefs()...)
	defs = append(defs, scheduleDefs()...)
	defs = append(defs, workflowDefs()...)
	return defs
}

// Execute runs a tool by name.
func (r *ToolRegistry) Execute(ctx context.Context, name string, companyID, userID int64, input map[string]any) (string, error) {
	executor, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return executor(ctx, companyID, userID, input)
}

func (r *ToolRegistry) registerTools() {
	r.registerLeaveTools()
	r.registerAttendanceTools()
	r.registerPayrollTools()
	r.registerKnowledgeTools()
	r.registerEmployeeTools()
	r.registerExpenseTools()
	r.registerApprovalTools()
	r.registerSalarySimTools()
	r.registerLoanTools()
	r.registerBenefitTools()
	r.registerPerformanceTools()
	r.registerTrainingTools()
	r.registerDisciplinaryTools()
	r.registerAnalyticsTools()
	r.registerOrgIntelTools()
	r.registerClearanceTools()
	r.registerScheduleTools()
	r.registerWorkflowTools()
}
