package employee

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Record-level access for /employees/:id routes. Admins see everyone;
// people always see themselves. Managers see the basic record and
// timeline (as they do in the employee list) but not the private profile
// (bank account, TIN, SSS/PhilHealth/Pag-IBIG numbers, address) or the
// employee's documents.
const (
	accessBasic   = false
	accessPrivate = true
)

func (h *Handler) recordAccess(private bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := auth.GetRole(c)
		if role == auth.RoleAdmin || role == auth.RoleSuperAdmin {
			c.Next()
			return
		}
		if role == auth.RoleManager && !private {
			c.Next()
			return
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid employee ID")
			c.Abort()
			return
		}
		me, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{UserID: ptr(auth.GetUserID(c)), CompanyID: auth.GetCompanyID(c)})
		if err != nil || me.ID != id {
			response.Forbidden(c, "You can only view your own employee record")
			c.Abort()
			return
		}
		c.Next()
	}
}

func ptr(v int64) *int64 { return &v }
