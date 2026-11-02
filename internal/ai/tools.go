package ai

import (
	"context"
	"fmt"
	"github.com/halaostory/halaos/internal/edition"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/store"
)

// ToolRegistry maps tool names to executors.
type ToolRegistry struct {
	queries       *store.Queries
	pool          *pgxpool.Pool
	tools         map[string]ToolExecutor
	edition       edition.Config
	disabledTools map[string]bool // tools of modules not shipped in this edition (read-only after construction)
}

// ToolExecutor runs a tool and returns the result as a string.
type ToolExecutor func(ctx context.Context, companyID, userID int64, input map[string]any) (string, error)

// NewToolRegistry creates and registers all HR tools.
func NewToolRegistry(queries *store.Queries, pool *pgxpool.Pool) *ToolRegistry {
	r := &ToolRegistry{
		queries: queries,
		pool:    pool,
		tools:   make(map[string]ToolExecutor),
		edition: edition.Load(),
	}
	r.registerTools()
	r.disabledTools = disabledToolsFor(r.edition)
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

// toolGroups maps each group of tools to the product module it belongs to,
// so an edition (e.g. HalaPeople) only exposes tools for features it ships.
var toolGroups = []struct {
	module string
	defs   func() []provider.ToolDefinition
}{
	{edition.ModTimeOff, leaveDefs},
	{edition.ModAttendance, attendanceDefs},
	{edition.ModPayroll, payrollDefs},
	{edition.ModCore, knowledgeDefs},
	{edition.ModPeople, employeeDefs},
	{edition.ModExpenses, expenseDefs},
	{edition.ModCore, approvalDefs},
	{edition.ModPayroll, salarySimDefs},
	{edition.ModPayroll, loanDefs},
	{edition.ModBenefits, benefitDefs},
	{edition.ModTalent, performanceDefs},
	{edition.ModTalent, trainingDefs},
	{edition.ModTalent, disciplinaryDefs},
	{edition.ModCore, analyticsDefs},
	{edition.ModIntelligence, orgIntelDefs},
	{edition.ModLifecycle, clearanceDefs},
	{edition.ModAttendance, scheduleDefs},
	{edition.ModIntelligence, workflowDefs},
	{edition.ModIntelligence, blindspotDefs},
	{edition.ModEngagement, pulseDefs},
	{edition.ModEngagement, recognitionDefs},
	{edition.ModCore, hrrequestDefs},
	{edition.ModLifecycle, lifecycleDefs},
}

// Definitions returns tool definitions for the LLM (only modules shipped in
// this edition).
func (r *ToolRegistry) Definitions() []provider.ToolDefinition {
	var defs []provider.ToolDefinition
	for _, g := range toolGroups {
		if r.edition.Enabled(g.module) {
			defs = append(defs, g.defs()...)
		}
	}
	return defs
}

// toolEnabled reports whether a tool belongs to a module of this edition.
func (r *ToolRegistry) toolEnabled(name string) bool { return !r.disabledTools[name] }

func disabledToolsFor(e edition.Config) map[string]bool {
	out := map[string]bool{}
	for _, g := range toolGroups {
		if !e.Enabled(g.module) {
			for _, d := range g.defs() {
				out[d.Name] = true
			}
		}
	}
	return out
}

// Execute runs a tool by name.
func (r *ToolRegistry) Execute(ctx context.Context, name string, companyID, userID int64, input map[string]any) (string, error) {
	executor, ok := r.tools[name]
	if !ok || !r.toolEnabled(name) {
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
	r.registerBlindspotTools()
	r.registerPulseTools()
	r.registerRecognitionTools()
	r.registerHRRequestTools()
	r.registerLifecycleTools()
}
