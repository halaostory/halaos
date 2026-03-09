package burnout

import (
	"testing"
)

func makeEmployee(id int64) employeeInfo {
	return employeeInfo{
		id:         id,
		employeeNo: "EMP-001",
		firstName:  "Juan",
		lastName:   "Cruz",
		department: "Engineering",
	}
}

func TestComputeScores_NoFactors(t *testing.T) {
	emps := []employeeInfo{makeEmployee(1)}
	results := computeScores(emps, nil, nil, nil, nil, nil)

	if len(results) != 0 {
		t.Errorf("expected 0 results for employee with no risk signals, got %d", len(results))
	}
}

func TestComputeScores_OvertimeHigh(t *testing.T) {
	emps := []employeeInfo{makeEmployee(1)}
	ot := map[int64]otData{1: {otDays: 12, avgHours: 3.5}}

	results := computeScores(emps, ot, nil, nil, nil, nil)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].BurnoutScore != 25 {
		t.Errorf("score = %d, want 25 (high OT)", results[0].BurnoutScore)
	}
	if results[0].Factors[0].Factor != "overtime_frequency" {
		t.Errorf("factor = %q, want overtime_frequency", results[0].Factors[0].Factor)
	}
}

func TestComputeScores_OvertimeMedium(t *testing.T) {
	emps := []employeeInfo{makeEmployee(1)}
	ot := map[int64]otData{1: {otDays: 7, avgHours: 2.0}}

	results := computeScores(emps, ot, nil, nil, nil, nil)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].BurnoutScore != 15 {
		t.Errorf("score = %d, want 15 (medium OT)", results[0].BurnoutScore)
	}
}

func TestComputeScores_OvertimeLow(t *testing.T) {
	emps := []employeeInfo{makeEmployee(1)}
	ot := map[int64]otData{1: {otDays: 3, avgHours: 1.0}}

	results := computeScores(emps, ot, nil, nil, nil, nil)

	if len(results) != 0 {
		t.Errorf("expected 0 results for low OT (< 5 days), got %d", len(results))
	}
}

func TestComputeScores_LeaveAvoidance_NeverUsed(t *testing.T) {
	emps := []employeeInfo{makeEmployee(1)}
	la := map[int64]leaveAvoidData{1: {earned: 15, used: 0}}

	results := computeScores(emps, nil, la, nil, nil, nil)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].BurnoutScore != 20 {
		t.Errorf("score = %d, want 20 (never used leave)", results[0].BurnoutScore)
	}
}

func TestComputeScores_LeaveAvoidance_LowUsage(t *testing.T) {
	emps := []employeeInfo{makeEmployee(1)}
	la := map[int64]leaveAvoidData{1: {earned: 15, used: 2}} // 2/15 = 13% < 20%

	results := computeScores(emps, nil, la, nil, nil, nil)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].BurnoutScore != 12 {
		t.Errorf("score = %d, want 12 (low leave usage)", results[0].BurnoutScore)
	}
}

func TestComputeScores_LongHours(t *testing.T) {
	tests := []struct {
		name     string
		avgDaily float64
		want     int
	}{
		{"extreme", 12.0, 20},
		{"high", 10.5, 12},
		{"normal", 8.0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			emps := []employeeInfo{makeEmployee(1)}
			lh := map[int64]hoursData{1: {avgDaily: tc.avgDaily, workdays: 20}}

			results := computeScores(emps, nil, nil, lh, nil, nil)

			score := 0
			if len(results) > 0 {
				score = results[0].BurnoutScore
			}
			if score != tc.want {
				t.Errorf("avgDaily=%.1f: score = %d, want %d", tc.avgDaily, score, tc.want)
			}
		})
	}
}

func TestComputeScores_WeekendWork(t *testing.T) {
	tests := []struct {
		name    string
		wkDays  int64
		wantPts int
	}{
		{"many weekends", 8, 15},  // capped at 15 even for >= 6
		{"some weekends", 4, 15},
		{"few weekends", 2, 0},    // < 3 → no flag
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			emps := []employeeInfo{makeEmployee(1)}
			wk := map[int64]int64{1: tc.wkDays}

			results := computeScores(emps, nil, nil, nil, wk, nil)

			score := 0
			if len(results) > 0 {
				score = results[0].BurnoutScore
			}
			if score != tc.wantPts {
				t.Errorf("weekendDays=%d: score = %d, want %d", tc.wkDays, score, tc.wantPts)
			}
		})
	}
}

