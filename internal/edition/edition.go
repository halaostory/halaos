// Package edition defines the products built from this codebase and which
// modules each one ships.
//
//   - halaos  — the full AI HR operating system (default; every module)
//   - people  — HalaPeople, a Warp-style "HR on autopilot" product for
//     Southeast Asia: people records, one-step hire/terminate, payroll,
//     statutory filings, e-signature, time off, benefits, expenses, devices.
//
// Each product is deployed separately (own domain, database and brand).
// Modules that are not part of an edition are not routed at all (404) and
// their background jobs do not run.
package edition

import (
	"os"
	"sort"
	"strings"
)

// Edition identifies a product.
type Edition string

const (
	HalaOS Edition = "halaos"
	People Edition = "people"
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
	ModESign        = "esign"        // self-onboarding portal, document templates, e-signature
	ModIT           = "it"           // devices/MDM, app provisioning (integration hub)
	ModAI           = "ai"           // AI assistant, agents, drafts, messaging bot
	ModAttendance   = "attendance"   // clock in/out, breaks, geofences, schedules, DTR
	ModTalent       = "talent"       // performance, training, disciplinary, grievance
	ModEngagement   = "engagement"   // pulse surveys, recognition, NPS, referrals
	ModIntelligence = "intelligence" // analytics, org intelligence, knowledge base, workflow automation
	ModWorkspace    = "workspace"    // virtual office
)

// All lists every module.
var All = []string{
	ModCore, ModPeople, ModLifecycle, ModPayroll, ModCompliance, ModTimeOff, ModBenefits, ModExpenses,
	ModESign, ModIT, ModAI, ModAttendance, ModTalent, ModEngagement, ModIntelligence, ModWorkspace,
}

var membership = map[Edition][]string{
	HalaOS: All,
	People: {ModCore, ModPeople, ModLifecycle, ModPayroll, ModCompliance, ModTimeOff, ModBenefits, ModExpenses, ModESign, ModIT, ModAI},
}

// Country is a market the product can be sold in.
type Country struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Status string `json:"status"` // live, coming_soon
}

var countries = map[Edition][]Country{
	HalaOS: {
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
	Edition     Edition         `json:"edition"`
	ProductName string          `json:"product_name"`
	Tagline     string          `json:"tagline"`
	Modules     []string        `json:"modules"`
	Countries   []Country       `json:"countries"`
	enabled     map[string]bool `json:"-"`
}

// Load reads APP_EDITION (default halaos), PRODUCT_NAME and
// PRODUCT_TAGLINE. Unknown editions fall back to halaos.
func Load() Config {
	return New(Edition(strings.ToLower(strings.TrimSpace(os.Getenv("APP_EDITION")))), os.Getenv("PRODUCT_NAME"), os.Getenv("PRODUCT_TAGLINE"))
}

// New builds a config for an edition.
func New(e Edition, name, tagline string) Config {
	if _, ok := membership[e]; !ok {
		e = HalaOS
	}
	if name == "" {
		name = map[Edition]string{HalaOS: "HalaOS", People: "HalaPeople"}[e]
	}
	if tagline == "" {
		tagline = map[Edition]string{
			HalaOS: "AI-powered HR Operating System",
			People: "Payroll, compliance and HR for Southeast Asia — on autopilot",
		}[e]
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
