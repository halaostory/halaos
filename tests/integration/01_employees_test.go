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
