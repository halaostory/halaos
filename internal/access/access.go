// Package access holds record-level checks for data that belongs to one
// employee (loans, requests, documents, claims). Role middleware decides
// who may call a route; these decide whose records they may see.
package access

import (
	"context"
	"strconv"

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

// Decide checks the caller may approve or reject a record of employeeID:
// HR admins decide anyone's; a manager only their direct reports'; nobody
// decides their own. It returns the caller's employee record (nil for an
// admin without one) and writes 403 when not allowed.
func Decide(c *gin.Context, q interface {
	employeeLookup
	GetEmployeeByID(ctx context.Context, arg store.GetEmployeeByIDParams) (store.Employee, error)
}, employeeID int64) (*store.Employee, bool) {
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	uid := auth.GetUserID(c)
	var me *store.Employee
	if e, err := q.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{UserID: &uid, CompanyID: companyID}); err == nil {
		me = &e
	}
	if me != nil && me.ID == employeeID {
		response.Forbidden(c, "You can't approve your own request")
		return nil, false
	}
	if IsAdmin(c) {
		return me, true
	}
	if me != nil && auth.GetRole(c) == auth.RoleManager {
		if e, err := q.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{ID: employeeID, CompanyID: companyID}); err == nil && e.ManagerID != nil && *e.ManagerID == me.ID {
			return me, true
		}
	}
	response.Forbidden(c, "Only the employee's manager or HR can decide this request")
	return nil, false
}

// EmployeeInCompany is route middleware for /…/:id routes about an
// employee: 404 unless that employee belongs to the caller's company.
// Role checks stay with the route; this stops cross-tenant IDs.
func EmployeeInCompany(q interface {
	GetEmployeeByID(ctx context.Context, arg store.GetEmployeeByIDParams) (store.Employee, error)
}, param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param(param), 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid employee ID")
			c.Abort()
			return
		}
		if _, err := q.GetEmployeeByID(c.Request.Context(), store.GetEmployeeByIDParams{ID: id, CompanyID: auth.GetCompanyID(c)}); err != nil {
			response.NotFound(c, "Employee not found")
			c.Abort()
			return
		}
		c.Next()
	}
}