func TestComputeScores_AttendanceIrregularity(t *testing.T) {
	tests := []struct {
		name    string
		stddev  float64
		wantPts int
	}{
		{"high variance", 75.0, 20},
		{"medium variance", 40.0, 10},
		{"low variance", 15.0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			emps := []employeeInfo{makeEmployee(1)}
			irr := map[int64]float64{1: tc.stddev}

			results := computeScores(emps, nil, nil, nil, nil, irr)

			score := 0
			if len(results) > 0 {
				score = results[0].BurnoutScore
			}
			if score != tc.wantPts {
				t.Errorf("stddev=%.1f: score = %d, want %d", tc.stddev, score, tc.wantPts)
			}
		})
	}
}

func TestComputeScores_AllFactors(t *testing.T) {
	emps := []employeeInfo{makeEmployee(1)}
	ot := map[int64]otData{1: {otDays: 10, avgHours: 3}}         // +25
	la := map[int64]leaveAvoidData{1: {earned: 15, used: 0}}     // +20
	lh := map[int64]hoursData{1: {avgDaily: 11, workdays: 20}}   // +20
	wk := map[int64]int64{1: 5}                                   // +15
	irr := map[int64]float64{1: 65}                                // +20

	results := computeScores(emps, ot, la, lh, wk, irr)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// 25+20+20+15+20 = 100 (capped)
	if results[0].BurnoutScore != 100 {
		t.Errorf("score = %d, want 100 (capped)", results[0].BurnoutScore)
	}
	if len(results[0].Factors) != 5 {
		t.Errorf("factors = %d, want 5", len(results[0].Factors))
	}
}

func TestComputeScores_CapAt100(t *testing.T) {
	emps := []employeeInfo{makeEmployee(1)}
	// All max factors: 25+20+20+15+20 = 100
	ot := map[int64]otData{1: {otDays: 15, avgHours: 5}}
	la := map[int64]leaveAvoidData{1: {earned: 20, used: 0}}
	lh := map[int64]hoursData{1: {avgDaily: 14, workdays: 25}}
	wk := map[int64]int64{1: 10}
	irr := map[int64]float64{1: 120}

	results := computeScores(emps, ot, la, lh, wk, irr)

	if results[0].BurnoutScore > 100 {
		t.Errorf("score = %d, should be capped at 100", results[0].BurnoutScore)
	}
}

func TestComputeScores_MultipleEmployees(t *testing.T) {
	emps := []employeeInfo{makeEmployee(1), makeEmployee(2), makeEmployee(3)}
	ot := map[int64]otData{1: {otDays: 10, avgHours: 3}} // only emp 1

	results := computeScores(emps, ot, nil, nil, nil, nil)

	if len(results) != 1 {
		t.Errorf("expected 1 result (only emp 1 has signals), got %d", len(results))
	}
	if results[0].EmployeeID != 1 {
		t.Errorf("result employee_id = %d, want 1", results[0].EmployeeID)
	}
}

func TestComputeScores_EmployeeNameAndDept(t *testing.T) {
	emp := employeeInfo{id: 1, employeeNo: "E-100", firstName: "Maria", lastName: "Santos", department: "HR"}
	ot := map[int64]otData{1: {otDays: 10, avgHours: 2}}

	results := computeScores([]employeeInfo{emp}, ot, nil, nil, nil, nil)

	if results[0].Name != "Maria Santos" {
		t.Errorf("name = %q, want %q", results[0].Name, "Maria Santos")
	}
	if results[0].Department != "HR" {
		t.Errorf("department = %q, want %q", results[0].Department, "HR")
	}
	if results[0].EmployeeNo != "E-100" {
		t.Errorf("employee_no = %q, want %q", results[0].EmployeeNo, "E-100")
	}
}
