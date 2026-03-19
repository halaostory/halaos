package testutil

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/store"
)

// FixtureUser returns a store.User with sensible defaults.
// Override fields after creation as needed.
func FixtureUser() store.User {
	return store.User{
		ID:                         1,
		CompanyID:                  1,
		Email:                      "admin@test.com",
		PasswordHash:               "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012", // dummy hash
		FirstName:                  "Admin",
		LastName:                   "User",
		Role:                       "admin",
		Status:                     "active",
		AvatarUrl:                  nil,
		Locale:                     "en",
		LastLoginAt:                pgtype.Timestamptz{},
		CreatedAt:                  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:                  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EmailVerified:              true,
		VerificationToken:          nil,
		VerificationTokenExpiresAt: pgtype.Timestamptz{},
	}
}

// UserScanValues returns the values in the exact scan order used by sqlc-generated
// User queries (16 fields).
func UserScanValues(u store.User) []interface{} {
	return []interface{}{
		u.ID,
		u.CompanyID,
		u.Email,
		u.PasswordHash,
		u.FirstName,
		u.LastName,
		u.Role,
		u.Status,
		u.AvatarUrl,
		u.Locale,
		u.LastLoginAt,
		u.CreatedAt,
		u.UpdatedAt,
		u.EmailVerified,
		u.VerificationToken,
		u.VerificationTokenExpiresAt,
	}
}

// UserRowsData converts a slice of Users to row data for StaticRows.
func UserRowsData(users []store.User) [][]interface{} {
	rows := make([][]interface{}, len(users))
	for i, u := range users {
		rows[i] = UserScanValues(u)
	}
	return rows
}

// FixtureEmployee returns a store.Employee with sensible defaults (27 fields).
func FixtureEmployee() store.Employee {
	return store.Employee{
		ID:                 1,
		CompanyID:          1,
		UserID:             nil,
		EmployeeNo:         "EMP-001",
		FirstName:          "John",
		LastName:           "Doe",
		MiddleName:         nil,
		Suffix:             nil,
		DisplayName:        nil,
		Email:              strPtr("john@test.com"),
		Phone:              nil,
		BirthDate:          pgtype.Date{},
		Gender:             nil,
		CivilStatus:        nil,
		Nationality:        nil,
		DepartmentID:       nil,
		PositionID:         nil,
		CostCenterID:       nil,
		ManagerID:          nil,
		HireDate:           time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		RegularizationDate: pgtype.Date{},
		SeparationDate:     pgtype.Date{},
		EmploymentType:     "regular",
		Status:             "active",
		CreatedAt:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		ContractEndDate:    pgtype.Date{},
	}
}

// EmployeeScanValues returns values in exact scan order (27 fields).
func EmployeeScanValues(e store.Employee) []interface{} {
	return []interface{}{
		e.ID, e.CompanyID, e.UserID, e.EmployeeNo,
		e.FirstName, e.LastName, e.MiddleName, e.Suffix,
		e.DisplayName, e.Email, e.Phone, e.BirthDate,
		e.Gender, e.CivilStatus, e.Nationality,
		e.DepartmentID, e.PositionID, e.CostCenterID, e.ManagerID,
		e.HireDate, e.RegularizationDate, e.SeparationDate,
		e.EmploymentType, e.Status,
		e.CreatedAt, e.UpdatedAt, e.ContractEndDate,
	}
}

// EmployeeRowsData converts a slice of Employees to row data.
func EmployeeRowsData(employees []store.Employee) [][]interface{} {
	rows := make([][]interface{}, len(employees))
	for i, e := range employees {
		rows[i] = EmployeeScanValues(e)
	}
	return rows
}

func strPtr(s string) *string { return &s }
