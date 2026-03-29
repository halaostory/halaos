package auth

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/store"
)

// countryDefaults holds country-specific company settings.
type countryDefaults struct {
	Country      string
	Currency     string
	Timezone     string
	PayFrequency string
}

// countryConfig returns defaults for a given country code.
func countryConfig(country string) countryDefaults {
	switch country {
	case "LKA":
		return countryDefaults{Country: "LKA", Currency: "LKR", Timezone: "Asia/Colombo", PayFrequency: "monthly"}
	case "SGP":
		return countryDefaults{Country: "SGP", Currency: "SGD", Timezone: "Asia/Singapore", PayFrequency: "monthly"}
	case "IDN":
		return countryDefaults{Country: "IDN", Currency: "IDR", Timezone: "Asia/Jakarta", PayFrequency: "monthly"}
	case "USA":
		return countryDefaults{Country: "USA", Currency: "USD", Timezone: "America/New_York", PayFrequency: "bi_weekly"}
	default:
		return countryDefaults{Country: "PHL", Currency: "PHP", Timezone: "Asia/Manila", PayFrequency: "semi_monthly"}
	}
}

func numericFromFloat(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(f, 'f', 2, 64))
	return n
}

func numericFromRate(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(f, 'f', 4, 64))
	return n
}

// seedCountryDefaults creates country-specific leave types, holidays, and loan types for a new company.
func seedCountryDefaults(ctx context.Context, q *store.Queries, companyID int64, country string) error {
	switch country {
	case "LKA":
		if err := seedLKALoanTypes(ctx, q, companyID); err != nil {
			return err
		}
		return seedLKADefaults(ctx, q, companyID)
	case "USA":
		if err := seedUSALoanTypes(ctx, q, companyID); err != nil {
			return err
		}
		return seedUSADefaults(ctx, q, companyID)
	default:
		if err := seedPHLLoanTypes(ctx, q, companyID); err != nil {
			return err
		}
		return seedPHLDefaults(ctx, q, companyID)
	}
}

func seedLoanTypes(ctx context.Context, q *store.Queries, loanTypes []store.CreateLoanTypeParams) error {
	for _, lt := range loanTypes {
		if _, err := q.CreateLoanType(ctx, lt); err != nil {
			return err
		}
	}
	return nil
}

func seedPHLLoanTypes(ctx context.Context, q *store.Queries, companyID int64) error {
	return seedLoanTypes(ctx, q, []store.CreateLoanTypeParams{
		{CompanyID: companyID, Name: "SSS Salary Loan", Code: "sss_salary", Provider: "government", MaxTermMonths: 24, InterestRate: numericFromRate(0.01), MaxAmount: numericFromFloat(50000), RequiresApproval: true, AutoDeduct: true},
		{CompanyID: companyID, Name: "Pag-IBIG Multi-Purpose", Code: "pagibig_mpl", Provider: "government", MaxTermMonths: 24, InterestRate: numericFromRate(0.0087), MaxAmount: numericFromFloat(80000), RequiresApproval: true, AutoDeduct: true},
		{CompanyID: companyID, Name: "Company Cash Advance", Code: "cash_advance", Provider: "company", MaxTermMonths: 6, InterestRate: numericFromRate(0), MaxAmount: numericFromFloat(20000), RequiresApproval: true, AutoDeduct: true},
		{CompanyID: companyID, Name: "Pag-IBIG Housing Loan", Code: "pagibig_hdp", Provider: "government", MaxTermMonths: 240, InterestRate: numericFromRate(0.0058), MaxAmount: numericFromFloat(300000), RequiresApproval: true, AutoDeduct: true},
		{CompanyID: companyID, Name: "SSS Calamity Loan", Code: "sss_calamity", Provider: "government", MaxTermMonths: 24, InterestRate: numericFromRate(0.01), MaxAmount: numericFromFloat(30000), RequiresApproval: true, AutoDeduct: true},
	})
}

func seedLKALoanTypes(ctx context.Context, q *store.Queries, companyID int64) error {
	return seedLoanTypes(ctx, q, []store.CreateLoanTypeParams{
		{CompanyID: companyID, Name: "Company Cash Advance", Code: "cash_advance", Provider: "company", MaxTermMonths: 6, InterestRate: numericFromRate(0), MaxAmount: numericFromFloat(500000), RequiresApproval: true, AutoDeduct: true},
		{CompanyID: companyID, Name: "Salary Advance", Code: "salary_advance", Provider: "company", MaxTermMonths: 1, InterestRate: numericFromRate(0), MaxAmount: numericFromFloat(100000), RequiresApproval: true, AutoDeduct: true},
		{CompanyID: companyID, Name: "Festival Advance", Code: "festival_advance", Provider: "company", MaxTermMonths: 3, InterestRate: numericFromRate(0), MaxAmount: numericFromFloat(50000), RequiresApproval: true, AutoDeduct: true},
		{CompanyID: companyID, Name: "Staff Loan", Code: "staff_loan", Provider: "company", MaxTermMonths: 36, InterestRate: numericFromRate(0.01), MaxAmount: numericFromFloat(1000000), RequiresApproval: true, AutoDeduct: true},
		{CompanyID: companyID, Name: "Housing Loan", Code: "housing_loan", Provider: "company", MaxTermMonths: 60, InterestRate: numericFromRate(0.0075), MaxAmount: numericFromFloat(3000000), RequiresApproval: true, AutoDeduct: true},
	})
}

