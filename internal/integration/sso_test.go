package integration

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateFinanceToken_Valid(t *testing.T) {
	secret := "test-secret-32-chars-long-padded!"
	svc := NewSSOService(secret)

	claims := &FinanceToHRClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "aistarlight",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        "sso-test-123",
		},
		Email:            "user@test.com",
		FinanceCompanyID: "550e8400-e29b-41d4-a716-446655440000",
		FinanceUserID:    "660e8400-e29b-41d4-a716-446655440001",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	result, err := svc.ValidateFinanceToken(tokenStr)
	require.NoError(t, err)
	assert.Equal(t, "user@test.com", result.Email)
	assert.Equal(t, "aistarlight", result.Issuer)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", result.FinanceCompanyID)
	assert.Equal(t, "660e8400-e29b-41d4-a716-446655440001", result.FinanceUserID)
}

func TestValidateFinanceToken_WrongIssuer(t *testing.T) {
	secret := "test-secret-32-chars-long-padded!"
	svc := NewSSOService(secret)

	claims := &FinanceToHRClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "aigonhr",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
		Email: "user@test.com",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(secret))

	_, err := svc.ValidateFinanceToken(tokenStr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "issuer")
}

func TestValidateFinanceToken_Expired(t *testing.T) {
	secret := "test-secret-32-chars-long-padded!"
	svc := NewSSOService(secret)

	claims := &FinanceToHRClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "aistarlight",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
		},
		Email: "user@test.com",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(secret))

	_, err := svc.ValidateFinanceToken(tokenStr)
	assert.Error(t, err)
}

func TestValidateFinanceToken_EmptySecret(t *testing.T) {
	svc := NewSSOService("")
	_, err := svc.ValidateFinanceToken("some-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}
