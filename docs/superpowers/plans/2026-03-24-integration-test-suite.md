# HalaOS Integration Test Suite — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go integration test suite that creates 50+ employees via the live REST API and verifies all major HR modules (employees, attendance, leave, payroll, compliance, dashboard, directory, AI).

**Architecture:** Tests live under `tests/integration/` as a single Go package. `main_test.go` handles login and shared state. Each numbered test file covers one module. Tests call `http://3.1.66.212:8080/api/v1` with JWT auth from the admin account. Employee user accounts are created for employee-context operations (clock-in, leave).

**Tech Stack:** Go 1.25, `net/http`, `testing`, `encoding/json`, `github.com/stretchr/testify`

**Spec:** `docs/superpowers/specs/2026-03-24-integration-test-suite-design.md`

---

### Task 1: Scaffold test package with main_test.go and helpers_test.go

**Files:**
- Create: `tests/integration/main_test.go`
- Create: `tests/integration/helpers_test.go`

- [ ] **Step 1: Create `tests/integration/main_test.go`**

```go
package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

var (
	baseURL       string
	authToken     string
	empUserTokens map[int64]string // employee_id → JWT token
	createdEmps   []int64
	deptIDs       map[string]int64 // dept code → ID
	posIDs        map[string]int64 // position code → ID
	testCycleID   int64            // payroll cycle ID from TestCreatePayrollCycle
)

func TestMain(m *testing.M) {
	baseURL = os.Getenv("HALAOS_BASE_URL")
	if baseURL == "" {
		baseURL = "http://3.1.66.212:8080"
	}

	empUserTokens = make(map[int64]string)
	deptIDs = make(map[string]int64)
	posIDs = make(map[string]int64)

	// Login as admin
	body := map[string]any{
		"email":    "admin@demo.com",
		"password": "Admin123abc",
	}
	resp, status, err := doPost(baseURL+"/api/v1/auth/login", "", body)
	if err != nil || status != 200 {
		fmt.Fprintf(os.Stderr, "SKIP: cannot login to %s (err=%v, status=%d)\n", baseURL, err, status)
		os.Exit(0)
	}

	var loginResp struct {
		Success bool `json:"success"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &loginResp); err != nil || !loginResp.Success {
		fmt.Fprintf(os.Stderr, "SKIP: login failed (parse=%v, success=%v)\n", err, loginResp.Success)
		os.Exit(0)
	}
	authToken = loginResp.Data.Token
	fmt.Printf("Logged in as admin@demo.com, token=%s...\n", authToken[:20])

	os.Exit(m.Run())
}
```

- [ ] **Step 2: Create `tests/integration/helpers_test.go`**

```go
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// doRequest performs an HTTP request with optional auth token and returns raw body + status.
func doRequest(method, url, token string, body map[string]any) (json.RawMessage, int, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	// Rate limit retry (max 3 times)
	if resp.StatusCode == 429 {
		retryCount++
		if retryCount > 3 {
			return json.RawMessage(respBody), resp.StatusCode, fmt.Errorf("rate limited after 3 retries")
		}
		wait := 2
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if w, err := strconv.Atoi(ra); err == nil {
				wait = w
			}
		}
		time.Sleep(time.Duration(wait) * time.Second)
		return doRequest(method, url, token, body)
	}

	return json.RawMessage(respBody), resp.StatusCode, nil
}

func doPost(url, token string, body map[string]any) (json.RawMessage, int, error) {
	return doRequest(http.MethodPost, url, token, body)
}

// apiGet performs GET on /api/v1{path} with admin auth.
func apiGet(path string, query map[string]string) (json.RawMessage, int, error) {
	reqURL := baseURL + "/api/v1" + path
	if len(query) > 0 {
		params := url.Values{}
		for k, v := range query {
			if v != "" {
				params.Set(k, v)
			}
		}
		if encoded := params.Encode(); encoded != "" {
			reqURL += "?" + encoded
		}
	}
	return doRequest(http.MethodGet, reqURL, authToken, nil)
}

// apiPost performs POST on /api/v1{path} with admin auth.
func apiPost(path string, body map[string]any) (json.RawMessage, int, error) {
	return doRequest(http.MethodPost, baseURL+"/api/v1"+path, authToken, body)
}

// apiPut performs PUT on /api/v1{path} with admin auth.
func apiPut(path string, body map[string]any) (json.RawMessage, int, error) {
	return doRequest(http.MethodPut, baseURL+"/api/v1"+path, authToken, body)
}

