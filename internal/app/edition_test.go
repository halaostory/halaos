package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/edition"
)

func TestModGating_PeopleEdition(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	a := &App{Edition: edition.New(edition.People, "", ""), sink: &gin.New().RouterGroup}
	ok := func(c *gin.Context) { c.String(200, "ok") }

	a.mod(edition.ModPayroll, api).GET("/payroll/cycles", ok)
	a.mod(edition.ModCompliance, api).GET("/gov/forms", ok)
	a.mod(edition.ModAttendance, api).GET("/attendance/clock-in", ok)
	a.mod(edition.ModTalent, api).GET("/performance/reviews", ok)

	want := map[string]int{
		"/api/v1/payroll/cycles":      200,
		"/api/v1/gov/forms":           200,
		"/api/v1/attendance/clock-in": 404,
		"/api/v1/performance/reviews": 404,
	}
	for path, code := range want {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != code {
			t.Errorf("%s: got %d want %d", path, w.Code, code)
		}
	}
}

func TestModGating_SuiteShipsEverything(t *testing.T) {
	a := &App{Edition: edition.New(edition.Suite, "", ""), sink: &gin.New().RouterGroup}
	g := &gin.New().RouterGroup
	for _, m := range edition.All {
		if a.mod(m, g) != g {
			t.Errorf("suite must route module %s", m)
		}
	}
}
