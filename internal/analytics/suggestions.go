package analytics

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

type suggestion struct {
	Type        string      `json:"type"`
	Priority    string      `json:"priority"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Count       int         `json:"count,omitempty"`
	Items       interface{} `json:"items,omitempty"`
}

// GetSuggestions returns AI-driven smart suggestions based on multiple data sources.
func (h *Handler) GetSuggestions(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	today := time.Now()

	var suggestions []suggestion

	// 1. Regularization due
	regDue, _ := h.queries.ListEmployeesDueForRegularization(c.Request.Context(), store.ListEmployeesDueForRegularizationParams{
		CompanyID: companyID,
		Column2:   today,
	})
	if len(regDue) > 0 {
		suggestions = append(suggestions, suggestion{
			Type:        "regularization",
			Priority:    "high",
			Title:       fmt.Sprintf("%d employee(s) due for regularization", len(regDue)),
			Description: "These probationary employees are approaching or past their regularization date.",
			Count:       len(regDue),
			Items:       regDue,
		})
	}

	// 2. Expiring contracts
	expiring, _ := h.queries.ListExpiringContracts(c.Request.Context(), store.ListExpiringContractsParams{
		CompanyID: companyID,
		Column2:   today,
	})
	if len(expiring) > 0 {
		suggestions = append(suggestions, suggestion{
			Type:        "contract_expiry",
			Priority:    "high",
			Title:       fmt.Sprintf("%d contract(s) expiring within 60 days", len(expiring)),
			Description: "Review these contractual employees for renewal or separation.",
			Count:       len(expiring),
			Items:       expiring,
		})
	}

	// 3. Upcoming birthdays
	birthdays, _ := h.queries.ListUpcomingBirthdays(c.Request.Context(), store.ListUpcomingBirthdaysParams{
		CompanyID: companyID,
		Column2:   today,
	})
	if len(birthdays) > 0 {
		suggestions = append(suggestions, suggestion{
			Type:        "birthday",
			Priority:    "low",
			Title:       fmt.Sprintf("%d upcoming birthday(s) in the next 30 days", len(birthdays)),
			Description: "Send greetings to celebrate your team members.",
			Count:       len(birthdays),
			Items:       birthdays,
		})
	}

	// 4. Pending onboarding tasks
	pendingTasks, _ := h.queries.ListPendingOnboardingTasks(c.Request.Context(), companyID)
	if len(pendingTasks) > 0 {
		suggestions = append(suggestions, suggestion{
			Type:        "onboarding",
			Priority:    "medium",
			Title:       fmt.Sprintf("%d pending onboarding task(s)", len(pendingTasks)),
			Description: "Complete these onboarding tasks to ensure smooth employee integration.",
			Count:       len(pendingTasks),
			Items:       pendingTasks,
		})
	}

	// 5. Employees without salary records
	noSalaryCount, _ := h.queries.CountEmployeesWithNoSalary(c.Request.Context(), store.CountEmployeesWithNoSalaryParams{
		CompanyID:     companyID,
		EffectiveFrom: today,
	})
	if noSalaryCount > 0 {
		suggestions = append(suggestions, suggestion{
			Type:        "missing_salary",
			Priority:    "high",
			Title:       fmt.Sprintf("%d employee(s) have no salary record", noSalaryCount),
			Description: "Assign salary to these employees to include them in payroll.",
			Count:       int(noSalaryCount),
		})
	}

	response.OK(c, suggestions)
}
