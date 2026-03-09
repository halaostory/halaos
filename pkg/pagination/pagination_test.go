package pagination

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newContext(query string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?"+query, nil)
	return c
}

func TestParse_Defaults(t *testing.T) {
	c := newContext("")
	p := Parse(c)

	if p.Page != DefaultPage {
		t.Errorf("page = %d, want %d", p.Page, DefaultPage)
	}
	if p.Limit != DefaultLimit {
		t.Errorf("limit = %d, want %d", p.Limit, DefaultLimit)
	}
	if p.Offset != 0 {
		t.Errorf("offset = %d, want 0", p.Offset)
	}
}

func TestParse_CustomValues(t *testing.T) {
	c := newContext("page=3&limit=50")
	p := Parse(c)

	if p.Page != 3 {
		t.Errorf("page = %d, want 3", p.Page)
	}
	if p.Limit != 50 {
		t.Errorf("limit = %d, want 50", p.Limit)
	}
	if p.Offset != 100 {
		t.Errorf("offset = %d, want 100 ((3-1)*50)", p.Offset)
	}
}

func TestParse_MaxLimit(t *testing.T) {
	c := newContext("limit=999")
	p := Parse(c)

	if p.Limit != MaxLimit {
		t.Errorf("limit = %d, want %d (max)", p.Limit, MaxLimit)
	}
}

func TestParse_NegativeValues(t *testing.T) {
	c := newContext("page=-1&limit=-5")
	p := Parse(c)

	if p.Page != DefaultPage {
		t.Errorf("page = %d, want %d for negative input", p.Page, DefaultPage)
	}
	if p.Limit != DefaultLimit {
		t.Errorf("limit = %d, want %d for negative input", p.Limit, DefaultLimit)
	}
}

func TestParse_ZeroValues(t *testing.T) {
	c := newContext("page=0&limit=0")
	p := Parse(c)

	if p.Page != DefaultPage {
		t.Errorf("page = %d, want %d for zero", p.Page, DefaultPage)
	}
	if p.Limit != DefaultLimit {
		t.Errorf("limit = %d, want %d for zero", p.Limit, DefaultLimit)
	}
}

func TestParse_InvalidValues(t *testing.T) {
	c := newContext("page=abc&limit=xyz")
	p := Parse(c)

	// strconv.Atoi returns 0 for invalid, which triggers defaults
	if p.Page != DefaultPage {
		t.Errorf("page = %d, want %d for invalid input", p.Page, DefaultPage)
	}
	if p.Limit != DefaultLimit {
		t.Errorf("limit = %d, want %d for invalid input", p.Limit, DefaultLimit)
	}
}

func TestParse_OffsetCalculation(t *testing.T) {
	tests := []struct {
		page   string
		limit  string
		offset int
	}{
		{"1", "10", 0},
		{"2", "10", 10},
		{"3", "20", 40},
		{"5", "50", 200},
	}
	for _, tc := range tests {
		c := newContext("page=" + tc.page + "&limit=" + tc.limit)
		p := Parse(c)
		if p.Offset != tc.offset {
			t.Errorf("page=%s limit=%s: offset = %d, want %d", tc.page, tc.limit, p.Offset, tc.offset)
		}
	}
}
