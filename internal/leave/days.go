package leave

import (
	"context"
	"time"

	"github.com/halaostory/halaos/internal/store"
)

// workingDays counts the days of [start, end] that are not weekends or
// company non-working holidays (regular and special non-working).
func workingDays(start, end time.Time, off map[string]bool) float64 {
	n := 0.0
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
			continue
		}
		if off[d.Format("2006-01-02")] {
			continue
		}
		n++
	}
	return n
}

// holidaysOff returns the company's non-working holiday dates in range.
func holidaysOff(ctx context.Context, q *store.Queries, companyID int64, start, end time.Time) map[string]bool {
	off := map[string]bool{}
	rows, err := q.ListHolidaysInPeriod(ctx, store.ListHolidaysInPeriodParams{CompanyID: companyID, HolidayDate: start, HolidayDate_2: end})
	if err != nil {
		return off
	}
	for _, h := range rows {
		if h.HolidayType == "regular" || h.HolidayType == "special_non_working" {
			off[h.HolidayDate.Format("2006-01-02")] = true
		}
	}
	return off
}
