package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Role string

const (
	RoleSuperAdmin Role = "super_admin"
	RoleAdmin      Role = "admin"
	RoleManager    Role = "manager"
	RoleEmployee   Role = "employee"
)

type Claims struct {
	UserID    int64  `json:"user_id"`
	Email     string `json:"email"`
	Role      Role   `json:"role"`
	CompanyID int64  `json:"company_id"`
	jwt.RegisteredClaims
}

type JWTService struct {
	secret        string
	expiry        time.Duration
	refreshExpiry time.Duration
	issuer        string
}

func NewJWTService(secret string, expiry, refreshExpiry time.Duration) *JWTService {
	return &JWTService{
		secret:        secret,
		expiry:        expiry,
		refreshExpiry: refreshExpiry,
		issuer:        "halaos",
	}
}

func (s *JWTService) GenerateToken(userID int64, email string, role Role, companyID int64) (string, error) {
	claims := Claims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		CompanyID: companyID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			Issuer:    s.issuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secret))
}

func (s *JWTService) GenerateRefreshToken(userID int64, email string, role Role, companyID int64) (string, error) {
	claims := Claims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		CompanyID: companyID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			Issuer:    s.issuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.refreshExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secret))
}

func (s *JWTService) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}
