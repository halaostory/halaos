package access

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// BodyEmployeesInCompany guards every write: when a JSON body names an
// employee ("employee_id" / "employee_ids", at the top level), each one
// must belong to the caller's company. Handlers that forget to check this
// would otherwise attach another tenant's employee to records here (and
// then show their name and number back).
func BodyEmployeesInCompany(q interface {
	GetEmployeeByID(ctx context.Context, arg store.GetEmployeeByIDParams) (store.Employee, error)
}) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
		default:
			c.Next()
			return
		}
		if !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") || c.Request.Body == nil {
			c.Next()
			return
		}
		raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 8<<20))
		if err != nil {
			response.BadRequest(c, "Could not read the request")
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(raw))
		ids := employeeRefs(raw)
		companyID := auth.GetCompanyID(c)
		for _, id := range ids {
			if _, err := q.GetEmployeeByID(c.Request.Context(), store.GetEmployeeByIDParams{ID: id, CompanyID: companyID}); err != nil {
				response.BadRequest(c, "Unknown employee")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// employeeRefs returns the positive employee IDs named at the top level.
func employeeRefs(raw []byte) []int64 {
	var body map[string]json.RawMessage
	if json.Unmarshal(raw, &body) != nil {
		return nil
	}
	var out []int64
	if v, ok := body["employee_id"]; ok {
		var id int64
		if json.Unmarshal(v, &id) == nil && id > 0 {
			out = append(out, id)
		}
	}
	if v, ok := body["employee_ids"]; ok {
		var ids []int64
		if json.Unmarshal(v, &ids) == nil {
			for _, id := range ids {
				if id > 0 {
					out = append(out, id)
				}
			}
		}
	}
	if len(out) > 500 {
		out = out[:500]
	}
	return out
}
