package integration

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// CrossAppClaims represents a cross-app SSO token payload.
type CrossAppClaims struct {
	jwt.RegisteredClaims
	HRCompanyID int64  `json:"hr_company_id"`
	HRUserID    int64  `json:"hr_user_id"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	FirstName   string `json:"first_name,omitempty"`
	LastName    string `json:"last_name,omitempty"`
}

// FinanceToHRClaims represents a cross-app SSO token from Finance→HR.
// Direction is identified by iss="aistarlight" (vs iss="aigonhr" for HR→Finance).
type FinanceToHRClaims struct {
	jwt.RegisteredClaims
	Email            string `json:"email"`
	FinanceCompanyID string `json:"finance_company_id"`
	FinanceUserID    string `json:"finance_user_id"`
}

// SSOService generates and validates cross-app SSO tokens.
type SSOService struct {
	secret string
}

// NewSSOService creates a new SSO service with the shared integration JWT secret.
func NewSSOService(secret string) *SSOService {
	return &SSOService{secret: secret}
}

// GenerateToken creates a short-lived cross-app JWT for navigating to AIStarlight.
func (s *SSOService) GenerateToken(companyID, userID int64, email, role, firstName, lastName string) (string, error) {
	if s.secret == "" {
		return "", fmt.Errorf("INTEGRATION_JWT_SECRET not configured")
	}

	now := time.Now()
	claims := CrossAppClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "aigonhr",
			Subject:   fmt.Sprintf("%d", userID),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        fmt.Sprintf("sso-%d-%d", userID, now.UnixMilli()),
		},
		HRCompanyID: companyID,
		HRUserID:    userID,
		Email:       email,
		Role:        role,
		FirstName:   firstName,
		LastName:    lastName,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secret))
}

// ValidateToken validates a cross-app JWT (used if AIGoNHR also accepts incoming SSO).
func (s *SSOService) ValidateToken(tokenStr string) (*CrossAppClaims, error) {
	if s.secret == "" {
		return nil, fmt.Errorf("INTEGRATION_JWT_SECRET not configured")
	}

	token, err := jwt.ParseWithClaims(tokenStr, &CrossAppClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*CrossAppClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// ValidateFinanceToken parses and validates a Finance→HR SSO JWT.
func (s *SSOService) ValidateFinanceToken(tokenStr string) (*FinanceToHRClaims, error) {
	if s.secret == "" {
		return nil, fmt.Errorf("INTEGRATION_JWT_SECRET not configured")
	}

	token, err := jwt.ParseWithClaims(tokenStr, &FinanceToHRClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid Finance SSO token: %w", err)
	}

	claims, ok := token.Claims.(*FinanceToHRClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	if claims.Issuer != "aistarlight" {
		return nil, fmt.Errorf("invalid issuer: expected aistarlight, got %s", claims.Issuer)
	}

	return claims, nil
}

// ValidateFinanceEmail validates a Finance→HR SSO token and returns the user email.
// Satisfies auth.FinanceSSOValidator interface.
func (s *SSOService) ValidateFinanceEmail(tokenStr string) (string, error) {
	claims, err := s.ValidateFinanceToken(tokenStr)
	if err != nil {
		return "", err
	}
	return claims.Email, nil
}
