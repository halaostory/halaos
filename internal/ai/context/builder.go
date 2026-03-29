package context

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/store"
)

// PageContext holds the user's current page information for context injection.
type PageContext struct {
	Section string `json:"section,omitempty"` // e.g., "home", "attendance", "leave", "payslips", "profile"
	Action  string `json:"action,omitempty"`  // e.g., "apply", "view", "edit"
}

// Builder constructs context strings to inject into the AI system prompt.
type Builder struct {
	queries *store.Queries
}

// NewBuilder creates a context builder.
func NewBuilder(queries *store.Queries) *Builder {
	return &Builder{queries: queries}
}

// Build generates a context string for the given user and page.
// The result is appended to the agent's system prompt.
func (b *Builder) Build(ctx context.Context, companyID, userID int64, page PageContext) string {
	var parts []string

	// Layer 1: Identity context (~150 tokens)
	identity := b.buildIdentity(ctx, companyID, userID)
	if identity != "" {
		parts = append(parts, identity)
	}

	// Layer 2: Page context (~200 tokens)
	pageCtx := b.buildPageContext(page)
	if pageCtx != "" {
		parts = append(parts, pageCtx)
	}

	// Layer 3: Data snapshot (~400 tokens)
	snapshot := b.buildSnapshot(ctx, companyID, userID, page)
	if snapshot != "" {
		parts = append(parts, snapshot)
	}

	// Layer 4: Integration status (~100 tokens)
	integrations := b.integrationSnapshot(ctx, companyID, userID)
	if integrations != "" {
		parts = append(parts, integrations)
	}

	if len(parts) == 0 {
		return ""
	}

	return fmt.Sprintf("\n\n--- USER CONTEXT ---\n%s\n--- END CONTEXT ---", strings.Join(parts, "\n\n"))
}

// buildIdentity returns user identity information.
func (b *Builder) buildIdentity(ctx context.Context, companyID, userID int64) string {
	user, err := b.queries.GetUserByID(ctx, userID)
	if err != nil {
		return ""
	}

	lines := []string{
		fmt.Sprintf("User: %s %s (ID: %d)", user.FirstName, user.LastName, userID),
		fmt.Sprintf("Role: %s", user.Role),
		fmt.Sprintf("Company ID: %d", companyID),
		fmt.Sprintf("Current time: %s", time.Now().Format("2006-01-02 15:04 MST")),
	}

	// Try to get employee info
	emp, err := b.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err == nil {
		lines = append(lines, fmt.Sprintf("Employee No: %s", emp.EmployeeNo))
	}

	return strings.Join(lines, "\n")
}

// buildPageContext returns context about the user's current page.
func (b *Builder) buildPageContext(page PageContext) string {
	if page.Section == "" {
		return ""
	}

	descriptions := map[string]string{
		"home":          "The user is on the Home dashboard. They can see their clock status, leave balance, and quick actions.",
		"attendance":    "The user is on the Attendance page. They can clock in/out and view attendance records.",
		"leave":         "The user is on the Leave page. They can view balances, apply for leave, and see history.",
		"payslips":      "The user is on the Payslips page. They can view and download their pay slips.",
		"profile":       "The user is on their Profile page. They can view personal info and change settings.",
		"notifications": "The user is viewing their Notifications.",
		"integrations":  "The user is on the Integrations page. They can manage connected services (Slack, Google, GitHub, etc.) and provisioning templates.",
	}

	desc, ok := descriptions[page.Section]
	if !ok {
		return fmt.Sprintf("Current page: %s", page.Section)
	}

	result := fmt.Sprintf("Current page: %s\n%s", page.Section, desc)
	if page.Action != "" {
		result += fmt.Sprintf("\nCurrent action: %s", page.Action)
	}
	return result
}

// buildSnapshot returns a data snapshot relevant to the current page.
func (b *Builder) buildSnapshot(ctx context.Context, companyID, userID int64, page PageContext) string {
	switch page.Section {
	case "leave":
		return b.leaveSnapshot(ctx, companyID, userID)
	case "attendance":
		return b.attendanceSnapshot(ctx, companyID, userID)
	default:
		return ""
	}
}

// leaveSnapshot returns current leave balance summary.
func (b *Builder) leaveSnapshot(ctx context.Context, companyID, userID int64) string {
	emp, err := b.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return ""
	}

	year := int32(time.Now().Year())
	balances, err := b.queries.ListLeaveBalances(ctx, store.ListLeaveBalancesParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
		Year:       year,
	})
	if err != nil || len(balances) == 0 {
		return ""
	}

	lines := []string{fmt.Sprintf("Leave balances for %d:", year)}
	for _, bal := range balances {
		earned := numericToFloat(bal.Earned)
		used := numericToFloat(bal.Used)
		carried := numericToFloat(bal.Carried)
		remaining := earned + carried - used
		lines = append(lines, fmt.Sprintf("- %s: %.1f days remaining (earned %.1f, used %.1f)", bal.LeaveTypeName, remaining, earned, used))
	}
	return strings.Join(lines, "\n")
}

// attendanceSnapshot returns today's attendance status.
func (b *Builder) attendanceSnapshot(ctx context.Context, companyID, userID int64) string {
	emp, err := b.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return ""
	}

	// Check for open attendance (clocked in but not out)
	att, err := b.queries.GetOpenAttendance(ctx, store.GetOpenAttendanceParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err != nil {
		return "Today's attendance: Not clocked in yet."
	}

	if att.ClockInAt.Valid {
		clockIn := att.ClockInAt.Time.Format("15:04")
		if att.ClockOutAt.Valid {
			clockOut := att.ClockOutAt.Time.Format("15:04")
			return fmt.Sprintf("Today's attendance: Clocked in at %s, clocked out at %s", clockIn, clockOut)
		}
		return fmt.Sprintf("Today's attendance: Clocked in at %s (still open)", clockIn)
	}

	return "Today's attendance: Not clocked in yet."
}

// integrationSnapshot returns a summary of the employee's connected integrations.
func (b *Builder) integrationSnapshot(ctx context.Context, companyID, userID int64) string {
	emp, err := b.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return ""
	}

	identities, err := b.queries.ListEmployeeIntegrations(ctx, store.ListEmployeeIntegrationsParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err != nil || len(identities) == 0 {
		return ""
	}

	lines := []string{"Connected integrations:"}
	for _, id := range identities {
		status := id.AccountStatus
		name := id.Provider
		if id.ExternalEmail != nil && *id.ExternalEmail != "" {
			lines = append(lines, fmt.Sprintf("- %s: %s (status: %s)", name, *id.ExternalEmail, status))
		} else if id.ExternalUsername != nil && *id.ExternalUsername != "" {
			lines = append(lines, fmt.Sprintf("- %s: @%s (status: %s)", name, *id.ExternalUsername, status))
		} else {
			lines = append(lines, fmt.Sprintf("- %s: connected (status: %s)", name, status))
		}
	}
	return strings.Join(lines, "\n")
}

// numericToFloat converts a pgtype.Numeric to float64.
func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}
