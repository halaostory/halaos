// Package home serves the Warp-style home dashboard: a greeting, the to-dos
// waiting on you, what's coming up (payroll, filings, starters, leavers),
// the status of every government agency registration, and what the agent
// did recently.
package home

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Handler serves GET /home.
type Handler struct {
	q       *store.Queries
	logger  *slog.Logger
	enabled func(module string) bool
}

// NewHandler builds the handler. enabled reports whether an edition module ships.
func NewHandler(q *store.Queries, logger *slog.Logger, enabled func(string) bool) *Handler {
	return &Handler{q: q, logger: logger, enabled: enabled}
}

// RegisterRoutes registers the home route (admins and managers).
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/home", auth.ManagerOrAbove(), h.Get)
	protected.GET("/home/agencies", auth.AdminOnly(), h.Agencies)
}

func (h *Handler) module(m string) bool { return h.enabled == nil || h.enabled(m) }

// Agency is one row of the registrations panel.
type Agency struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Status     string `json:"status"` // active, needs_info, needs_approval, filing_started, awaiting_agency
	Detail     string `json:"detail"`
	EmployerNo string `json:"employer_no,omitempty"`
	Registered int    `json:"employees_registered"`
	Employees  int    `json:"employees_total"`
	Link       string `json:"link"`
}

// Registrations summarises the panel ("3 / 4 agencies active").
type Registrations struct {
	Country  string         `json:"country"`
	Active   int            `json:"active"`
	Total    int            `json:"total"`
	Counts   map[string]int `json:"counts"`
	Agencies []Agency       `json:"agencies"`
}

