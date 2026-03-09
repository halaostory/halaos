package recruitment

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/testutil"
)

var adminAuth = testutil.AuthContext{
	UserID: 1, Email: "admin@test.com", Role: auth.RoleAdmin, CompanyID: 1,
}

func newTestHandler(mockDB *testutil.MockDBTX) *Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(mockDB, logger)
}

// --- nilIfEmpty ---

func TestNilIfEmpty(t *testing.T) {
	if got := nilIfEmpty(""); got != nil {
		t.Fatalf("expected nil for empty string, got %v", got)
	}
	s := "hello"
	got := nilIfEmpty(s)
	if got == nil || *got != s {
		t.Fatalf("expected %q, got %v", s, got)
	}
}

// --- ListJobPostings ---

func TestListJobPostings_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	now := time.Now()
	rows := testutil.NewRows([][]interface{}{
		{int64(1), "Go Developer", strPtr("Engineering"), strPtr("Senior"),
			"regular", "Manila", "open", &now, (*time.Time)(nil), now, int64(3)},
	})
	mockDB.OnQuery(rows, nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/recruitment/jobs", nil, adminAuth)
	h.ListJobPostings(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := testutil.ResponseBody(w)
	data, ok := body["data"].([]interface{})
	if !ok || len(data) != 1 {
		t.Fatalf("expected 1 result, got %v", body)
	}
}

func TestListJobPostings_Empty(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/recruitment/jobs", nil, adminAuth)
	h.ListJobPostings(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListJobPostings_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	c, w := testutil.NewGinContextWithQuery("GET", "/recruitment/jobs", nil, adminAuth)
	h.ListJobPostings(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListJobPostings_WithStatusFilter(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	q := url.Values{"status": {"open"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/recruitment/jobs", q, adminAuth)
	h.ListJobPostings(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	calls := mockDB.Calls()
	if len(calls) != 1 || calls[0].Args[1] != "open" {
		t.Fatalf("expected status filter 'open', got %v", calls[0].Args)
	}
}

// --- GetJobPosting ---

func TestGetJobPosting_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("GET", "/recruitment/jobs/abc", gin.Params{{Key: "id", Value: "abc"}}, nil, adminAuth)
	h.GetJobPosting(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetJobPosting_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	c, w := testutil.NewGinContextWithParams("GET", "/recruitment/jobs/1", gin.Params{{Key: "id", Value: "1"}}, nil, adminAuth)
	h.GetJobPosting(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetJobPosting_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	now := time.Now()
	mockDB.OnQueryRow(testutil.NewRow(
		int64(1), "Go Dev", (*int64)(nil), (*int64)(nil),
		"desc", "reqs", (*float64)(nil), (*float64)(nil),
		"regular", "Manila", "open", (*time.Time)(nil), (*time.Time)(nil), now,
	))

	c, w := testutil.NewGinContextWithParams("GET", "/recruitment/jobs/1", gin.Params{{Key: "id", Value: "1"}}, nil, adminAuth)
	h.GetJobPosting(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- CreateJobPosting ---

func TestCreateJobPosting_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("POST", "/recruitment/jobs", gin.H{}, adminAuth)
	h.CreateJobPosting(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateJobPosting_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewRow(int64(42)))

	c, w := testutil.NewGinContext("POST", "/recruitment/jobs", gin.H{
		"title": "Go Developer",
	}, adminAuth)
	h.CreateJobPosting(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateJobPosting_DefaultEmploymentType(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewRow(int64(1)))

	c, _ := testutil.NewGinContext("POST", "/recruitment/jobs", gin.H{
		"title": "Test",
	}, adminAuth)
	h.CreateJobPosting(c)

	calls := mockDB.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	// employment_type is arg index 8 (0-indexed: companyID, title, deptID, posID, desc, reqs, salMin, salMax, empType, ...)
	if calls[0].Args[8] != "regular" {
		t.Fatalf("expected default employment_type 'regular', got %v", calls[0].Args[8])
	}
}

func TestCreateJobPosting_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("insert failed")))

	c, w := testutil.NewGinContext("POST", "/recruitment/jobs", gin.H{
		"title": "Go Developer",
	}, adminAuth)
	h.CreateJobPosting(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- UpdateJobPosting ---

func TestUpdateJobPosting_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("PUT", "/recruitment/jobs/abc", gin.Params{{Key: "id", Value: "abc"}}, nil, adminAuth)
	h.UpdateJobPosting(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateJobPosting_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnExec(pgconn.NewCommandTag("UPDATE 0"), nil)

	c, w := testutil.NewGinContextWithParams("PUT", "/recruitment/jobs/999",
		gin.Params{{Key: "id", Value: "999"}},
		gin.H{"title": strPtr("Updated")}, adminAuth)
	h.UpdateJobPosting(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateJobPosting_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnExec(pgconn.NewCommandTag("UPDATE 1"), nil)

	c, w := testutil.NewGinContextWithParams("PUT", "/recruitment/jobs/1",
		gin.Params{{Key: "id", Value: "1"}},
		gin.H{"title": strPtr("Updated Title")}, adminAuth)
	h.UpdateJobPosting(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateJobPosting_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnExec(pgconn.NewCommandTag(""), fmt.Errorf("db error"))

	c, w := testutil.NewGinContextWithParams("PUT", "/recruitment/jobs/1",
		gin.Params{{Key: "id", Value: "1"}},
		gin.H{"title": strPtr("X")}, adminAuth)
	h.UpdateJobPosting(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- ListApplicants ---

func TestListApplicants_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	now := time.Now()
	rows := testutil.NewRows([][]interface{}{
		{int64(1), "John", "Doe", "john@test.com", strPtr("123"),
			intPtr(85), strPtr("Good fit"), "new", "manual", now, "Go Developer"},
	})
	mockDB.OnQuery(rows, nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/recruitment/applicants", nil, adminAuth)
	h.ListApplicants(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListApplicants_Empty(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/recruitment/applicants", nil, adminAuth)
	h.ListApplicants(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestListApplicants_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	c, w := testutil.NewGinContextWithQuery("GET", "/recruitment/applicants", nil, adminAuth)
	h.ListApplicants(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestListApplicants_WithFilters(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	q := url.Values{
		"job_posting_id": {"5"},
		"status":         {"screening"},
	}
	c, w := testutil.NewGinContextWithQuery("GET", "/recruitment/applicants", q, adminAuth)
	h.ListApplicants(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	calls := mockDB.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Args[1] != "5" {
		t.Fatalf("expected job_posting_id '5', got %v", calls[0].Args[1])
	}
	if calls[0].Args[2] != "screening" {
		t.Fatalf("expected status 'screening', got %v", calls[0].Args[2])
	}
}

// --- CreateApplicant ---

func TestCreateApplicant_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("POST", "/recruitment/applicants", gin.H{}, adminAuth)
	h.CreateApplicant(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateApplicant_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewRow(int64(10)))

	c, w := testutil.NewGinContext("POST", "/recruitment/applicants", gin.H{
		"job_posting_id": 1,
		"first_name":     "Jane",
		"last_name":      "Smith",
		"email":          "jane@test.com",
	}, adminAuth)
	h.CreateApplicant(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateApplicant_DefaultSource(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewRow(int64(1)))

	c, _ := testutil.NewGinContext("POST", "/recruitment/applicants", gin.H{
		"job_posting_id": 1,
		"first_name":     "Jane",
		"last_name":      "Smith",
		"email":          "jane@test.com",
	}, adminAuth)
	h.CreateApplicant(c)

	calls := mockDB.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	// source is arg index 8 (0-indexed: companyID, jobPostingID, first, last, email, phone, resumeURL, resumeText, source, notes)
	if calls[0].Args[8] != "manual" {
		t.Fatalf("expected default source 'manual', got %v", calls[0].Args[8])
	}
}

func TestCreateApplicant_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("insert failed")))

	c, w := testutil.NewGinContext("POST", "/recruitment/applicants", gin.H{
		"job_posting_id": 1,
		"first_name":     "Jane",
		"last_name":      "Smith",
		"email":          "jane@test.com",
	}, adminAuth)
	h.CreateApplicant(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- UpdateApplicantStatus ---

func TestUpdateApplicantStatus_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("PUT", "/recruitment/applicants/abc/status",
		gin.Params{{Key: "id", Value: "abc"}}, nil, adminAuth)
	h.UpdateApplicantStatus(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateApplicantStatus_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("PUT", "/recruitment/applicants/1/status",
		gin.Params{{Key: "id", Value: "1"}}, gin.H{}, adminAuth)
	h.UpdateApplicantStatus(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateApplicantStatus_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnExec(pgconn.NewCommandTag("UPDATE 0"), nil)

	c, w := testutil.NewGinContextWithParams("PUT", "/recruitment/applicants/999/status",
		gin.Params{{Key: "id", Value: "999"}},
		gin.H{"status": "screening"}, adminAuth)
	h.UpdateApplicantStatus(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateApplicantStatus_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnExec(pgconn.NewCommandTag("UPDATE 1"), nil)

	c, w := testutil.NewGinContextWithParams("PUT", "/recruitment/applicants/1/status",
		gin.Params{{Key: "id", Value: "1"}},
		gin.H{"status": "hired"}, adminAuth)
	h.UpdateApplicantStatus(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	// Verify company_id is used in the SQL (cross-tenant guard)
	calls := mockDB.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Args[1] != int64(1) { // companyID
		t.Fatalf("expected companyID=1, got %v", calls[0].Args[1])
	}
}

func TestUpdateApplicantStatus_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnExec(pgconn.NewCommandTag(""), fmt.Errorf("db error"))

	c, w := testutil.NewGinContextWithParams("PUT", "/recruitment/applicants/1/status",
		gin.Params{{Key: "id", Value: "1"}},
		gin.H{"status": "screening"}, adminAuth)
	h.UpdateApplicantStatus(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- GetApplicant ---

func TestGetApplicant_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("GET", "/recruitment/applicants/abc",
		gin.Params{{Key: "id", Value: "abc"}}, nil, adminAuth)
	h.GetApplicant(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetApplicant_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	c, w := testutil.NewGinContextWithParams("GET", "/recruitment/applicants/999",
		gin.Params{{Key: "id", Value: "999"}}, nil, adminAuth)
	h.GetApplicant(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetApplicant_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	now := time.Now()
	// QueryRow for applicant (15 fields)
	mockDB.OnQueryRow(testutil.NewRow(
		int64(1), "John", "Doe", "john@test.com", strPtr("123"),
		strPtr("https://resume.pdf"), strPtr("Resume text"),
		intPtr(85), strPtr("Good candidate"),
		"screening", "manual", strPtr("Some notes"), now,
		"Go Developer", int64(5),
	))
	// Query for interviews (empty)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	c, w := testutil.NewGinContextWithParams("GET", "/recruitment/applicants/1",
		gin.Params{{Key: "id", Value: "1"}}, nil, adminAuth)
	h.GetApplicant(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ScheduleInterview ---

func TestScheduleInterview_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("POST", "/recruitment/applicants/abc/interviews",
		gin.Params{{Key: "id", Value: "abc"}}, nil, adminAuth)
	h.ScheduleInterview(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestScheduleInterview_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("POST", "/recruitment/applicants/1/interviews",
		gin.Params{{Key: "id", Value: "1"}}, gin.H{}, adminAuth)
	h.ScheduleInterview(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestScheduleInterview_InvalidDateFormat(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("POST", "/recruitment/applicants/1/interviews",
		gin.Params{{Key: "id", Value: "1"}},
		gin.H{"scheduled_at": "bad-date"}, adminAuth)
	h.ScheduleInterview(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestScheduleInterview_ApplicantNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// EXISTS check returns false
	mockDB.OnQueryRow(testutil.NewRow(false))

	c, w := testutil.NewGinContextWithParams("POST", "/recruitment/applicants/999/interviews",
		gin.Params{{Key: "id", Value: "999"}},
		gin.H{"scheduled_at": "2026-04-01T10:00:00Z"}, adminAuth)
	h.ScheduleInterview(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestScheduleInterview_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// EXISTS check
	mockDB.OnQueryRow(testutil.NewRow(true))
	// INSERT returning id
	mockDB.OnQueryRow(testutil.NewRow(int64(7)))
	// UPDATE applicant status (silent)
	mockDB.OnExec(pgconn.NewCommandTag("UPDATE 1"), nil)

	c, w := testutil.NewGinContextWithParams("POST", "/recruitment/applicants/1/interviews",
		gin.Params{{Key: "id", Value: "1"}},
		gin.H{
			"scheduled_at":     "2026-04-01T10:00:00Z",
			"duration_minutes": 45,
			"interview_type":   "technical",
		}, adminAuth)
	h.ScheduleInterview(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestScheduleInterview_DefaultDurationAndType(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewRow(true))
	mockDB.OnQueryRow(testutil.NewRow(int64(1)))
	mockDB.OnExec(pgconn.NewCommandTag("UPDATE 1"), nil)

	c, _ := testutil.NewGinContextWithParams("POST", "/recruitment/applicants/1/interviews",
		gin.Params{{Key: "id", Value: "1"}},
		gin.H{"scheduled_at": "2026-04-01T10:00:00Z"}, adminAuth)
	h.ScheduleInterview(c)

	calls := mockDB.Calls()
	// calls[0]=EXISTS, calls[1]=INSERT, calls[2]=UPDATE
	if len(calls) < 2 {
		t.Fatalf("expected at least 2 calls, got %d", len(calls))
	}
	insertCall := calls[1]
	// INSERT args: applicantID, interviewerID, schedAt, durationMinutes, location, interviewType
	if insertCall.Args[3] != 60 {
		t.Fatalf("expected default duration 60, got %v", insertCall.Args[3])
	}
	if insertCall.Args[5] != "initial" {
		t.Fatalf("expected default type 'initial', got %v", insertCall.Args[5])
	}
}

func TestScheduleInterview_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewRow(true))
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("insert failed")))

	c, w := testutil.NewGinContextWithParams("POST", "/recruitment/applicants/1/interviews",
		gin.Params{{Key: "id", Value: "1"}},
		gin.H{"scheduled_at": "2026-04-01T10:00:00Z"}, adminAuth)
	h.ScheduleInterview(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GetRecruitmentStats ---

func TestGetRecruitmentStats_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewRow(int64(5), int64(100), int64(30), int64(3)))

	c, w := testutil.NewGinContextWithQuery("GET", "/recruitment/stats", nil, adminAuth)
	h.GetRecruitmentStats(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetRecruitmentStats_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("db error")))

	c, w := testutil.NewGinContextWithQuery("GET", "/recruitment/stats", nil, adminAuth)
	h.GetRecruitmentStats(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
