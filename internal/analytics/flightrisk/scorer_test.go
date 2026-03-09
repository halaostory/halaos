package flightrisk

import (
	"testing"
	"time"
)

func makeEmployee(id int64) employeeInfo {
	return employeeInfo{
		id:           id,
		employeeNo:   "EMP-001",
		firstName:    "Juan",
		lastName:     "Cruz",
		department:   "Engineering",
		departmentID: 10,
		hireDate:     time.Date(2022, 1, 15, 0, 0, 0, 0, time.UTC), // ~50 months, outside risk windows
	}
}

func TestComputeScores_NoFactors(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	emps := []employeeInfo{makeEmployee(1)}
	results := computeScores(emps, nil, nil, nil, nil, now)

	if len(results) != 0 {
		t.Errorf("expected 0 results for employee with no risk signals, got %d", len(results))
	}
}

func TestComputeScores_AttendanceDeterioration_WithHistory(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	emps := []employeeInfo{makeEmployee(1)}
	// recent rate = 10/30 = 0.33, older rate = 5/60 = 0.083, 0.33 > 1.3*0.083 = 0.108
	att := map[int64]attendanceData{1: {recent: 10, older: 5}}

	results := computeScores(emps, att, nil, nil, nil, now)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].RiskScore != 20 {
		t.Errorf("score = %d, want 20", results[0].RiskScore)
	}
}

func TestComputeScores_AttendanceDeterioration_NoHistory(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	emps := []employeeInfo{makeEmployee(1)}
	att := map[int64]attendanceData{1: {recent: 5, older: 0}} // no older data, recent >= 3

	results := computeScores(emps, att, nil, nil, nil, now)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].RiskScore != 20 {
		t.Errorf("score = %d, want 20", results[0].RiskScore)
	}
}

func TestComputeScores_AttendanceDeterioration_FewRecent(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	emps := []employeeInfo{makeEmployee(1)}
	att := map[int64]attendanceData{1: {recent: 2, older: 0}} // no older, recent < 3

	results := computeScores(emps, att, nil, nil, nil, now)

	if len(results) != 0 {
		t.Errorf("expected 0 results for minor attendance issues, got %d", len(results))
	}
}

func TestComputeScores_LeaveExhaustion(t *testing.T) {
	// March = month 3, before October
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	emps := []employeeInfo{makeEmployee(1)}
	lr := map[int64]leaveData{1: {totalUsed: 13, totalEarned: 15}} // 87% > 80%

	results := computeScores(emps, nil, lr, nil, nil, now)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].RiskScore != 15 {
		t.Errorf("score = %d, want 15 (leave exhaustion)", results[0].RiskScore)
	}
}

func TestComputeScores_LeaveExhaustion_AfterOctober(t *testing.T) {
	// November = month 11, >= 10 → should NOT flag
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	emps := []employeeInfo{makeEmployee(1)}
	lr := map[int64]leaveData{1: {totalUsed: 13, totalEarned: 15}}

	results := computeScores(emps, nil, lr, nil, nil, now)

	if len(results) != 0 {
		t.Errorf("expected 0 results after October, got %d", len(results))
	}
}

func TestComputeScores_SalaryStagnation(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	emps := []employeeInfo{makeEmployee(1)}

	tests := []struct {
		name      string
		lastChange time.Time
		wantPts   int
	}{
		{"18+ months", time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), 15},
		{"12-17 months", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 10},
		{"6-11 months", time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC), 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sal := map[int64]time.Time{1: tc.lastChange}
			results := computeScores(emps, nil, nil, sal, nil, now)

			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			if results[0].RiskScore != tc.wantPts {
				t.Errorf("score = %d, want %d", results[0].RiskScore, tc.wantPts)
			}
		})
	}
}

