package leave

import (
	"testing"
	"time"
)

func TestWorkingDays(t *testing.T) {
	d := func(s string) time.Time { v, _ := time.Parse("2006-01-02", s); return v }
	// Mon 2026-11-02 .. Sun 2026-11-08: 5 weekdays, one is a holiday.
	if got := workingDays(d("2026-11-02"), d("2026-11-08"), map[string]bool{"2026-11-04": true}); got != 4 {
		t.Fatalf("got %v want 4", got)
	}
	if got := workingDays(d("2026-11-07"), d("2026-11-08"), nil); got != 0 {
		t.Fatalf("weekend only: got %v", got)
	}
	if got := workingDays(d("2026-11-02"), d("2026-11-02"), nil); got != 1 {
		t.Fatalf("one day: got %v", got)
	}
}