func seedUSALoanTypes(ctx context.Context, q *store.Queries, companyID int64) error {
	return seedLoanTypes(ctx, q, []store.CreateLoanTypeParams{
		{CompanyID: companyID, Name: "401(k) Loan", Code: "401k_loan", Provider: "government", MaxTermMonths: 60, InterestRate: numericFromRate(0.0042), MaxAmount: numericFromFloat(50000), RequiresApproval: true, AutoDeduct: true},
		{CompanyID: companyID, Name: "Company Cash Advance", Code: "cash_advance", Provider: "company", MaxTermMonths: 6, InterestRate: numericFromRate(0), MaxAmount: numericFromFloat(5000), RequiresApproval: true, AutoDeduct: true},
		{CompanyID: companyID, Name: "Emergency Loan", Code: "emergency_loan", Provider: "company", MaxTermMonths: 12, InterestRate: numericFromRate(0), MaxAmount: numericFromFloat(2500), RequiresApproval: true, AutoDeduct: true},
	})
}

func seedLKADefaults(ctx context.Context, q *store.Queries, companyID int64) error {
	female := "female"

	// Sri Lanka statutory leave types
	leaveTypes := []store.CreateLeaveTypeParams{
		{CompanyID: companyID, Code: "AL", Name: "Annual Leave", IsPaid: true, DefaultDays: numericFromFloat(14), IsConvertible: false, AccrualType: "yearly", IsStatutory: true},
		{CompanyID: companyID, Code: "CL", Name: "Casual Leave", IsPaid: true, DefaultDays: numericFromFloat(7), IsConvertible: false, AccrualType: "yearly", IsStatutory: true},
		{CompanyID: companyID, Code: "SL", Name: "Sick Leave", IsPaid: true, DefaultDays: numericFromFloat(7), RequiresAttachment: true, AccrualType: "yearly", IsStatutory: true},
		{CompanyID: companyID, Code: "ML", Name: "Maternity Leave", IsPaid: true, DefaultDays: numericFromFloat(84), GenderSpecific: &female, AccrualType: "none", IsStatutory: true},
	}

	for _, lt := range leaveTypes {
		if _, err := q.CreateLeaveType(ctx, lt); err != nil {
			return err
		}
	}

	// Sri Lanka public holidays 2025–2026 (Poya days + national holidays)
	holidays := []struct {
		Name string
		Date string
		Type string
		Year int32
	}{
		// 2025
		{"Thai Pongal / Tamil Thai Pongal Day", "2025-01-14", "regular", 2025},
		{"Duruthu Full Moon Poya Day", "2025-01-13", "regular", 2025},
		{"National Day (Independence Day)", "2025-02-04", "regular", 2025},
		{"Navam Full Moon Poya Day", "2025-02-12", "regular", 2025},
		{"Mahasivarathri Day", "2025-02-26", "regular", 2025},
		{"Medin Full Moon Poya Day", "2025-03-13", "regular", 2025},
		{"Bak Full Moon Poya Day", "2025-04-12", "regular", 2025},
		{"Sinhala and Tamil New Year", "2025-04-13", "regular", 2025},
		{"Sinhala and Tamil New Year (Day 2)", "2025-04-14", "regular", 2025},
		{"Good Friday", "2025-04-18", "regular", 2025},
		{"May Day", "2025-05-01", "regular", 2025},
		{"Vesak Full Moon Poya Day", "2025-05-12", "regular", 2025},
		{"Day after Vesak", "2025-05-13", "regular", 2025},
		{"Poson Full Moon Poya Day", "2025-06-11", "regular", 2025},
		{"Esala Full Moon Poya Day", "2025-07-10", "regular", 2025},
		{"Nikini Full Moon Poya Day", "2025-08-09", "regular", 2025},
		{"Binara Full Moon Poya Day", "2025-09-07", "regular", 2025},
		{"Vap Full Moon Poya Day", "2025-10-06", "regular", 2025},
		{"Deepavali", "2025-10-20", "regular", 2025},
		{"Il Full Moon Poya Day", "2025-11-05", "regular", 2025},
		{"Unduvap Full Moon Poya Day", "2025-12-04", "regular", 2025},
		{"Christmas Day", "2025-12-25", "regular", 2025},
		// 2026
		{"Thai Pongal / Tamil Thai Pongal Day", "2026-01-14", "regular", 2026},
		{"Duruthu Full Moon Poya Day", "2026-01-02", "regular", 2026},
		{"National Day (Independence Day)", "2026-02-04", "regular", 2026},
		{"Navam Full Moon Poya Day", "2026-02-01", "regular", 2026},
		{"Medin Full Moon Poya Day", "2026-03-03", "regular", 2026},
		{"Good Friday", "2026-04-03", "regular", 2026},
		{"Bak Full Moon Poya Day", "2026-04-01", "regular", 2026},
		{"Sinhala and Tamil New Year", "2026-04-13", "regular", 2026},
		{"Sinhala and Tamil New Year (Day 2)", "2026-04-14", "regular", 2026},
		{"May Day / Vesak Full Moon Poya Day", "2026-05-01", "regular", 2026},
		{"Day after Vesak", "2026-05-02", "regular", 2026},
		{"Poson Full Moon Poya Day", "2026-05-31", "regular", 2026},
		{"Esala Full Moon Poya Day", "2026-06-29", "regular", 2026},
		{"Nikini Full Moon Poya Day", "2026-07-29", "regular", 2026},
		{"Binara Full Moon Poya Day", "2026-08-27", "regular", 2026},
		{"Vap Full Moon Poya Day", "2026-09-26", "regular", 2026},
		{"Deepavali", "2026-11-08", "regular", 2026},
		{"Il Full Moon Poya Day", "2026-10-25", "regular", 2026},
		{"Unduvap Full Moon Poya Day", "2026-11-24", "regular", 2026},
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

func seedPHLDefaults(ctx context.Context, q *store.Queries, companyID int64) error {
	female := "female"
	male := "male"

	// Philippine statutory leave types
	leaveTypes := []store.CreateLeaveTypeParams{
		{CompanyID: companyID, Code: "SIL", Name: "Service Incentive Leave", IsPaid: true, DefaultDays: numericFromFloat(5), IsConvertible: true, AccrualType: "yearly", IsStatutory: true},
		{CompanyID: companyID, Code: "ML", Name: "Maternity Leave", IsPaid: true, DefaultDays: numericFromFloat(105), GenderSpecific: &female, AccrualType: "none", IsStatutory: true},
		{CompanyID: companyID, Code: "PL", Name: "Paternity Leave", IsPaid: true, DefaultDays: numericFromFloat(7), GenderSpecific: &male, AccrualType: "none", IsStatutory: true},
		{CompanyID: companyID, Code: "SPL", Name: "Solo Parent Leave", IsPaid: true, DefaultDays: numericFromFloat(7), AccrualType: "yearly", IsStatutory: true},
		{CompanyID: companyID, Code: "VAWC", Name: "VAWC Leave", IsPaid: true, DefaultDays: numericFromFloat(10), GenderSpecific: &female, AccrualType: "none", IsStatutory: true},
	}

	for _, lt := range leaveTypes {
		if _, err := q.CreateLeaveType(ctx, lt); err != nil {
			return err
		}
	}

	// Philippine holidays 2025-2026 (abbreviated — key holidays)
	holidays := []struct {
		Name string
		Date string
		Type string
		Year int32
	}{
		// 2025 Regular Holidays
		{"New Year's Day", "2025-01-01", "regular", 2025},
		{"Araw ng Kagitingan", "2025-04-09", "regular", 2025},
		{"Maundy Thursday", "2025-04-17", "regular", 2025},
		{"Good Friday", "2025-04-18", "regular", 2025},
		{"Labor Day", "2025-05-01", "regular", 2025},
		{"Independence Day", "2025-06-12", "regular", 2025},
		{"National Heroes Day", "2025-08-25", "regular", 2025},
		{"Bonifacio Day", "2025-11-30", "regular", 2025},
		{"Christmas Day", "2025-12-25", "regular", 2025},
		{"Rizal Day", "2025-12-30", "regular", 2025},
		// 2025 Special Non-Working
		{"EDSA Revolution Anniversary", "2025-02-25", "special_non_working", 2025},
		{"Black Saturday", "2025-04-19", "special_non_working", 2025},
		{"Ninoy Aquino Day", "2025-08-21", "special_non_working", 2025},
		{"All Saints' Day", "2025-11-01", "special_non_working", 2025},
		{"Last Day of the Year", "2025-12-31", "special_non_working", 2025},
		// 2026 Regular Holidays
		{"New Year's Day", "2026-01-01", "regular", 2026},
		{"Araw ng Kagitingan", "2026-04-09", "regular", 2026},
		{"Maundy Thursday", "2026-04-02", "regular", 2026},
		{"Good Friday", "2026-04-03", "regular", 2026},
		{"Labor Day", "2026-05-01", "regular", 2026},
		{"Independence Day", "2026-06-12", "regular", 2026},
		{"National Heroes Day", "2026-08-31", "regular", 2026},
		{"Bonifacio Day", "2026-11-30", "regular", 2026},
		{"Christmas Day", "2026-12-25", "regular", 2026},
		{"Rizal Day", "2026-12-30", "regular", 2026},
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
