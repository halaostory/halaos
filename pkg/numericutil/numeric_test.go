package numericutil

import (
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestToFloat_Valid(t *testing.T) {
	n := pgtype.Numeric{Valid: true, Int: big.NewInt(12345), Exp: -2} // 123.45
	got := ToFloat(n)
	if got != 123.45 {
		t.Errorf("ToFloat(123.45) = %v, want 123.45", got)
	}
}

func TestToFloat_Zero(t *testing.T) {
	n := pgtype.Numeric{Valid: true, Int: big.NewInt(0), Exp: 0}
	got := ToFloat(n)
	if got != 0 {
		t.Errorf("ToFloat(0) = %v, want 0", got)
	}
}

func TestToFloat_Invalid(t *testing.T) {
	n := pgtype.Numeric{Valid: false}
	got := ToFloat(n)
	if got != 0 {
		t.Errorf("ToFloat(invalid) = %v, want 0", got)
	}
}

func TestToFloat_LargeNumber(t *testing.T) {
	n := pgtype.Numeric{Valid: true, Int: big.NewInt(5000000), Exp: -2} // 50000.00
	got := ToFloat(n)
	if got != 50000.0 {
		t.Errorf("ToFloat(50000) = %v, want 50000", got)
	}
}

func TestToFloat_Negative(t *testing.T) {
	n := pgtype.Numeric{Valid: true, Int: big.NewInt(-999), Exp: -1} // -99.9
	got := ToFloat(n)
	if got != -99.9 {
		t.Errorf("ToFloat(-99.9) = %v, want -99.9", got)
	}
}