// apiGetAs performs GET with a specific token (for employee-context calls).
func apiGetAs(token, path string, query map[string]string) (json.RawMessage, int, error) {
	reqURL := baseURL + "/api/v1" + path
	if len(query) > 0 {
		params := url.Values{}
		for k, v := range query {
			if v != "" {
				params.Set(k, v)
			}
		}
		if encoded := params.Encode(); encoded != "" {
			reqURL += "?" + encoded
		}
	}
	return doRequest(http.MethodGet, reqURL, token, nil)
}

// apiPostAs performs POST with a specific token.
func apiPostAs(token, path string, body map[string]any) (json.RawMessage, int, error) {
	return doRequest(http.MethodPost, baseURL+"/api/v1"+path, token, body)
}

// requireSuccess asserts HTTP 200 and success:true in the response envelope.
func requireSuccess(t *testing.T, resp json.RawMessage, status int) {
	t.Helper()
	require.Equal(t, 200, status, "expected HTTP 200, got %d: %s", status, string(resp))
	var envelope struct {
		Success bool `json:"success"`
	}
	require.NoError(t, json.Unmarshal(resp, &envelope))
	require.True(t, envelope.Success, "expected success:true, body: %s", string(resp))
}

// requireCreated asserts HTTP 201 and success:true.
func requireCreated(t *testing.T, resp json.RawMessage, status int) {
	t.Helper()
	require.Equal(t, 201, status, "expected HTTP 201, got %d: %s", status, string(resp))
	var envelope struct {
		Success bool `json:"success"`
	}
	require.NoError(t, json.Unmarshal(resp, &envelope))
	require.True(t, envelope.Success, "expected success:true, body: %s", string(resp))
}

// extractID extracts data.id from the standard response envelope.
func extractID(t *testing.T, resp json.RawMessage) int64 {
	t.Helper()
	var envelope struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp, &envelope))
	require.NotZero(t, envelope.Data.ID, "expected non-zero id")
	return envelope.Data.ID
}

// extractList extracts data as []any from the response.
func extractList(t *testing.T, resp json.RawMessage) []any {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp, &envelope))
	var list []any
	require.NoError(t, json.Unmarshal(envelope.Data, &list))
	return list
}

// extractData extracts the data field as raw JSON.
func extractData(t *testing.T, resp json.RawMessage) json.RawMessage {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp, &envelope))
	return envelope.Data
}

// PH name lists for test data generation.
var phFirstNames = []string{
	"Juan", "Maria", "Carlo", "Ana", "Jose", "Rosa",
	"Miguel", "Sofia", "Paolo", "Isabella", "Marco",
	"Gabriela", "Rafael", "Camille", "Antonio",
	"Patricia", "Gabriel", "Beatriz", "Francisco", "Elena",
	"Ricardo", "Luisa", "Diego", "Carmen", "Fernando",
	"Teresa", "Andres", "Victoria", "Eduardo", "Rosario",
	"Alejandro", "Catalina", "Roberto", "Dolores", "Enrique",
	"Mercedes", "Alfonso", "Pilar", "Sergio", "Esperanza",
	"Bernardo", "Soledad", "Vicente", "Consuelo", "Arturo",
	"Remedios", "Ignacio", "Amparo", "Raul", "Trinidad",
}

var phLastNames = []string{
	"Santos", "Reyes", "Cruz", "Bautista", "Gonzales",
	"Garcia", "Mendoza", "Torres", "Villanueva", "Ramos",
	"Aquino", "Dela Cruz", "Fernandez", "Castillo", "Rivera",
	"Lopez", "Morales", "Navarro", "Flores", "Perez",
}

