package lifecycle

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/employee"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/internal/testutil"
)

var adminAuth = testutil.DefaultAdmin

func newTestHandler(db *testutil.MockDBTX) *Handler {
	return NewHandler(store.New(db), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func idParam(id string) gin.Params { return gin.Params{{Key: "id", Value: id}} }

type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
}

func TestHire_InvalidHireDate(t *testing.T) {
	h := newTestHandler(testutil.NewMockDBTX())
	c, w := testutil.NewGinContext("POST", "/lifecycle/hire", map[string]any{
		"first_name": "Ana", "last_name": "Cruz", "hire_date": "11/02/2026",
	}, adminAuth)
	h.Hire(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHire_MissingName(t *testing.T) {
	h := newTestHandler(testutil.NewMockDBTX())
	c, w := testutil.NewGinContext("POST", "/lifecycle/hire", map[string]any{"hire_date": "2026-11-02"}, adminAuth)
	h.Hire(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHire_UnknownCustomFieldRejectedBeforeWrite(t *testing.T) {
	db := testutil.NewMockDBTX()
	db.OnQuery(testutil.NewEmptyRows(), nil) // ListCustomFields
	h := newTestHandler(db)
	c, w := testutil.NewGinContext("POST", "/lifecycle/hire", map[string]any{
		"first_name": "Ana", "last_name": "Cruz", "hire_date": "2026-11-02",
		"custom_fields": map[string]string{"shoe_size": "42"},
	}, adminAuth)
	h.Hire(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if n := len(db.Calls()); n != 1 {
		t.Errorf("expected only the custom field lookup, got %d DB calls", n)
	}
}

func TestHire_MissingRequiredCustomField(t *testing.T) {
	db := testutil.NewMockDBTX()
	now := time.Now()
	db.OnQuery(testutil.NewRows([][]interface{}{
		{int64(1), int64(1), "tshirt", "T-Shirt", "select", []string{"S", "M"}, true, false, int32(0), true, now, now},
	}), nil)
	h := newTestHandler(db)
	c, w := testutil.NewGinContext("POST", "/lifecycle/hire", map[string]any{
		"first_name": "Ana", "last_name": "Cruz", "hire_date": "2026-11-02",
	}, adminAuth)
	h.Hire(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTerminate_Validation(t *testing.T) {
	cases := []struct {
		name string
		id   string
		body map[string]any
	}{
		{"bad id", "abc", map[string]any{"separation_type": "resignation", "last_working_day": "2026-12-01"}},
		{"bad type", "5", map[string]any{"separation_type": "vacation", "last_working_day": "2026-12-01"}},
		{"bad date", "5", map[string]any{"separation_type": "resignation", "last_working_day": "Dec 1"}},
		{"lwd before notice", "5", map[string]any{"separation_type": "resignation", "notice_date": "2026-12-10", "last_working_day": "2026-12-01"}},
		{"missing lwd", "5", map[string]any{"separation_type": "resignation"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandler(testutil.NewMockDBTX())
			c, w := testutil.NewGinContextWithParams("POST", "/lifecycle/employees/"+tc.id+"/terminate", idParam(tc.id), tc.body, adminAuth)
			h.Terminate(c)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestGetInbox_CountsAndFilter(t *testing.T) {
	now := time.Now()
	db := testutil.NewMockDBTX()
	db.OnQuery(testutil.NewRows([][]interface{}{
		{"leave_approval", int64(1), int64(7), "Ana Cruz", "Leave request", "2026-11-05", "normal", now},
		{"onboarding_task", int64(2), int64(8), "Ben Lim", "Laptop", "2026-10-30", "high", now},
		{"leave_approval", int64(3), int64(9), "Cy Tan", "Leave request", "", "normal", now},
	}), nil)
	h := newTestHandler(db)
	c, w := testutil.NewGinContext("GET", "/inbox?kind=leave_approval", nil, adminAuth)
	c.Request.URL.RawQuery = "kind=leave_approval"
	h.GetInbox(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var env envelope
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	var got InboxResponse
	if err := json.Unmarshal(env.Data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 3 || got.High != 1 || got.Counts["leave_approval"] != 2 || got.Counts["onboarding_task"] != 1 {
		t.Errorf("unexpected summary: %+v", got)
	}
	if len(got.Items) != 2 {
		t.Errorf("filter should keep 2 leave items, got %d", len(got.Items))
	}
}

func TestCreateCustomField_SelectNeedsOptions(t *testing.T) {
	h := newTestHandler(testutil.NewMockDBTX())
	c, w := testutil.NewGinContext("POST", "/custom-fields", map[string]any{"label": "Size", "field_type": "select"}, adminAuth)
	h.CreateCustomField(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateCustomField_DerivesKeyFromLabel(t *testing.T) {
	now := time.Now()
	db := testutil.NewMockDBTX()
	db.OnQueryRow(testutil.NewRow(int64(3), int64(1), "t_shirt_size", "T-Shirt Size", "text", []string{}, false, false, int32(0), true, now, now))
	h := newTestHandler(db)
	c, w := testutil.NewGinContext("POST", "/custom-fields", map[string]any{"label": "T-Shirt Size"}, adminAuth)
	h.CreateCustomField(c)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	calls := db.Calls()
	if len(calls) != 1 || calls[0].Args[1] != "t_shirt_size" {
		t.Errorf("expected derived key t_shirt_size, got %+v", calls)
	}
}

// Routes must coexist with the existing /employees/:id/* routes without a gin panic.
func TestRoutesRegisterAlongsideEmployeeRoutes(t *testing.T) {
	r := gin.New()
	g := r.Group("/api/v1")
	employee.NewHandler(nil, nil, nil).RegisterRoutes(g)
	newTestHandler(testutil.NewMockDBTX()).RegisterRoutes(g)
}

func TestRulesBrief_PrioritisesGovernmentAndUrgent(t *testing.T) {
	now := time.Now()
	b := rulesBrief([]store.ListHRInboxItemsRow{
		{Kind: "onboarding_task", Priority: "normal", CreatedAt: now},
		{Kind: "onboarding_task", Priority: "normal", CreatedAt: now},
		{Kind: "leave_approval", Priority: "normal", CreatedAt: now},
		{Kind: "gov_human_input", Priority: "high", CreatedAt: now},
	})
	if b.Generated != "rules" || len(b.Actions) != 3 || !strings.Contains(b.Actions[0], "CAPTCHA/OTP") {
		t.Errorf("%+v", b)
	}
	if !strings.Contains(b.Summary, "4 items") || !strings.Contains(b.Summary, "1 of them urgent") {
		t.Errorf("summary %q", b.Summary)
	}
	if e := rulesBrief(nil); !strings.Contains(e.Summary, "Nothing") {
		t.Error("empty inbox brief")
	}
}
