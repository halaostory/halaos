package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/store"
)

// leaveConditions represents the structured conditions for leave rules.
type leaveConditions struct {
	MaxDays           *float64 `json:"max_days,omitempty"`
	RequireBalance    *bool    `json:"require_balance,omitempty"`
	RequireNoConflict *bool    `json:"require_no_conflict,omitempty"`
	AllowedLeaveTypes []string `json:"allowed_leave_types,omitempty"`
	MinTenureMonths   *int     `json:"min_tenure_months,omitempty"`
}

// otConditions represents the structured conditions for OT rules.
type otConditions struct {
	MaxHours       *float64 `json:"max_hours,omitempty"`
	AllowedOTTypes []string `json:"allowed_ot_types,omitempty"`
}

// evaluatedCondition records what was checked and whether it passed.
type evaluatedCondition struct {
	Check  string `json:"check"`
	Value  any    `json:"value"`
	Limit  any    `json:"limit"`
	Passed bool   `json:"passed"`
}

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

func evaluateLeaveConditions(
	ctx context.Context,
	queries *store.Queries,
	pool *pgxpool.Pool,
	companyID int64,
	req store.ListPendingLeaveRequestsForAutoApprovalRow,
	conditionsJSON json.RawMessage,
) (bool, string, json.RawMessage) {
	var conds leaveConditions
	if err := json.Unmarshal(conditionsJSON, &conds); err != nil {
		return false, "", nil
	}

	days := numericToFloat(req.Days)
	evaluated := make([]evaluatedCondition, 0)
	allPassed := true

	// max_days check
	if conds.MaxDays != nil {
		passed := days <= *conds.MaxDays
		evaluated = append(evaluated, evaluatedCondition{
			Check: "max_days", Value: days, Limit: *conds.MaxDays, Passed: passed,
		})
		if !passed {
			allPassed = false
		}
	}

	// allowed_leave_types check
	if len(conds.AllowedLeaveTypes) > 0 {
		passed := contains(conds.AllowedLeaveTypes, req.LeaveTypeCode)
		evaluated = append(evaluated, evaluatedCondition{
			Check: "allowed_leave_types", Value: req.LeaveTypeCode, Limit: conds.AllowedLeaveTypes, Passed: passed,
		})
		if !passed {
			allPassed = false
		}
	}

	// min_tenure_months check
	if conds.MinTenureMonths != nil {
		tenureMonths := int(time.Since(req.HireDate).Hours() / 24 / 30)
		passed := tenureMonths >= *conds.MinTenureMonths
		evaluated = append(evaluated, evaluatedCondition{
			Check: "min_tenure_months", Value: tenureMonths, Limit: *conds.MinTenureMonths, Passed: passed,
		})
		if !passed {
			allPassed = false
		}
	}

	// require_balance check
	if conds.RequireBalance != nil && *conds.RequireBalance {
		year := time.Now().Year()
		balance, err := queries.GetLeaveBalance(ctx, store.GetLeaveBalanceParams{
			CompanyID:   companyID,
			EmployeeID:  req.EmployeeID,
			LeaveTypeID: req.LeaveTypeID,
			Year:        int32(year),
		})
		if err != nil {
			evaluated = append(evaluated, evaluatedCondition{
				Check: "require_balance", Value: 0, Limit: days, Passed: false,
			})
			allPassed = false
		} else {
			available := balanceAvailable(balance)
			passed := available >= days
			evaluated = append(evaluated, evaluatedCondition{
				Check: "require_balance", Value: available, Limit: days, Passed: passed,
			})
			if !passed {
				allPassed = false
			}
		}
	}

	// require_no_conflict check
	if conds.RequireNoConflict != nil && *conds.RequireNoConflict {
		hasConflict := checkDepartmentConflict(ctx, pool, companyID, req)
		evaluated = append(evaluated, evaluatedCondition{
			Check: "require_no_conflict", Value: !hasConflict, Limit: true, Passed: !hasConflict,
		})
		if hasConflict {
			allPassed = false
		}
	}

	evaluatedJSON, _ := json.Marshal(evaluated)

	if !allPassed || len(evaluated) == 0 {
		return false, "", evaluatedJSON
	}

	reason := fmt.Sprintf("auto-approved: leave %.0f day(s) %s, all conditions met", days, req.LeaveTypeCode)
	return true, reason, evaluatedJSON
}

func evaluateOTConditions(
	req store.ListPendingOTRequestsForAutoApprovalRow,
	conditionsJSON json.RawMessage,
) (bool, string, json.RawMessage) {
	var conds otConditions
	if err := json.Unmarshal(conditionsJSON, &conds); err != nil {
		return false, "", nil
	}

	hours := numericToFloat(req.Hours)
	evaluated := make([]evaluatedCondition, 0)
	allPassed := true

	// max_hours check
	if conds.MaxHours != nil {
		passed := hours <= *conds.MaxHours
		evaluated = append(evaluated, evaluatedCondition{
			Check: "max_hours", Value: hours, Limit: *conds.MaxHours, Passed: passed,
		})
		if !passed {
			allPassed = false
		}
	}

	// allowed_ot_types check
	if len(conds.AllowedOTTypes) > 0 {
		passed := contains(conds.AllowedOTTypes, req.OtType)
		evaluated = append(evaluated, evaluatedCondition{
			Check: "allowed_ot_types", Value: req.OtType, Limit: conds.AllowedOTTypes, Passed: passed,
		})
		if !passed {
			allPassed = false
		}
	}

	evaluatedJSON, _ := json.Marshal(evaluated)

	if !allPassed || len(evaluated) == 0 {
		return false, "", evaluatedJSON
	}

	reason := fmt.Sprintf("auto-approved: OT %.1f hours (%s), all conditions met", hours, req.OtType)
	return true, reason, evaluatedJSON
}

func balanceAvailable(b store.LeaveBalance) float64 {
	earned := numericToFloat(b.Earned)
	used := numericToFloat(b.Used)
	carried := numericToFloat(b.Carried)
	adjusted := numericToFloat(b.Adjusted)
	return earned + carried + adjusted - used
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

func checkDepartmentConflict(ctx context.Context, pool *pgxpool.Pool, companyID int64, req store.ListPendingLeaveRequestsForAutoApprovalRow) bool {
	if req.DepartmentID == nil {
		return false
	}
	var count int64
	err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM leave_requests lr
		 JOIN employees e ON e.id = lr.employee_id
		 WHERE lr.company_id = $1
		   AND e.department_id = $2
		   AND lr.status = 'approved'
		   AND lr.start_date <= $4
		   AND lr.end_date >= $3
		   AND lr.employee_id != $5`,
		companyID, *req.DepartmentID, req.StartDate, req.EndDate, req.EmployeeID,
	).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}
