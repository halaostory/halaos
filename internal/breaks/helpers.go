package breaks

import "time"

// validBreakTypes lists the allowed break type values.
var validBreakTypes = map[string]bool{
	"meal":       true,
	"bathroom":   true,
	"rest":       true,
	"leave_post": true,
}

// parseDateRange parses a date string (YYYY-MM-DD) into start-of-day and end-of-day UTC times.
// If dateStr is empty, returns today's range.
func parseDateRange(dateStr string) (time.Time, time.Time, error) {
	loc := time.UTC
	if dateStr == "" {
		now := time.Now().In(loc)
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		return today, today.Add(24 * time.Hour), nil
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return t, t.Add(24 * time.Hour), nil
}
