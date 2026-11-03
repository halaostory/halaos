// Package edition defines which HR modules a deployment ships.
//
// HalaPeople is an open-source, Warp-style "HR on autopilot" platform for
// Southeast Asia. It comes in two editions:
//
//   - people — the default: people records, one-step hire/terminate,
//     payroll, statutory filings, contractors, e-signature, time off,
//     benefits, expenses, devices and the AI agent.
//   - suite  — people plus the optional HR extras: attendance/time
//     tracking, talent (performance, training), engagement (pulse,
//     recognition) and people analytics/workflow automation.
//     "halaos" is accepted as a legacy alias.
//
// Modules that are not part of an edition are not routed at all (404) and
// their background jobs do not run.
package edition

import (
	"os"
	"sort"
	"strconv"
	"strings"
)

// Edition identifies a product.
type Edition string

const (
	People Edition = "people"
	Suite  Edition = "suite"
	// legacyHalaOS was the pre-HalaPeople name of the suite edition.
	legacyHalaOS Edition = "halaos"
)

// Module names. Keep in sync with frontend/src/edition.ts.
const (
	ModCore         = "core"         // auth, company, users, notifications, audit, import/export, holidays, announcements, approvals, dashboard, billing, self-service, HR requests, policies
	ModPeople       = "people"       // employees, directory, documents/201 file, custom fields
	ModLifecycle    = "lifecycle"    // one-step hire/terminate, onboarding & offboarding checklists, clearance, HR inbox, milestones
	ModPayroll      = "payroll"      // payroll, 13th month, final pay, loans, salary structures, reports, accounting sync
	ModCompliance   = "compliance"   // government forms, tax filings, government filing agent
	ModTimeOff      = "time_off"     // leave, leave calendar, encashment, overtime requests (payroll input)
	ModBenefits     = "benefits"     // benefit plans & claims
	ModExpenses     = "expenses"     // expense claims & reimbursements
	ModContractors  = "contractors"  // freelancers/contractors: invoices, EWT, BIR 2307, 0619-E/1601-EQ
	ModESign        = "esign"        // self-onboarding portal, document templates, e-signature
	ModIT           = "it"           // devices/MDM, app provisioning (integration hub)
	ModAI           = "ai"           // AI assistant, agents, drafts, messaging bot
	ModAttendance   = "attendance"   // clock in/out, breaks, geofences, schedules, DTR
	ModTalent       = "talent"       // performance, training, disciplinary, grievance
	ModEngagement   = "engagement"   // pulse surveys, recognition
	ModIntelligence = "intelligence" // people analytics, org intelligence, knowledge base, workflow automation
)

// All lists every module.
var All = []string{
	ModCore, ModPeople, ModLifecycle, ModPayroll, ModCompliance, ModTimeOff, ModBenefits, ModExpenses, ModContractors,
	ModESign, ModIT, ModAI, ModAttendance, ModTalent, ModEngagement, ModIntelligence,
}

var membership = map[Edition][]string{
	Suite:  All,
	People: {ModCore, ModPeople, ModLifecycle, ModPayroll, ModCompliance, ModTimeOff, ModBenefits, ModExpenses, ModContractors, ModESign, ModIT, ModAI},
}

// Country is a market the product can be sold in.
type Country struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Status string `json:"status"` // live, coming_soon
}

var countries = map[Edition][]Country{
	Suite: {
		{"PHL", "Philippines", "live"}, {"LKA", "Sri Lanka", "live"}, {"USA", "United States", "live"},
		{"SGP", "Singapore", "coming_soon"}, {"IDN", "Indonesia", "coming_soon"},
	},
	// Only markets with a complete payroll + statutory filing engine are "live".
	People: {
		{"PHL", "Philippines", "live"},
		{"SGP", "Singapore", "coming_soon"}, {"MYS", "Malaysia", "coming_soon"},
		{"IDN", "Indonesia", "coming_soon"}, {"VNM", "Vietnam", "coming_soon"}, {"THA", "Thailand", "coming_soon"},
	},
}

// Config is the resolved product configuration.
type Config struct {
	Edition     Edition   `json:"edition"`
	ProductName string    `json:"product_name"`
	Tagline     string    `json:"tagline"`
	Modules     []string  `json:"modules"`
	Countries   []Country `json:"countries"`
	AIBilling   bool      `json:"ai_billing"` // BILLING_ENABLED: meter AI usage with prepaid tokens
	// AIAssistant reports whether an AI provider is configured (the agent
	// chat and AI tools are served only then).
	AIAssistant bool `json:"ai_assistant"`

	enabled map[string]bool `json:"-"`
}

// Load reads APP_EDITION (default people), PRODUCT_NAME and
// PRODUCT_TAGLINE. Unknown editions fall back to people.
func Load() Config {
	c := New(Edition(strings.ToLower(strings.TrimSpace(os.Getenv("APP_EDITION")))), os.Getenv("PRODUCT_NAME"), os.Getenv("PRODUCT_TAGLINE"))
	c.AIBilling, _ = strconv.ParseBool(os.Getenv("BILLING_ENABLED"))
	return c
}

// New builds a config for an edition.
func New(e Edition, name, tagline string) Config {
	if e == legacyHalaOS {
		e = Suite
	}
	if _, ok := membership[e]; !ok {
		e = People
	}
	if name == "" {
		name = "HalaPeople"
	}
	if tagline == "" {
		tagline = "Payroll, compliance and HR for Southeast Asia — on autopilot"
	}
	c := Config{Edition: e, ProductName: name, Tagline: tagline, Countries: countries[e], enabled: map[string]bool{}}
	for _, m := range membership[e] {
		c.enabled[m] = true
		c.Modules = append(c.Modules, m)
	}
	sort.Strings(c.Modules)
	return c
}

// Enabled reports whether a module ships in this edition.
func (c Config) Enabled(module string) bool { return c.enabled[module] }

// CountryLive reports whether new companies may sign up for this country.
func (c Config) CountryLive(code string) bool {
	for _, k := range c.Countries {
		if k.Code == code {
			return k.Status == "live"
		}
	}
	return false
}
