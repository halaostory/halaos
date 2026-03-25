package auth

import (
	"context"
	"time"

	"github.com/tonypk/aigonhr/internal/store"
)

func seedUSADefaults(ctx context.Context, q *store.Queries, companyID int64) error {
	leaveTypes := []store.CreateLeaveTypeParams{
		{CompanyID: companyID, Code: "PTO", Name: "Paid Time Off", IsPaid: true, DefaultDays: numericFromFloat(15), IsConvertible: false, AccrualType: "yearly", IsStatutory: false},
		{CompanyID: companyID, Code: "SL", Name: "Sick Leave", IsPaid: true, DefaultDays: numericFromFloat(5), IsConvertible: false, AccrualType: "yearly", IsStatutory: false},
		{CompanyID: companyID, Code: "FMLA", Name: "Family and Medical Leave", IsPaid: false, DefaultDays: numericFromFloat(60), IsConvertible: false, AccrualType: "none", IsStatutory: true},
		{CompanyID: companyID, Code: "BRV", Name: "Bereavement Leave", IsPaid: true, DefaultDays: numericFromFloat(3), IsConvertible: false, AccrualType: "none", IsStatutory: false},
		{CompanyID: companyID, Code: "JD", Name: "Jury Duty", IsPaid: true, DefaultDays: numericFromFloat(5), IsConvertible: false, AccrualType: "none", IsStatutory: false},
	}

	for _, lt := range leaveTypes {
		if _, err := q.CreateLeaveType(ctx, lt); err != nil {
			return err
		}
	}

	holidays := []struct {
		Name string
		Date string
		Type string
		Year int32
	}{
		{"New Year's Day", "2025-01-01", "regular", 2025},
		{"Martin Luther King Jr. Day", "2025-01-20", "regular", 2025},
		{"Presidents' Day", "2025-02-17", "regular", 2025},
		{"Memorial Day", "2025-05-26", "regular", 2025},
		{"Juneteenth", "2025-06-19", "regular", 2025},
		{"Independence Day", "2025-07-04", "regular", 2025},
		{"Labor Day", "2025-09-01", "regular", 2025},
		{"Columbus Day", "2025-10-13", "regular", 2025},
		{"Veterans Day", "2025-11-11", "regular", 2025},
		{"Thanksgiving", "2025-11-27", "regular", 2025},
		{"Christmas Day", "2025-12-25", "regular", 2025},
		{"New Year's Day", "2026-01-01", "regular", 2026},
		{"Martin Luther King Jr. Day", "2026-01-19", "regular", 2026},
		{"Presidents' Day", "2026-02-16", "regular", 2026},
		{"Memorial Day", "2026-05-25", "regular", 2026},
		{"Juneteenth", "2026-06-19", "regular", 2026},
		{"Independence Day", "2026-07-04", "regular", 2026},
		{"Labor Day", "2026-09-07", "regular", 2026},
		{"Columbus Day", "2026-10-12", "regular", 2026},
		{"Veterans Day", "2026-11-11", "regular", 2026},
		{"Thanksgiving", "2026-11-26", "regular", 2026},
		{"Christmas Day", "2026-12-25", "regular", 2026},
	}

	for _, h := range holidays {
		d, _ := time.Parse("2006-01-02", h.Date)
		if _, err := q.CreateHoliday(ctx, store.CreateHolidayParams{
			CompanyID:    companyID,
			Name:         h.Name,
			HolidayDate:  d,
			HolidayType:  h.Type,
			Year:         h.Year,
			IsNationwide: true,
		}); err != nil {
			return err
		}
	}

	return nil
}
