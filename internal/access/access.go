// Package access holds record-level checks for data that belongs to one
// employee (loans, requests, documents, claims). Role middleware decides
// who may call a route; these decide whose records they may see.
package access

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Who may see an employee's record besides HR admins and the employee.
type Level int

const (
	// SelfOrAdmin: payroll, documents, requests — the employee and HR only.
	SelfOrAdmin Level = iota
	// SelfOrManager: also managers (approvals, onboarding progress).
	SelfOrManager
)

type employeeLookup interface {
	GetEmployeeByUserID(ctx context.Context, arg store.GetEmployeeByUserIDParams) (store.Employee, error)
}

// IsAdmin reports whether the caller is an HR admin.
func IsAdmin(c *gin.Context) bool {
	r := auth.GetRole(c)
	return r == auth.RoleAdmin || r == auth.RoleSuperAdmin
}

// Allowed reports whether the caller may see a record of employeeID.
func Allowed(c *gin.Context, q employeeLookup, employeeID int64, lvl Level) bool {
	if IsAdmin(c) {
		return true
	}
	if lvl == SelfOrManager && auth.GetRole(c) == auth.RoleManager {
		return true
	}
	uid := auth.GetUserID(c)
	me, err := q.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{UserID: &uid, CompanyID: auth.GetCompanyID(c)})
	return err == nil && me.ID == employeeID
}

// Check writes a 404 (so record IDs can't be probed) and returns false
// when the caller may not see the record.
func Check(c *gin.Context, q employeeLookup, employeeID int64, lvl Level) bool {
	if Allowed(c, q, employeeID, lvl) {
		return true
	}
	response.NotFound(c, "Not found")
	return false
}