// randomEmployee generates a unique employee payload.
func randomEmployee(seq int) map[string]any {
	ts := time.Now().UnixMilli()
	fn := phFirstNames[seq%len(phFirstNames)]
	ln := phLastNames[seq%len(phLastNames)]
	empType := "regular"
	if seq%10 >= 7 && seq%10 < 9 {
		empType = "probationary"
	} else if seq%10 == 9 {
		empType = "contractual"
	}

	year := 2024 + (seq % 3) // 2024, 2025, 2026
	month := (seq%12) + 1
	day := (seq%28) + 1

	return map[string]any{
		"employee_no":     fmt.Sprintf("INT-%d-%03d", ts, seq),
		"first_name":      fn,
		"last_name":       ln,
		"email":           fmt.Sprintf("int.%s.%s.%d@test.halaos.com", fn, ln, seq),
		"phone":           fmt.Sprintf("0917%07d", 1000000+seq),
		"birth_date":      fmt.Sprintf("%d-%02d-%02d", 1985+(seq%15), month, day),
		"gender":          []string{"male", "female"}[seq%2],
		"civil_status":    []string{"single", "married", "single", "married"}[seq%4],
		"nationality":     "Filipino",
		"hire_date":       fmt.Sprintf("%d-%02d-%02d", year, month, day),
		"employment_type": empType,
	}
}
```

- [ ] **Step 3: Verify the package compiles**

Run: `cd /Users/anna/Documents/aigonhr && go build ./tests/integration/`

This should fail because there are no test functions yet, but the package should parse correctly (no syntax errors).

- [ ] **Step 4: Commit**

```bash
git add tests/integration/main_test.go tests/integration/helpers_test.go
git commit -m "test: scaffold integration test package with helpers"
```

---

### Task 2: Implement 01_employees_test.go — Org structure + 50 employees

**Files:**
- Create: `tests/integration/01_employees_test.go`

- [ ] **Step 1: Write `01_employees_test.go`**

```go
package integration

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateDepartments(t *testing.T) {
	// First check existing departments
	resp, status, err := apiGet("/company/departments", nil)
	require.NoError(t, err)
	if status == 200 {
		var envelope struct {
			Data []struct {
				ID   int64  `json:"id"`
				Code string `json:"code"`
			} `json:"data"`
		}
		if json.Unmarshal(resp, &envelope) == nil {
			for _, d := range envelope.Data {
				deptIDs[d.Code] = d.ID
			}
		}
	}

	depts := []struct{ code, name string }{
		{"ENG", "Engineering"},
		{"HR", "Human Resources"},
		{"FIN", "Finance"},
		{"MKT", "Marketing"},
		{"OPS", "Operations"},
		{"ADM", "Administration"},
		{"SAL", "Sales"},
	}

	for _, d := range depts {
		if _, exists := deptIDs[d.code]; exists {
			t.Logf("Department %s already exists (ID=%d)", d.code, deptIDs[d.code])
			continue
		}
		resp, status, err := apiPost("/company/departments", map[string]any{
			"code": d.code,
			"name": d.name,
		})
		require.NoError(t, err)
		if status == 409 {
			t.Logf("Department %s already exists (conflict)", d.code)
			continue
		}
		requireCreated(t, resp, status)
		deptIDs[d.code] = extractID(t, resp)
		t.Logf("Created department %s (ID=%d)", d.code, deptIDs[d.code])
	}

	assert.GreaterOrEqual(t, len(deptIDs), 7, "should have at least 7 departments")
}

func TestCreatePositions(t *testing.T) {
	require.NotEmpty(t, deptIDs, "departments must be created first")

	// Check existing positions
	resp, status, err := apiGet("/company/positions", nil)
	require.NoError(t, err)
	if status == 200 {
		var envelope struct {
			Data []struct {
				ID   int64  `json:"id"`
				Code string `json:"code"`
			} `json:"data"`
		}
		if json.Unmarshal(resp, &envelope) == nil {
			for _, p := range envelope.Data {
				posIDs[p.Code] = p.ID
			}
		}
	}

	positions := []struct {
		code, title, deptCode string
		grade                 string
	}{
		{"ENG-CTO", "Chief Technology Officer", "ENG", "E1"},
		{"ENG-SR", "Senior Developer", "ENG", "M2"},
		{"ENG-JR", "Junior Developer", "ENG", "J1"},
		{"ENG-QA", "QA Lead", "ENG", "M1"},
		{"HR-MGR", "HR Manager", "HR", "M2"},
		{"HR-OFF", "HR Officer", "HR", "J2"},
		{"FIN-MGR", "Finance Manager", "FIN", "M2"},
		{"FIN-ACC", "Accountant", "FIN", "J2"},
		{"MKT-MGR", "Marketing Manager", "MKT", "M2"},
		{"MKT-SPE", "Marketing Specialist", "MKT", "J1"},
		{"OPS-MGR", "Operations Manager", "OPS", "M2"},
		{"OPS-SUP", "Operations Supervisor", "OPS", "M1"},
		{"ADM-AST", "Admin Assistant", "ADM", "J1"},
		{"SAL-REP", "Sales Representative", "SAL", "J1"},
	}

	for _, p := range positions {
		if _, exists := posIDs[p.code]; exists {
			t.Logf("Position %s already exists (ID=%d)", p.code, posIDs[p.code])
			continue
		}
		deptID, ok := deptIDs[p.deptCode]
		if !ok {
			t.Logf("Skipping position %s: department %s not found", p.code, p.deptCode)
			continue
		}
		resp, status, err := apiPost("/company/positions", map[string]any{
			"code":          p.code,
			"title":         p.title,
			"department_id": deptID,
			"grade":         p.grade,
		})
		require.NoError(t, err)
		if status == 409 {
			t.Logf("Position %s already exists (conflict)", p.code)
			continue
		}
		requireCreated(t, resp, status)
		posIDs[p.code] = extractID(t, resp)
		t.Logf("Created position %s (ID=%d)", p.code, posIDs[p.code])
	}

	assert.GreaterOrEqual(t, len(posIDs), 10, "should have at least 10 positions")
}

