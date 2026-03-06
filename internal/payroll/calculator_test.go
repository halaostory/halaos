package payroll

import (
	"math"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestRound2(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{1.234, 1.23},
		{1.235, 1.24},
		{1.236, 1.24},
		{0, 0},
		{-1.234, -1.23},
		{100.005, 100.01},
		{99999.999, 100000.0},
	}
	for _, tt := range tests {
		got := round2(tt.input)
		if got != tt.expected {
			t.Errorf("round2(%f) = %f, want %f", tt.input, got, tt.expected)
		}
	}
}

func TestCountWorkingDays(t *testing.T) {
	tests := []struct {
		name     string
		start    time.Time
		end      time.Time
		expected int
	}{
		{
			name:     "full week Mon-Fri",
			start:    time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),  // Monday
			end:      time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC),  // Friday
			expected: 5,
		},
		{
			name:     "includes weekend",
			start:    time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),  // Monday
			end:      time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC),  // Sunday
			expected: 5,
		},
		{
			name:     "two weeks",
			start:    time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),  // Monday
			end:      time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC), // Friday
			expected: 10,
		},
		{
			name:     "semi-monthly 1-15",
			start:    time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),  // Sunday
			end:      time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), // Sunday
			expected: 10,
		},
		{
			name:     "full month March 2026",
			start:    time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			end:      time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
			expected: 22,
		},
		{
			name:     "single Saturday",
			start:    time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC),
			end:      time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC),
			expected: 1, // minimum 1
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countWorkingDays(tt.start, tt.end)
			if got != tt.expected {
				t.Errorf("countWorkingDays(%s, %s) = %d, want %d",
					tt.start.Format("2006-01-02"), tt.end.Format("2006-01-02"), got, tt.expected)
			}
		})
	}
}

func TestComputePay(t *testing.T) {
	calc := &Calculator{}

	tests := []struct {
		name               string
		salary             float64
		daysWorked         float64
		workingDays        int
		otHours            float64
		lateMinutes        int64
		undertimeMinutes   int64
		unpaidLeaveDays    float64
		expectedBasicPay   float64
		expectedGrossRange [2]float64 // [min, max] for floating point comparison
	}{
		{
			name:             "full month no OT",
			salary:           30000,
			daysWorked:       22,
			workingDays:      22,
			expectedBasicPay: 30000,
			expectedGrossRange: [2]float64{29999.99, 30000.01},
		},
		{
			name:             "half month",
			salary:           30000,
			daysWorked:       11,
			workingDays:      22,
			expectedBasicPay: 15000,
			expectedGrossRange: [2]float64{14999.99, 15000.01},
		},
		{
			name:             "with overtime",
			salary:           30000,
			daysWorked:       22,
			workingDays:      22,
			otHours:          8,
			expectedBasicPay: 30000,
			expectedGrossRange: [2]float64{31700, 31800}, // 30000 + (30000/22/8*1.25*8) ≈ 31704.55
		},
		{
			name:             "with late deduction",
			salary:           30000,
			daysWorked:       22,
			workingDays:      22,
			lateMinutes:      60, // 1 hour late
			expectedBasicPay: 30000,
			expectedGrossRange: [2]float64{29820, 29835}, // 30000 - (30000/22/8) ≈ 29829.55
		},
		{
			name:             "with unpaid leave",
			salary:           30000,
			daysWorked:       20,
			workingDays:      22,
			unpaidLeaveDays:  2,
			expectedBasicPay: 27272.73, // 30000/22*20
			expectedGrossRange: [2]float64{24540, 24550}, // basicPay - leave deduction
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pd := EmployeePayData{
				BasicSalary:      tt.salary,
				DaysWorked:       tt.daysWorked,
				OvertimeHours:    tt.otHours,
				LateMinutes:      tt.lateMinutes,
				UndertimeMinutes: tt.undertimeMinutes,
				UnpaidLeaveDays:  tt.unpaidLeaveDays,
			}

			calc.computePay(&pd, tt.workingDays)

			if math.Abs(pd.BasicPay-tt.expectedBasicPay) > 0.02 {
				t.Errorf("BasicPay = %.2f, want %.2f", pd.BasicPay, tt.expectedBasicPay)
			}

			if pd.GrossPay < tt.expectedGrossRange[0] || pd.GrossPay > tt.expectedGrossRange[1] {
				t.Errorf("GrossPay = %.2f, want between %.2f and %.2f",
					pd.GrossPay, tt.expectedGrossRange[0], tt.expectedGrossRange[1])
			}
		})
	}
}

func TestNumericConversion(t *testing.T) {
	// Test numericFromFloat and numericToFloat roundtrip
	values := []float64{0, 1.5, 100.99, 12345.67, -50.25}
	for _, v := range values {
		n := numericFromFloat(v)
		got := numericToFloat(n)
		if math.Abs(got-v) > 0.01 {
			t.Errorf("roundtrip(%f) = %f, diff too large", v, got)
		}
	}

	// Test zero value numeric
	var zero pgtype.Numeric
	if numericToFloat(zero) != 0 {
		t.Error("zero numeric should return 0")
	}
}

func TestInterfaceToFloat(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected float64
	}{
		{nil, 0},
		{float64(1.5), 1.5},
		{float32(2.5), 2.5},
		{int64(10), 10},
		{int32(5), 5},
		{"3.14", 3.14},
		{[]byte("2.71"), 2.71},
		{true, 0}, // unsupported type
	}
	for _, tt := range tests {
		got := interfaceToFloat(tt.input)
		if math.Abs(got-tt.expected) > 0.001 {
			t.Errorf("interfaceToFloat(%v) = %f, want %f", tt.input, got, tt.expected)
		}
	}
}

func TestInterfaceToInt64(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected int64
	}{
		{nil, 0},
		{int64(42), 42},
		{int32(10), 10},
		{float64(7.9), 7},
		{"123", 123},
		{[]byte("456"), 456},
	}
	for _, tt := range tests {
		got := interfaceToInt64(tt.input)
		if got != tt.expected {
			t.Errorf("interfaceToInt64(%v) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}
