package numericutil

import "github.com/jackc/pgx/v5/pgtype"

// ToFloat converts a pgtype.Numeric to float64, returning 0 if invalid.
func ToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, _ := n.Float64Value()
	if !f.Valid {
		return 0
	}
	return f.Float64
}