// SetupStep is one item of the "get payroll-ready" checklist.
type SetupStep struct {
	Key      string `json:"key"`
	Done     bool   `json:"done"`
	Optional bool   `json:"optional,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Link     string `json:"link"`
}

// Response is the home payload. Sections a module does not ship are omitted.
type Response struct {
	Company       string                         `json:"company"`
	Country       string                         `json:"country"`
	Headcount     *store.HomeHeadcountRow        `json:"headcount"`
	Todos         []store.ListHRInboxItemsRow    `json:"todos"`
	TodoTotal     int                            `json:"todo_total"`
	Starts        []store.HomeUpcomingStartsRow  `json:"starts"`
	Leavers       []store.HomeUpcomingLeaversRow `json:"leavers"`
	NextPayroll   *store.HomeNextPayrollRow      `json:"next_payroll,omitempty"`
	LastPayroll   *store.HomeLastPayrollRow      `json:"last_payroll,omitempty"`
	Filings       []store.HomeUpcomingFilingsRow `json:"filings"`
	Overdue       int                            `json:"filings_overdue"`
	Registrations *Registrations                 `json:"registrations,omitempty"`
	AgentRuns     []store.AgentRoutineRun        `json:"agent_runs"`
	Setup         []SetupStep                    `json:"setup,omitempty"`
}

// Get handles GET /home.
func (h *Handler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	comp, err := h.q.GetCompanyByID(ctx, companyID)
	if err != nil {
		response.NotFound(c, "Company not found")
		return
	}
	isAdmin := auth.GetRole(c) == auth.RoleAdmin || auth.GetRole(c) == auth.RoleSuperAdmin
	out := Response{
		Company: comp.Name, Country: comp.Country,
		Todos: []store.ListHRInboxItemsRow{}, Starts: []store.HomeUpcomingStartsRow{}, Leavers: []store.HomeUpcomingLeaversRow{},
		Filings: []store.HomeUpcomingFilingsRow{}, AgentRuns: []store.AgentRoutineRun{},
	}
	if hc, err := h.q.HomeHeadcount(ctx, companyID); err == nil {
		out.Headcount = &hc
	} else {
		h.logger.Warn("home: headcount", "error", err)
	}
	if rows, err := h.q.HomeUpcomingStarts(ctx, companyID); err == nil && rows != nil {
		out.Starts = rows
	}
	if rows, err := h.q.HomeUpcomingLeavers(ctx, companyID); err == nil && rows != nil {
		out.Leavers = rows
	}
	if isAdmin && h.module("lifecycle") {
		if items, err := h.q.ListHRInboxItems(ctx, store.ListHRInboxItemsParams{CompanyID: companyID, Lim: 100}); err == nil {
			out.TodoTotal = len(items)
			if len(items) > 8 {
				items = items[:8]
			}
			out.Todos = items
		}
	}
	if isAdmin && h.module("payroll") {
		if np, err := h.q.HomeNextPayroll(ctx, companyID); err == nil {
			out.NextPayroll = &np
		}
		if lp, err := h.q.HomeLastPayroll(ctx, companyID); err == nil {
			out.LastPayroll = &lp
		}
	}
	if isAdmin && h.module("compliance") {
		if rows, err := h.q.HomeUpcomingFilings(ctx, companyID); err == nil && rows != nil {
			out.Filings = rows
		}
		if n, err := h.q.HomeOverdueFilings(ctx, companyID); err == nil {
			out.Overdue = int(n)
		}
		out.Registrations = h.registrations(ctx, comp)
	}
	if isAdmin && h.module("payroll") {
		out.Setup = h.setup(ctx, comp)
	}
	if isAdmin && h.module("ai") {
		if rows, err := h.q.ListAgentRoutineRuns(ctx, store.ListAgentRoutineRunsParams{CompanyID: companyID, Lim: 6}); err == nil && rows != nil {
			out.AgentRuns = rows
		}
	}
	response.OK(c, out)
}

func (h *Handler) registrations(ctx context.Context, comp store.Company) *Registrations {
	open := map[string]map[string]int{} // agency → status → n
	if rows, err := h.q.HomeGovSubmissionsByAgency(ctx, comp.ID); err == nil {
		for _, r := range rows {
			a := strings.ToUpper(r.Agency)
			if open[a] == nil {
				open[a] = map[string]int{}
			}
			open[a][r.Status] += int(r.N)
		}
	}
	reg := &Registrations{Country: comp.Country, Counts: map[string]int{}}
	switch comp.Country {
	case "PHL":
		cov, _ := h.q.HomeEmployeeIDCoverage(ctx, comp.ID)
		for _, a := range []struct {
			code, name string
			no         *string
			covered    int32
		}{
			{"BIR", "BIR", comp.Tin, cov.Bir},
			{"SSS", "SSS", comp.SssErNo, cov.Sss},
			{"PHILHEALTH", "PhilHealth", comp.PhilhealthErNo, cov.Philhealth},
			{"PAGIBIG", "Pag-IBIG", comp.PagibigErNo, cov.Pagibig},
		} {
			ag := Agency{Code: a.code, Name: a.name, Registered: int(a.covered), Employees: int(cov.Total), Link: "/gov-filings"}
			if a.no != nil {
				ag.EmployerNo = strings.TrimSpace(*a.no)
			}
			ag.Status, ag.Detail = agencyStatus(ag, open[a.code])
			reg.Agencies = append(reg.Agencies, ag)
		}
	case "USA":
		nums, _ := h.q.HomeRegistrationNumbers(ctx, comp.ID)
		for _, n := range nums {
			if !strings.HasPrefix(n.RegistrationType, "state_withholding_") {
				continue
			}
			st := strings.TrimPrefix(n.RegistrationType, "state_withholding_")
			reg.Agencies = append(reg.Agencies, Agency{Code: st, Name: st + " withholding", Status: "active", EmployerNo: n.RegistrationValue, Detail: "Registered", Link: "/settings"})
		}
		if states, err := h.q.ListUnregisteredWorkStates(ctx, comp.ID); err == nil {
			for _, st := range states {
				reg.Agencies = append(reg.Agencies, Agency{Code: st, Name: st + " withholding", Status: "filing_started",
					Detail: "Employees work here — registration task opened", Link: "/gov-filings"})
			}
		}
		sort.Slice(reg.Agencies, func(i, j int) bool { return reg.Agencies[i].Code < reg.Agencies[j].Code })
	default:
		return nil
	}
	for _, a := range reg.Agencies {
		reg.Counts[a.Status]++
		if a.Status == "active" {
			reg.Active++
		}
	}
	reg.Total = len(reg.Agencies)
	return reg
}

// agencyStatus decides a Philippine agency's pill from the employer number,
// open submissions and how many employees the agency already knows.
func agencyStatus(a Agency, open map[string]int) (string, string) {
	switch {
	case a.EmployerNo == "":
		return "needs_info", "Add your employer number in Settings → Company"
	case open["needs_human"] > 0 || open["failed"] > 0:
		return "needs_info", "A filing needs your input"
	case open["ready"] > 0 || open["draft"] > 0:
		return "needs_approval", plural(open["ready"]+open["draft"], "filing waiting for approval", "filings waiting for approval")
	case open["approved"] > 0 || open["running"] > 0:
		return "filing_started", "Submitting now"
	case open["submitted"] > 0:
		return "awaiting_agency", "Filed — waiting for the agency's confirmation"
	case a.Employees > 0 && a.Registered < a.Employees:
		return "needs_info", plural(a.Employees-a.Registered, "employee without an ID number", "employees without an ID number")
	default:
		return "active", "All set"
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

func filled(p *string) bool { return p != nil && strings.TrimSpace(*p) != "" }

// setup computes the "get payroll-ready" checklist from live data (Warp:
// sign up, add the company's tax IDs and bank, add people, run payroll).
func (h *Handler) setup(ctx context.Context, comp store.Company) []SetupStep {
	cnt, err := h.q.HomeSetupCounts(ctx, comp.ID)
	if err != nil {
		h.logger.Warn("home: setup counts", "error", err)
		return nil
	}
	steps := []SetupStep{
		{Key: "company", Done: filled(comp.LegalName) && filled(comp.Tin) && (comp.Country != "PHL" || filled(comp.BirRdo)), Link: "/settings"},
	}
	if comp.Country == "PHL" {
		steps = append(steps, SetupStep{Key: "agencies", Done: filled(comp.SssErNo) && filled(comp.PhilhealthErNo) && filled(comp.PagibigErNo), Link: "/settings"})
	}
	steps = append(steps,
		SetupStep{Key: "bank", Done: filled(comp.BankName) && filled(comp.BankAccountNo), Link: "/settings"},
		SetupStep{Key: "schedule", Done: cnt.PaySchedules > 0, Link: "/payroll"},
		SetupStep{Key: "people", Done: cnt.Employees > 0, Detail: strconv.Itoa(int(cnt.Employees)), Link: "/hire"},
		SetupStep{Key: "salaries", Done: cnt.Employees > 0 && cnt.WithoutSalary == 0, Detail: strconv.Itoa(int(cnt.WithoutSalary)), Link: "/employees"},
	)
	if comp.Country == "PHL" {
		if cov, err := h.q.HomeEmployeeIDCoverage(ctx, comp.ID); err == nil {
			missing := 0
			for _, n := range []int32{cov.Bir, cov.Sss, cov.Philhealth, cov.Pagibig} {
				missing = max(missing, int(cov.Total-n))
			}
			steps = append(steps, SetupStep{Key: "gov_ids", Done: cov.Total > 0 && missing == 0, Detail: strconv.Itoa(missing), Link: "/employees"})
		}
	}
	steps = append(steps,
		SetupStep{Key: "history", Done: cnt.ImportedPeriods > 0 || cnt.PayrollRuns > 0, Optional: true, Link: "/payroll"},
		SetupStep{Key: "first_payroll", Done: cnt.PayrollRuns > 0, Link: "/payroll"},
	)
	return steps
}
