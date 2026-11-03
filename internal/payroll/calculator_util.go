package payroll

import (
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

func countWorkingDays(start, end time.Time) int {
	days := 0
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			days++
		}
	}
	if days == 0 {
		days = 1
	}
	return days
}

func numericFromFloat(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(f, 'f', 2, 64))
	return n
}

func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, _ := n.Float64Value()
	if !f.Valid {
		return 0
	}
	return f.Float64
}

func interfaceToFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case []byte:
		f, _ := strconv.ParseFloat(string(val), 64)
		return f
	default:
		return 0
	}
}

func interfaceToInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int64:
		return val
	case int32:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		i, _ := strconv.ParseInt(val, 10, 64)
		return i
	case []byte:
		i, _ := strconv.ParseInt(string(val), 10, 64)
		return i
	default:
		return 0
	}
}

func pgTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// periodFraction is the share of a monthly salary paid in one period.
func periodFraction(frequency string) float64 {
	switch frequency {
	case "semi_monthly":
		return 0.5
	case "bi_weekly":
		return 12.0 / 26.0
	case "weekly":
		return 12.0 / 52.0
	default:
		return 1
	}
}

// periodTax computes withholding for a period with a monthly table: the
// period's taxable income is converted to a monthly equivalent, taxed, and the
// tax scaled back (the BIR semi-monthly table is exactly half the monthly one).
func periodTax(taxable, frac float64, monthlyTax func(monthlyTaxable float64) float64) float64 {
	if taxable <= 0 || frac <= 0 {
		return 0
	}
	return round2(monthlyTax(taxable/frac) * frac)
}

// scheduleScope decides which employees a cycle pays and at what frequency.
type scheduleScope struct {
	Frequency  string
	ScheduleID int64 // 0 = no schedules configured: pay every active employee
	IsDefault  bool  // unassigned employees belong to the default schedule
}

// includes reports whether an employee (assigned schedule, 0 = none) is paid in this scope.
func (s scheduleScope) includes(assigned int64) bool {
	if s.ScheduleID == 0 {
		return true
	}
	if assigned == 0 {
		return s.IsDefault
	}
	return assigned == s.ScheduleID
}
