package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// Context key constants (mirrored from auth package to avoid import cycle).
const (
	ctxKeyUserID    = "user_id"
	ctxKeyEmail     = "email"
	ctxKeyRole      = "role"
	ctxKeyCompanyID = "company_id"
)

// AuthContext holds context values to inject into gin.Context.
// Role is interface{} to accept auth.Role without importing auth.
type AuthContext struct {
	UserID    int64
	Email     string
	Role      interface{}
	CompanyID int64
}

// DefaultAdmin provides a standard admin auth context for tests.
// Role is set as a plain string; callers in the auth package should
// override with auth.Role("admin") if needed for type assertion.
var DefaultAdmin = AuthContext{
	UserID:    1,
	Email:     "admin@test.com",
	Role:      "admin",
	CompanyID: 1,
}

// DefaultEmployee provides a standard employee auth context.
var DefaultEmployee = AuthContext{
	UserID:    10,
	Email:     "emp@test.com",
	Role:      "employee",
	CompanyID: 1,
}

// NewGinContext creates a gin.Context with optional JSON body and auth context.
func NewGinContext(method, path string, body interface{}, ac AuthContext) (*gin.Context, *httptest.ResponseRecorder) {
	return NewGinContextWithParams(method, path, nil, body, ac)
}

// NewGinContextWithParams creates a gin.Context with URL params, body, and auth.
func NewGinContextWithParams(method, path string, params gin.Params, body interface{}, ac AuthContext) (*gin.Context, *httptest.ResponseRecorder) {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req, _ := http.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.Request = req

	if params != nil {
		c.Params = params
	}

	// Set auth context
	c.Set(ctxKeyUserID, ac.UserID)
	c.Set(ctxKeyEmail, ac.Email)
	c.Set(ctxKeyRole, ac.Role)
	c.Set(ctxKeyCompanyID, ac.CompanyID)

	return c, w
}

// NewGinContextWithQuery creates a gin.Context with query parameters.
func NewGinContextWithQuery(method, path string, query url.Values, ac AuthContext) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	fullPath := path
	if len(query) > 0 {
		fullPath = path + "?" + query.Encode()
	}

	req, _ := http.NewRequest(method, fullPath, nil)
	c.Request = req

	c.Set(ctxKeyUserID, ac.UserID)
	c.Set(ctxKeyEmail, ac.Email)
	c.Set(ctxKeyRole, ac.Role)
	c.Set(ctxKeyCompanyID, ac.CompanyID)

	return c, w
}

// ResponseBody parses the JSON response body.
func ResponseBody(w *httptest.ResponseRecorder) map[string]interface{} {
	var result map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	return result
}