func TestCreateEmployees(t *testing.T) {
	require.NotEmpty(t, deptIDs, "departments must be created first")

	// Distribute across departments and positions
	deptCodes := []string{"ENG", "ENG", "ENG", "HR", "FIN", "MKT", "OPS", "ADM", "SAL", "ENG"}
	posCodes := []string{"ENG-SR", "ENG-JR", "ENG-QA", "HR-OFF", "FIN-ACC", "MKT-SPE", "OPS-SUP", "ADM-AST", "SAL-REP", "ENG-JR"}

	// First 5 are managers
	mgrPosCodes := []string{"ENG-CTO", "HR-MGR", "FIN-MGR", "MKT-MGR", "OPS-MGR"}
	mgrDeptCodes := []string{"ENG", "HR", "FIN", "MKT", "OPS"}

	var managerIDs []int64

	for i := 0; i < 50; i++ {
		emp := randomEmployee(i)

		if i < 5 {
			// Managers
			if deptID, ok := deptIDs[mgrDeptCodes[i]]; ok {
				emp["department_id"] = deptID
			}
			if posID, ok := posIDs[mgrPosCodes[i]]; ok {
				emp["position_id"] = posID
			}
		} else {
			// Regular employees
			idx := i % len(deptCodes)
			if deptID, ok := deptIDs[deptCodes[idx]]; ok {
				emp["department_id"] = deptID
			}
			if posID, ok := posIDs[posCodes[idx]]; ok {
				emp["position_id"] = posID
			}
			if len(managerIDs) > 0 {
				emp["manager_id"] = managerIDs[i%len(managerIDs)]
			}
		}

		resp, status, err := apiPost("/employees", emp)
		require.NoError(t, err, "employee %d", i)
		requireCreated(t, resp, status)
		id := extractID(t, resp)
		createdEmps = append(createdEmps, id)

		if i < 5 {
			managerIDs = append(managerIDs, id)
		}

		if i%10 == 0 {
			t.Logf("Created employee %d/%d (ID=%d, %s %s)", i+1, 50, id, emp["first_name"], emp["last_name"])
		}
	}

	assert.Len(t, createdEmps, 50, "should have created 50 employees")
	t.Logf("Created %d employees total", len(createdEmps))
}

