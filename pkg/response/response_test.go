package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupContext() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

func TestOK(t *testing.T) {
	c, w := setupContext()
	OK(c, map[string]string{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.Error != nil {
		t.Error("expected no error")
	}
}

func TestCreated(t *testing.T) {
	c, w := setupContext()
	Created(c, "new-item")

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Success {
		t.Error("expected success=true")
	}
}

func TestPaginated(t *testing.T) {
	c, w := setupContext()
	items := []string{"a", "b", "c"}
	Paginated(c, items, 25, 2, 10)

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.Meta == nil {
		t.Fatal("expected meta")
	}
	if resp.Meta.Total != 25 {
		t.Errorf("meta.total = %d, want 25", resp.Meta.Total)
	}
	if resp.Meta.Page != 2 {
		t.Errorf("meta.page = %d, want 2", resp.Meta.Page)
	}
	if resp.Meta.Limit != 10 {
		t.Errorf("meta.limit = %d, want 10", resp.Meta.Limit)
	}
	if resp.Meta.Pages != 3 {
		t.Errorf("meta.pages = %d, want 3", resp.Meta.Pages)
	}
}

func TestPaginated_ExactDivision(t *testing.T) {
	c, w := setupContext()
	Paginated(c, []string{}, 20, 1, 10)

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Meta.Pages != 2 {
		t.Errorf("meta.pages = %d, want 2", resp.Meta.Pages)
	}
}

func TestBadRequest(t *testing.T) {
	c, w := setupContext()
	BadRequest(c, "invalid input")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Success {
		t.Error("expected success=false")
	}
	if resp.Error == nil || resp.Error.Code != "bad_request" {
		t.Errorf("expected bad_request error code, got %v", resp.Error)
	}
	if resp.Error.Message != "invalid input" {
		t.Errorf("error message = %q, want %q", resp.Error.Message, "invalid input")
	}
}

func TestUnauthorized(t *testing.T) {
	c, w := setupContext()
	Unauthorized(c, "not logged in")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestForbidden(t *testing.T) {
	c, w := setupContext()
	Forbidden(c, "no access")

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestNotFound(t *testing.T) {
	c, w := setupContext()
	NotFound(c, "not found")

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestConflict(t *testing.T) {
	c, w := setupContext()
	Conflict(c, "duplicate")

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestInternalError(t *testing.T) {
	c, w := setupContext()
	InternalError(c, "server error")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error == nil || resp.Error.Code != "internal_error" {
		t.Errorf("expected internal_error code, got %v", resp.Error)
	}
}
