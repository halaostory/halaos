package teamhealth

import (
	"testing"
)

func TestComputeScores_NoData(t *testing.T) {
	depts := []deptInfo{{id: 1, name: "Engineering"}}
	results := computeScores(depts, nil, nil, nil, nil, nil)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// All defaults: 15 + 15 + 18 + 20 + 20 = 88
	if results[0].HealthScore != 88 {
		t.Errorf("score = %d, want 88 (all defaults)", results[0].HealthScore)
	}
	if len(results[0].Factors) != 5 {
		t.Errorf("factors = %d, want 5", len(results[0].Factors))
	}
}

func TestComputeScores_PerfectHealth(t *testing.T) {
	depts := []deptInfo{{id: 1, name: "Engineering"}}
	att := map[int64]attendanceInfo{1: {onTime: 100, total: 100, rate: 1.0}}
	leave := map[int64]float64{1: 1.0}    // 100% remaining
	ot := map[int64]float64{1: 0}          // no OT
	turn := map[int64]int64{1: 0}          // no separations
	griev := map[int64]int64{1: 0}         // no grievances

	results := computeScores(depts, att, leave, ot, turn, griev)

	// 20 + 20 + 20 + 20 + 20 = 100
	if results[0].HealthScore != 100 {
		t.Errorf("score = %d, want 100 (perfect)", results[0].HealthScore)
	}
}

func TestComputeScores_AttendanceRate(t *testing.T) {
	tests := []struct {
		name string
		rate float64
		want int
	}{
		{"perfect", 1.0, 20},
		{"good", 0.9, 18},
		{"mediocre", 0.5, 10},
		{"bad", 0.0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			depts := []deptInfo{{id: 1, name: "Test"}}
			att := map[int64]attendanceInfo{1: {rate: tc.rate}}

			results := computeScores(depts, att, nil, nil, nil, nil)
			// Find attendance factor
			for _, f := range results[0].Factors {
				if f.Name == "attendance" {
					if f.Score != tc.want {
						t.Errorf("rate=%.1f: attendance score = %d, want %d", tc.rate, f.Score, tc.want)
					}
					return
				}
			}
			t.Error("attendance factor not found")
		})
	}
}

func TestComputeScores_OvertimeHealth(t *testing.T) {
	tests := []struct {
		name    string
		otHours float64
		want    int
	}{
		{"no OT", 0, 20},
		{"moderate OT", 5, 10},
		{"high OT", 10, 0},
		{"extreme OT", 15, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			depts := []deptInfo{{id: 1, name: "Test"}}
			ot := map[int64]float64{1: tc.otHours}

			results := computeScores(depts, nil, nil, ot, nil, nil)
			for _, f := range results[0].Factors {
				if f.Name == "overtime" {
					if f.Score != tc.want {
						t.Errorf("otHours=%.1f: overtime score = %d, want %d", tc.otHours, f.Score, tc.want)
					}
					return
				}
			}
			t.Error("overtime factor not found")
		})
	}
}

func TestComputeScores_TurnoverHealth(t *testing.T) {
	tests := []struct {
		name     string
		sepCount int64
		want     int
	}{
		{"no separations", 0, 20},
		{"1-2 separations", 2, 15},
		{"3-4 separations", 4, 8},
		{"5+ separations", 7, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			depts := []deptInfo{{id: 1, name: "Test"}}
			turn := map[int64]int64{1: tc.sepCount}

			results := computeScores(depts, nil, nil, nil, turn, nil)
			for _, f := range results[0].Factors {
				if f.Name == "turnover" {
					if f.Score != tc.want {
						t.Errorf("separations=%d: turnover score = %d, want %d", tc.sepCount, f.Score, tc.want)
					}
					return
				}
			}
			t.Error("turnover factor not found")
		})
	}
}

func TestComputeScores_GrievanceHealth(t *testing.T) {
	tests := []struct {
		name   string
		gCount int64
		want   int
	}{
		{"no grievances", 0, 20},
		{"1-2 grievances", 2, 14},
		{"3-4 grievances", 4, 6},
		{"5+ grievances", 8, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			depts := []deptInfo{{id: 1, name: "Test"}}
			griev := map[int64]int64{1: tc.gCount}

			results := computeScores(depts, nil, nil, nil, nil, griev)
			for _, f := range results[0].Factors {
				if f.Name == "grievances" {
					if f.Score != tc.want {
						t.Errorf("grievances=%d: score = %d, want %d", tc.gCount, f.Score, tc.want)
					}
					return
				}
			}
			t.Error("grievances factor not found")
		})
	}
}

func TestComputeScores_MultipleDepartments(t *testing.T) {
	depts := []deptInfo{
		{id: 1, name: "Engineering"},
		{id: 2, name: "Sales"},
		{id: 3, name: "HR"},
	}

	att := map[int64]attendanceInfo{
		1: {rate: 1.0},
		2: {rate: 0.5},
	}
	// dept 3 has no data

	results := computeScores(depts, att, nil, nil, nil, nil)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Verify scores differ
	if results[0].HealthScore == results[1].HealthScore {
		t.Error("dept 1 (perfect) and dept 2 (bad) should have different scores")
	}
}

func TestComputeScores_LeaveBalance(t *testing.T) {
	tests := []struct {
		name      string
		remaining float64
		want      int
	}{
		{"all remaining", 1.0, 20},
		{"half remaining", 0.5, 10},
		{"none remaining", 0.0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			depts := []deptInfo{{id: 1, name: "Test"}}
			leave := map[int64]float64{1: tc.remaining}

			results := computeScores(depts, nil, leave, nil, nil, nil)
			for _, f := range results[0].Factors {
				if f.Name == "leave_balance" {
					if f.Score != tc.want {
						t.Errorf("remaining=%.1f: leave score = %d, want %d", tc.remaining, f.Score, tc.want)
					}
					return
				}
			}
			t.Error("leave_balance factor not found")
		})
	}
}

func TestComputeScores_WorstCase(t *testing.T) {
	depts := []deptInfo{{id: 1, name: "Troubled"}}
	att := map[int64]attendanceInfo{1: {rate: 0}}
	leave := map[int64]float64{1: 0}
	ot := map[int64]float64{1: 20}    // extreme OT
	turn := map[int64]int64{1: 10}
	griev := map[int64]int64{1: 10}

	results := computeScores(depts, att, leave, ot, turn, griev)

	// 0 + 0 + 0 + 0 + 0 = 0
	if results[0].HealthScore != 0 {
		t.Errorf("score = %d, want 0 (worst case)", results[0].HealthScore)
	}
}

func TestComputeScores_DeptNamePreserved(t *testing.T) {
	depts := []deptInfo{{id: 42, name: "Finance Department"}}
	results := computeScores(depts, nil, nil, nil, nil, nil)

	if results[0].DepartmentID != 42 {
		t.Errorf("dept ID = %d, want 42", results[0].DepartmentID)
	}
	if results[0].DepartmentName != "Finance Department" {
		t.Errorf("dept name = %q, want %q", results[0].DepartmentName, "Finance Department")
	}
}