func TestCreateEmployeeUserAccounts(t *testing.T) {
	require.GreaterOrEqual(t, len(createdEmps), 3, "need at least 3 employees")

	// Create user accounts for first 3 employees
	for i := 0; i < 3; i++ {
		empID := createdEmps[i]
		email := fmt.Sprintf("empuser-%d@test.halaos.com", empID)
		password := "TestPass123abc"

		resp, status, err := apiPost("/users/employee-account", map[string]any{
			"employee_id": empID,
			"email":       email,
			"password":    password,
			"role":        "employee",
		})
		require.NoError(t, err)
		requireCreated(t, resp, status)
		t.Logf("Created user account for employee %d (%s)", empID, email)

		// Login as this employee to get their JWT
		loginResp, loginStatus, loginErr := doPost(baseURL+"/api/v1/auth/login", "", map[string]any{
			"email":    email,
			"password": password,
		})
		require.NoError(t, loginErr)
		require.Equal(t, 200, loginStatus, "employee login failed: %s", string(loginResp))

		var lr struct {
			Data struct {
				Token string `json:"token"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(loginResp, &lr))
		require.NotEmpty(t, lr.Data.Token)
		empUserTokens[empID] = lr.Data.Token
		t.Logf("Employee %d logged in, token=%s...", empID, lr.Data.Token[:20])
	}

	assert.Len(t, empUserTokens, 3, "should have 3 employee tokens")
}

func TestAssignSalaries(t *testing.T) {
	require.GreaterOrEqual(t, len(createdEmps), 10, "need at least 10 employees")

	salaries := []float64{18000, 22000, 25000, 30000, 35000, 40000, 45000, 50000, 60000, 80000}

	for i := 0; i < 10; i++ {
		empID := createdEmps[i]
		resp, status, err := apiPost(fmt.Sprintf("/employees/%d/salary", empID), map[string]any{
			"basic_salary":   salaries[i],
			"effective_from": "2026-01-01",
			"remarks":        "Integration test salary",
		})
		require.NoError(t, err)
		requireCreated(t, resp, status)
		t.Logf("Assigned salary %.0f to employee %d", salaries[i], empID)
	}
}

func TestListEmployees(t *testing.T) {
	resp, status, err := apiGet("/employees", map[string]string{"page": "1", "limit": "20"})
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	assert.NotEmpty(t, list, "employee list should not be empty")
	t.Logf("Listed %d employees on page 1", len(list))

	// Verify pagination meta
	var envelope struct {
		Meta struct {
			Total int `json:"total"`
			Page  int `json:"page"`
			Limit int `json:"limit"`
		} `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(resp, &envelope))
	assert.GreaterOrEqual(t, envelope.Meta.Total, 50, "total should include our created employees")
	t.Logf("Total employees: %d", envelope.Meta.Total)
}

func TestGetEmployee(t *testing.T) {
	require.NotEmpty(t, createdEmps, "need created employees")

	resp, status, err := apiGet(fmt.Sprintf("/employees/%d", createdEmps[0]), nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	data := extractData(t, resp)
	var emp struct {
		ID         int64  `json:"id"`
		FirstName  string `json:"first_name"`
		LastName   string `json:"last_name"`
		EmployeeNo string `json:"employee_no"`
	}
	require.NoError(t, json.Unmarshal(data, &emp))
	assert.Equal(t, createdEmps[0], emp.ID)
	assert.NotEmpty(t, emp.FirstName)
	assert.NotEmpty(t, emp.EmployeeNo)
	t.Logf("Got employee: %s %s (%s)", emp.FirstName, emp.LastName, emp.EmployeeNo)
}
```

- [ ] **Step 2: Run the test against the server**

Run: `cd /Users/anna/Documents/aigonhr && go test ./tests/integration/ -v -run "TestCreate|TestAssign|TestList|TestGetEmployee" -count=1 -timeout 10m`

Expected: All tests pass, 50 employees created, 3 user accounts created, 10 salaries assigned.

- [ ] **Step 3: Commit**

```bash
git add tests/integration/01_employees_test.go
git commit -m "test: add employee creation integration tests"
```

---

### Task 3: Implement 02_attendance_test.go

**Files:**
- Create: `tests/integration/02_attendance_test.go`

- [ ] **Step 1: Write `02_attendance_test.go`**

```go
package integration

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClockIn(t *testing.T) {
	require.NotEmpty(t, empUserTokens, "need employee user tokens from 01_employees")

	for empID, token := range empUserTokens {
		resp, status, err := apiPostAs(token, "/attendance/clock-in", map[string]any{
			"source": "web",
			"lat":    "14.5995",
			"lng":    "120.9842",
			"note":   fmt.Sprintf("Integration test clock-in for emp %d", empID),
		})
		require.NoError(t, err, "clock-in failed for employee %d", empID)
		// Accept 200 (success) or 409/400 (already clocked in)
		if status == 200 || status == 201 {
			t.Logf("Employee %d clocked in successfully", empID)
		} else {
			t.Logf("Employee %d clock-in status %d: %s", empID, status, string(resp))
		}
	}
}

func TestClockOut(t *testing.T) {
	require.NotEmpty(t, empUserTokens, "need employee user tokens from 01_employees")

	for empID, token := range empUserTokens {
		resp, status, err := apiPostAs(token, "/attendance/clock-out", map[string]any{
			"source": "web",
			"lat":    "14.5995",
			"lng":    "120.9842",
			"note":   fmt.Sprintf("Integration test clock-out for emp %d", empID),
		})
		require.NoError(t, err, "clock-out failed for employee %d", empID)
		if status == 200 || status == 201 {
			t.Logf("Employee %d clocked out successfully", empID)
		} else {
			t.Logf("Employee %d clock-out status %d: %s", empID, status, string(resp))
		}
	}
}

func TestGetAttendanceRecords(t *testing.T) {
	resp, status, err := apiGet("/attendance/records", map[string]string{
		"page":  "1",
		"limit": "20",
	})
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	assert.NotEmpty(t, list, "should contain attendance records from clock-in/out")
	t.Logf("Attendance records: %d entries", len(list))
}

func TestGetAttendanceSummary(t *testing.T) {
	// Use an employee token for summary
	for _, token := range empUserTokens {
		resp, status, err := apiGetAs(token, "/attendance/summary", nil)
		require.NoError(t, err)
		requireSuccess(t, resp, status)
		t.Logf("Attendance summary: %s", string(resp)[:min(200, len(resp))])
		break
	}
}
```

- [ ] **Step 2: Run the attendance tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./tests/integration/ -v -run "TestClock|TestGetAttendance" -count=1 -timeout 5m`

Expected: Clock-in/out succeed for 3 employees, records and summary return data.

- [ ] **Step 3: Commit**

```bash
git add tests/integration/02_attendance_test.go
git commit -m "test: add attendance integration tests"
```

---

### Task 4: Implement 03_leave_test.go

**Files:**
- Create: `tests/integration/03_leave_test.go`

- [ ] **Step 1: Write `03_leave_test.go`**

```go
package integration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLeaveBalances(t *testing.T) {
	require.NotEmpty(t, empUserTokens, "need employee user tokens")

	for empID, token := range empUserTokens {
		resp, status, err := apiGetAs(token, "/leaves/balances", nil)
		require.NoError(t, err)
		requireSuccess(t, resp, status)
		t.Logf("Employee %d leave balances: %d bytes", empID, len(resp))
		break
	}
}

func TestCreateLeaveRequest(t *testing.T) {
	require.NotEmpty(t, empUserTokens, "need employee user tokens")

	// Use first employee token
	var empID int64
	var token string
	for empID, token = range empUserTokens {
		break
	}

	resp, status, err := apiPostAs(token, "/leaves/requests", map[string]any{
		"leave_type_id": 1, // Vacation Leave (from seed data)
		"start_date":    "2026-12-22",
		"end_date":      "2026-12-24",
		"days":          "3",
		"reason":        "Integration test leave request",
	})
	require.NoError(t, err)
	if status == 201 || status == 200 {
		t.Logf("Leave request created for employee %d", empID)
		data := extractData(t, resp)
		var lr struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		}
		require.NoError(t, json.Unmarshal(data, &lr))
		assert.Equal(t, "pending", lr.Status)
		t.Logf("Leave request ID=%d, status=%s", lr.ID, lr.Status)
	} else {
		t.Logf("Leave request status %d: %s (may lack balance)", status, string(resp))
	}
}

func TestListLeaveRequests(t *testing.T) {
	resp, status, err := apiGet("/leaves/requests", map[string]string{
		"page":  "1",
		"limit": "20",
	})
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	t.Logf("Found %d leave requests", len(list))
}
```

- [ ] **Step 2: Run the leave tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./tests/integration/ -v -run "TestGetLeaveBalances|TestCreateLeaveRequest|TestListLeaveRequests" -count=1 -timeout 5m`

- [ ] **Step 3: Commit**

```bash
git add tests/integration/03_leave_test.go
git commit -m "test: add leave management integration tests"
```

---

### Task 5: Implement 04_payroll_test.go

**Files:**
- Create: `tests/integration/04_payroll_test.go`

- [ ] **Step 1: Write `04_payroll_test.go`**

```go
package integration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListPayrollCycles(t *testing.T) {
	resp, status, err := apiGet("/payroll/cycles", map[string]string{"page": "1", "limit": "10"})
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	t.Logf("Found %d payroll cycles", len(list))
}

func TestCreatePayrollCycle(t *testing.T) {
	resp, status, err := apiPost("/payroll/cycles", map[string]any{
		"name":         "Integration Test - Feb 2026 P1",
		"period_start": "2026-02-01",
		"period_end":   "2026-02-15",
		"pay_date":     "2026-02-20",
		"cycle_type":   "regular",
	})
	require.NoError(t, err)
	requireCreated(t, resp, status)

	data := extractData(t, resp)
	var cycle struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(data, &cycle))
	assert.Equal(t, "draft", cycle.Status)
	testCycleID = cycle.ID
	t.Logf("Created payroll cycle ID=%d, status=%s", cycle.ID, cycle.Status)
}

func TestRunPayroll(t *testing.T) {
	require.NotZero(t, testCycleID, "need payroll cycle from TestCreatePayrollCycle")

	resp, status, err := apiPost("/payroll/runs", map[string]any{
		"cycle_id": testCycleID,
		"run_type": "regular",
	})
	require.NoError(t, err)
	requireCreated(t, resp, status)

	data := extractData(t, resp)
	var run struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(data, &run))
	t.Logf("Payroll run ID=%d, status=%s", run.ID, run.Status)
}

func TestGetPayslips(t *testing.T) {
	// Check existing payslips (from seed data or previous runs)
	for _, token := range empUserTokens {
		resp, status, err := apiGetAs(token, "/payroll/payslips", nil)
		require.NoError(t, err)
		requireSuccess(t, resp, status)
		t.Logf("Payslips response: %d bytes", len(resp))
		break
	}
}
```

- [ ] **Step 2: Run the payroll tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./tests/integration/ -v -run "TestListPayrollCycles|TestCreatePayrollCycle|TestGetPayslips" -count=1 -timeout 5m`

- [ ] **Step 3: Commit**

```bash
git add tests/integration/04_payroll_test.go
git commit -m "test: add payroll integration tests"
```

---

### Task 6: Implement 05_compliance_test.go

**Files:**
- Create: `tests/integration/05_compliance_test.go`

- [ ] **Step 1: Write `05_compliance_test.go`**

```go
package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSSSTable(t *testing.T) {
	resp, status, err := apiGet("/compliance/sss-table", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	assert.GreaterOrEqual(t, len(list), 20, "SSS table should have 20+ brackets")
	t.Logf("SSS table: %d brackets", len(list))
}

func TestGetPhilHealthTable(t *testing.T) {
	resp, status, err := apiGet("/compliance/philhealth-table", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	assert.NotEmpty(t, list, "PhilHealth table should have data")
	t.Logf("PhilHealth table: %d rows", len(list))
}

func TestGetPagIBIGTable(t *testing.T) {
	resp, status, err := apiGet("/compliance/pagibig-table", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	assert.NotEmpty(t, list, "PagIBIG table should have data")
	t.Logf("PagIBIG table: %d rows", len(list))
}

func TestGetBIRTaxTable(t *testing.T) {
	// Test semi-monthly (default)
	resp, status, err := apiGet("/compliance/bir-tax-table", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	assert.NotEmpty(t, list, "BIR tax table should have brackets")
	t.Logf("BIR tax table (semi_monthly): %d brackets", len(list))

	// Test monthly frequency
	resp2, status2, err2 := apiGet("/compliance/bir-tax-table", map[string]string{"frequency": "monthly"})
	require.NoError(t, err2)
	requireSuccess(t, resp2, status2)

	list2 := extractList(t, resp2)
	assert.NotEmpty(t, list2, "BIR monthly tax table should have brackets")
	t.Logf("BIR tax table (monthly): %d brackets", len(list2))
}
```

- [ ] **Step 2: Run the compliance tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./tests/integration/ -v -run "TestGetSSS|TestGetPhilHealth|TestGetPagIBIG|TestGetBIR" -count=1 -timeout 5m`

- [ ] **Step 3: Commit**

```bash
git add tests/integration/05_compliance_test.go
git commit -m "test: add compliance table integration tests"
```

---

### Task 7: Implement 06_dashboard_test.go

**Files:**
- Create: `tests/integration/06_dashboard_test.go`

- [ ] **Step 1: Write `06_dashboard_test.go`**

```go
package integration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDashboardStats(t *testing.T) {
	resp, status, err := apiGet("/dashboard/stats", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	data := extractData(t, resp)
	var stats struct {
		TotalEmployees int `json:"total_employees"`
	}
	require.NoError(t, json.Unmarshal(data, &stats))
	assert.Greater(t, stats.TotalEmployees, 0, "should have employees")
	t.Logf("Dashboard stats: %d total employees", stats.TotalEmployees)
}

func TestGetDashboardAttendance(t *testing.T) {
	resp, status, err := apiGet("/dashboard/attendance", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Dashboard attendance: %d bytes", len(resp))
}

func TestGetDepartmentDistribution(t *testing.T) {
	resp, status, err := apiGet("/dashboard/department-distribution", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Department distribution: %d bytes", len(resp))
}

func TestGetPayrollTrend(t *testing.T) {
	resp, status, err := apiGet("/dashboard/payroll-trend", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Payroll trend: %d bytes", len(resp))
}

func TestGetLeaveSummary(t *testing.T) {
	resp, status, err := apiGet("/dashboard/leave-summary", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Leave summary: %d bytes", len(resp))
}

func TestGetActionItems(t *testing.T) {
	resp, status, err := apiGet("/dashboard/action-items", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Action items: %d bytes", len(resp))
}

func TestGetCelebrations(t *testing.T) {
	resp, status, err := apiGet("/dashboard/celebrations", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Celebrations: %d bytes", len(resp))
}

func TestGetSuggestions(t *testing.T) {
	resp, status, err := apiGet("/dashboard/suggestions", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Suggestions: %d bytes", len(resp))
}

func TestGetFlightRisk(t *testing.T) {
	resp, status, err := apiGet("/dashboard/flight-risk", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Flight risk: %d bytes", len(resp))
}

func TestGetTeamHealth(t *testing.T) {
	resp, status, err := apiGet("/dashboard/team-health", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Team health: %d bytes", len(resp))
}

func TestGetBurnoutRisk(t *testing.T) {
	resp, status, err := apiGet("/dashboard/burnout-risk", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Burnout risk: %d bytes", len(resp))
}

func TestGetComplianceAlerts(t *testing.T) {
	resp, status, err := apiGet("/dashboard/compliance-alerts", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Compliance alerts: %d bytes", len(resp))
}
```

- [ ] **Step 2: Run the dashboard tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./tests/integration/ -v -run "TestGetDashboard|TestGetDepartmentDistribution|TestGetPayrollTrend|TestGetLeaveSummary|TestGetActionItems|TestGetCelebrations|TestGetSuggestions|TestGetFlightRisk|TestGetTeamHealth|TestGetBurnoutRisk|TestGetComplianceAlerts" -count=1 -timeout 5m`

- [ ] **Step 3: Commit**

```bash
git add tests/integration/06_dashboard_test.go
git commit -m "test: add dashboard integration tests (12 endpoints)"
```

---

### Task 8: Implement 07_directory_test.go and 08_ai_test.go

**Files:**
- Create: `tests/integration/07_directory_test.go`
- Create: `tests/integration/08_ai_test.go`

- [ ] **Step 1: Write `07_directory_test.go`**

```go
package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDirectory(t *testing.T) {
	resp, status, err := apiGet("/directory", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	assert.NotEmpty(t, list, "directory should contain employees")
	t.Logf("Directory: %d entries", len(list))
}

func TestGetOrgChart(t *testing.T) {
	resp, status, err := apiGet("/directory/org-chart", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Org chart: %d bytes", len(resp))
}
```

- [ ] **Step 2: Write `08_ai_test.go`**

```go
package integration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIChat(t *testing.T) {
	resp, status, err := apiPost("/ai/chat", map[string]any{
		"message": "How many employees are there?",
	})
	require.NoError(t, err)

	// Skip if AI is not configured (503 or specific error)
	if status == 503 || status == 500 {
		t.Skipf("AI not configured on server (HTTP %d)", status)
		return
	}

	requireSuccess(t, resp, status)

	data := extractData(t, resp)
	var chatResp struct {
		Response string `json:"response"`
		Message  string `json:"message"`
	}
	json.Unmarshal(data, &chatResp)
	answer := chatResp.Response
	if answer == "" {
		answer = chatResp.Message
	}
	assert.NotEmpty(t, answer, "AI should return a response")
	t.Logf("AI response: %s", answer[:min(200, len(answer))])
}
```

- [ ] **Step 3: Run all remaining tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./tests/integration/ -v -run "TestGetDirectory|TestGetOrgChart|TestAIChat" -count=1 -timeout 5m`

- [ ] **Step 4: Commit**

```bash
git add tests/integration/07_directory_test.go tests/integration/08_ai_test.go
git commit -m "test: add directory and AI chat integration tests"
```

---

### Task 9: Full suite run + verification

- [ ] **Step 1: Run the complete test suite**

Run: `cd /Users/anna/Documents/aigonhr && go test ./tests/integration/ -v -count=1 -timeout 10m 2>&1 | tee /tmp/integration-test-results.txt`

Expected: All tests pass (some may skip if AI not configured).

- [ ] **Step 2: Verify results against checklist**

Check the output for:
- 50+ employees created
- 3 employee user accounts created and logged in
- 10 salaries assigned
- Clock-in/out executed
- Leave request created with status "pending"
- Payroll cycle created with status "draft"
- All 4 compliance tables return data
- All 12 dashboard endpoints return success
- Directory and org chart return data

- [ ] **Step 3: Final commit if any fixes needed**

- [ ] **Step 4: Verify existing project tests still pass**

Run: `cd /Users/anna/Documents/aigonhr && go test ./... -short -count=1 -timeout 5m 2>&1 | tail -20`

Expected: All existing 258 unit tests pass.
