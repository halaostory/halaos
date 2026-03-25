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
		baseURL = "http://3.1.66.212"
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
		fmt.Fprintf(os.Stderr, "FATAL: cannot login to %s (err=%v, status=%d)\n", baseURL, err, status)
		os.Exit(1)
	}

	var loginResp struct {
		Success bool `json:"success"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &loginResp); err != nil || !loginResp.Success {
		fmt.Fprintf(os.Stderr, "FATAL: login failed (parse=%v, success=%v)\n", err, loginResp.Success)
		os.Exit(1)
	}
	authToken = loginResp.Data.Token
	fmt.Printf("Logged in as admin@demo.com, token=%s...\n", authToken[:20])

	os.Exit(m.Run())
}