func TestComputeScores_HighRiskTenure(t *testing.T) {
	tests := []struct {
		name     string
		months   int // months since hire
		wantFlag bool
	}{
		{"11 months", 11, true},
		{"12 months", 12, true},
		{"14 months", 14, true},
		{"15 months", 15, false},
		{"23 months", 23, true},
		{"26 months", 26, true},
		{"27 months", 27, false},
		{"6 months", 6, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
			hireDate := now.AddDate(0, -tc.months, 0)
			emp := employeeInfo{
				id:           1,
				employeeNo:   "EMP-001",
				firstName:    "Juan",
				lastName:     "Cruz",
				department:   "Engineering",
				departmentID: 10,
				hireDate:     hireDate,
			}

			results := computeScores([]employeeInfo{emp}, nil, nil, nil, nil, now)

			flagged := len(results) > 0
			if flagged != tc.wantFlag {
				t.Errorf("months=%d: flagged=%v, want %v", tc.months, flagged, tc.wantFlag)
			}
			if flagged && results[0].RiskScore != 15 {
				t.Errorf("months=%d: score = %d, want 15", tc.months, results[0].RiskScore)
			}
		})
	}
}

func TestComputeScores_DepartmentTurnover(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		sepCount int64
		wantPts int
	}{
		{"3+ separations", 5, 15},
		{"2 separations", 2, 10},
		{"1 separation", 1, 5},
		{"0 separations", 0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			emps := []employeeInfo{makeEmployee(1)}
			dt := map[int64]int64{10: tc.sepCount} // dept 10

			results := computeScores(emps, nil, nil, nil, dt, now)

			score := 0
			if len(results) > 0 {
				score = results[0].RiskScore
			}
			if score != tc.wantPts {
				t.Errorf("sepCount=%d: score = %d, want %d", tc.sepCount, score, tc.wantPts)
			}
		})
	}
}

func TestComputeScores_NoDeptID(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	emp := makeEmployee(1)
	emp.departmentID = 0 // no department

	dt := map[int64]int64{0: 5} // should NOT match

	results := computeScores([]employeeInfo{emp}, nil, nil, nil, dt, now)

	if len(results) != 0 {
		t.Errorf("expected 0 results for employee with no dept ID, got %d", len(results))
	}
}

func TestComputeScores_AllFactors(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	emp := employeeInfo{
		id:           1,
		employeeNo:   "EMP-001",
		firstName:    "Maria",
		lastName:     "Santos",
		department:   "Sales",
		departmentID: 5,
		hireDate:     now.AddDate(0, -12, 0), // 12 months = high risk tenure
	}

	att := map[int64]attendanceData{1: {recent: 5, older: 0}} // +20
	lr := map[int64]leaveData{1: {totalUsed: 14, totalEarned: 15}} // +15
	sal := map[int64]time.Time{1: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)} // +15
	dt := map[int64]int64{5: 3} // +15
	// tenure at 12 months → +15

	results := computeScores([]employeeInfo{emp}, att, lr, sal, dt, now)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// 20+15+15+15+15 = 80
	if results[0].RiskScore != 80 {
		t.Errorf("score = %d, want 80", results[0].RiskScore)
	}
	if len(results[0].Factors) != 5 {
		t.Errorf("factors count = %d, want 5", len(results[0].Factors))
	}
}

func TestComputeScores_CapAt100(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	emp := employeeInfo{
		id: 1, employeeNo: "EMP-001", firstName: "A", lastName: "B",
		department: "X", departmentID: 5,
		hireDate: now.AddDate(0, -12, 0),
	}

	att := map[int64]attendanceData{1: {recent: 10, older: 0}}
	lr := map[int64]leaveData{1: {totalUsed: 14, totalEarned: 15}}
	sal := map[int64]time.Time{1: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)}
	dt := map[int64]int64{5: 10}

	results := computeScores([]employeeInfo{emp}, att, lr, sal, dt, now)

	if results[0].RiskScore > 100 {
		t.Errorf("score = %d, should be capped at 100", results[0].RiskScore)
	}
}
