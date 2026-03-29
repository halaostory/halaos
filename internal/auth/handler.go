package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/halaostory/halaos/internal/email"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// FinanceSSOValidator validates Finance→HR SSO tokens and extracts the user email.
// Implemented by integration.SSOService.
type FinanceSSOValidator interface {
	ValidateFinanceEmail(tokenStr string) (email string, err error)
}

// HashPassword hashes a plain-text password using bcrypt.
func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hashed), err
}

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	jwt     *JWTService
	email   *email.Service
	logger  *slog.Logger
	redis   *redis.Client
	sso     FinanceSSOValidator
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, jwt *JWTService, emailSvc *email.Service, logger *slog.Logger, rdb *redis.Client, sso FinanceSSOValidator) *Handler {
	return &Handler{
		queries: queries,
		pool:    pool,
		jwt:     jwt,
		email:   emailSvc,
		logger:  logger,
		redis:   rdb,
		sso:     sso,
	}
}

// generateVerificationToken creates a secure random hex token.
func generateVerificationToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type registerRequest struct {
	CompanyName  string `json:"company_name"` // optional for magic link; derived from email domain if empty
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password"`     // optional for magic link; random if empty
	FirstName    string `json:"first_name"`   // optional for magic link; "User" if empty
	LastName     string `json:"last_name"`    // optional for magic link; "" if empty
	Country      string `json:"country"`      // PHL (default), LKA, SGP, IDN
	ReferralCode string `json:"referral_code"` // optional referral code from ?ref= link
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type authResponse struct {
	Token        string       `json:"token"`
	RefreshToken string       `json:"refresh_token"`
	User         userResponse `json:"user"`
}

type userResponse struct {
	ID              int64   `json:"id"`
	Email           string  `json:"email"`
	FirstName       string  `json:"first_name"`
	LastName        string  `json:"last_name"`
	Role            string  `json:"role"`
	CompanyID       int64   `json:"company_id"`
	AvatarUrl       *string `json:"avatar_url,omitempty"`
	CompanyCountry  string  `json:"company_country,omitempty"`
	CompanyCurrency string  `json:"company_currency,omitempty"`
	CompanyTimezone string  `json:"company_timezone,omitempty"`
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Check if email already exists
	_, err := h.queries.GetUserByEmail(c.Request.Context(), req.Email)
	if err == nil {
		response.Conflict(c, "Email already registered")
		return
	}

	// Fill defaults for magic-link registration (email-only)
	if req.CompanyName == "" {
		parts := strings.SplitN(req.Email, "@", 2)
		if len(parts) == 2 {
			req.CompanyName = parts[1] // use domain as company name
		} else {
			req.CompanyName = "My Company"
		}
	}
	if req.FirstName == "" {
		req.FirstName = "User"
	}
	if req.Password == "" {
		// Generate random password for magic-link signups
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		req.Password = hex.EncodeToString(b)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	// Create company and user in a transaction
	tx, err := h.pool.Begin(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to begin transaction", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}
	defer tx.Rollback(c.Request.Context())

	qtx := h.queries.WithTx(tx)

	// Resolve country defaults
	country := req.Country
	if country == "" {
		country = "PHL"
	}
	cc := countryConfig(country)

	// Create company with country-specific settings
	company, err := qtx.CreateCompanyWithCountry(c.Request.Context(), store.CreateCompanyWithCountryParams{
		Name:         req.CompanyName,
		Country:      cc.Country,
		Currency:     cc.Currency,
		Timezone:     cc.Timezone,
		PayFrequency: cc.PayFrequency,
	})
	if err != nil {
		h.logger.Error("failed to create company", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	// Create user with super_admin role
	user, err := qtx.CreateUser(c.Request.Context(), store.CreateUserParams{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Role:         string(RoleSuperAdmin),
		CompanyID:    company.ID,
	})
	if err != nil {
		h.logger.Error("failed to create user", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	// Create token balance with initial free tokens
	_, tokenErr := qtx.CreateTokenBalance(c.Request.Context(), store.CreateTokenBalanceParams{
		CompanyID: company.ID,
		Balance:   1000, // Free tier tokens
	})
	if tokenErr != nil {
		h.logger.Warn("failed to create initial token balance", "company_id", company.ID, "error", tokenErr)
	}

	// Seed country-specific leave types and holidays
	if seedErr := seedCountryDefaults(c.Request.Context(), qtx, company.ID, country); seedErr != nil {
		h.logger.Warn("failed to seed country defaults", "company_id", company.ID, "country", country, "error", seedErr)
	}

	// Track referral if a referral code was provided
	if req.ReferralCode != "" {
		referrer, refErr := qtx.GetCompanyByReferralCode(c.Request.Context(), &req.ReferralCode)
		if refErr == nil && referrer.ID != company.ID {
			_ = qtx.SetReferredByCode(c.Request.Context(), store.SetReferredByCodeParams{
				ID:             company.ID,
				ReferredByCode: &req.ReferralCode,
			})
			_, _ = qtx.CreateReferralEvent(c.Request.Context(), store.CreateReferralEventParams{
				ReferrerCompanyID: referrer.ID,
				ReferredCompanyID: company.ID,
				ReferralCode:      req.ReferralCode,
			})
			h.logger.Info("referral tracked", "referrer", referrer.ID, "referred", company.ID, "code", req.ReferralCode)
		}
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		h.logger.Error("failed to commit transaction", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	// Generate verification token and send email
	if h.email != nil && h.email.IsEnabled() {
		verToken, err := generateVerificationToken()
		if err != nil {
			h.logger.Error("failed to generate verification token", "error", err)
			response.InternalError(c, "Registration failed")
			return
		}

		expiresAt := pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true}
		if err := h.queries.SetVerificationToken(c.Request.Context(), store.SetVerificationTokenParams{
			ID:                         user.ID,
			VerificationToken:          &verToken,
			VerificationTokenExpiresAt: expiresAt,
		}); err != nil {
			h.logger.Error("failed to save verification token", "error", err)
		}

		go func() {
			if sendErr := h.email.SendVerificationEmail(req.Email, req.FirstName, verToken); sendErr != nil {
				h.logger.Error("failed to send verification email", "email", req.Email, "error", sendErr)
			}
		}()

		response.Created(c, gin.H{
			"message":        "Registration successful. Please check your email to verify your account.",
			"email_sent":     true,
			"email_verified": false,
		})
		return
	}

	// No email service — auto-verify and return tokens (dev mode)
	_ = h.queries.MarkEmailVerified(c.Request.Context(), user.ID)

	token, err := h.jwt.GenerateToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		h.logger.Error("failed to generate token", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	refreshToken, err := h.jwt.GenerateRefreshToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		h.logger.Error("failed to generate refresh token", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	response.Created(c, authResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User: userResponse{
			ID:              user.ID,
			Email:           user.Email,
			FirstName:       user.FirstName,
			LastName:        user.LastName,
			Role:            user.Role,
			CompanyID:       user.CompanyID,
			CompanyCountry:  company.Country,
			CompanyCurrency: company.Currency,
			CompanyTimezone: company.Timezone,
		},
	})
}

type cliRegisterRequest struct {
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required,min=8"`
	CompanyName  string `json:"company_name"`
	Country      string `json:"country"`
	ReferralCode string `json:"referral_code"`
}

type cliRegisterResponse struct {
	Token          string       `json:"token"`
	RefreshToken   string       `json:"refresh_token"`
	APIKey         string       `json:"api_key"`
	APIKeyPrefix   string       `json:"api_key_prefix"`
	User           userResponse `json:"user"`
}

// CLIRegister handles POST /auth/cli-register.
// It allows new users to register and receive an API key in one step from the CLI,
// skipping browser-based email verification.
func (h *Handler) CLIRegister(c *gin.Context) {
	var req cliRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Check if email already exists
	_, err := h.queries.GetUserByEmail(c.Request.Context(), req.Email)
	if err == nil {
		response.Error(c, http.StatusConflict, "email_exists", "Email already registered. Use cli-login instead.")
		return
	}

	// Derive company name from email domain if empty
	if req.CompanyName == "" {
		parts := strings.SplitN(req.Email, "@", 2)
		if len(parts) == 2 {
			req.CompanyName = parts[1]
		} else {
			req.CompanyName = "My Company"
		}
	}

	// Default country
	country := req.Country
	if country == "" {
		country = "PHL"
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	// Begin transaction
	tx, err := h.pool.Begin(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to begin transaction", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}
	defer tx.Rollback(c.Request.Context())

	qtx := h.queries.WithTx(tx)

	// Resolve country defaults
	cc := countryConfig(country)

	// Create company with country-specific settings
	company, err := qtx.CreateCompanyWithCountry(c.Request.Context(), store.CreateCompanyWithCountryParams{
		Name:         req.CompanyName,
		Country:      cc.Country,
		Currency:     cc.Currency,
		Timezone:     cc.Timezone,
		PayFrequency: cc.PayFrequency,
	})
	if err != nil {
		h.logger.Error("failed to create company", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	// Create user with super_admin role
	user, err := qtx.CreateUser(c.Request.Context(), store.CreateUserParams{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		FirstName:    "User",
		LastName:     "",
		Role:         string(RoleSuperAdmin),
		CompanyID:    company.ID,
	})
	if err != nil {
		h.logger.Error("failed to create user", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	// Create token balance with initial free tokens
	_, tokenErr := qtx.CreateTokenBalance(c.Request.Context(), store.CreateTokenBalanceParams{
		CompanyID: company.ID,
		Balance:   1000,
	})
	if tokenErr != nil {
		h.logger.Warn("failed to create initial token balance", "company_id", company.ID, "error", tokenErr)
	}

	// Seed country-specific leave types and holidays
	if seedErr := seedCountryDefaults(c.Request.Context(), qtx, company.ID, country); seedErr != nil {
		h.logger.Warn("failed to seed country defaults", "company_id", company.ID, "country", country, "error", seedErr)
	}

	// Track referral if a referral code was provided
	if req.ReferralCode != "" {
		referrer, refErr := qtx.GetCompanyByReferralCode(c.Request.Context(), &req.ReferralCode)
		if refErr == nil && referrer.ID != company.ID {
			_ = qtx.SetReferredByCode(c.Request.Context(), store.SetReferredByCodeParams{
				ID:             company.ID,
				ReferredByCode: &req.ReferralCode,
			})
			_, _ = qtx.CreateReferralEvent(c.Request.Context(), store.CreateReferralEventParams{
				ReferrerCompanyID: referrer.ID,
				ReferredCompanyID: company.ID,
				ReferralCode:      req.ReferralCode,
			})
			h.logger.Info("referral tracked", "referrer", referrer.ID, "referred", company.ID, "code", req.ReferralCode)
		}
	}

	// Auto-verify email (CLI registration skips email verification flow)
	if err := qtx.MarkEmailVerified(c.Request.Context(), user.ID); err != nil {
		h.logger.Error("failed to mark email verified", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	// Create API key inside transaction
	apiKey := generateAPIKey()
	apiKeyPfx := apiKeyPrefix(apiKey)
	apiKeyHash := hashAPIKey(apiKey)

	keyRow, err := qtx.CreateAPIKey(c.Request.Context(), store.CreateAPIKeyParams{
		UserID:    user.ID,
		CompanyID: company.ID,
		Prefix:    apiKeyPfx,
		KeyHash:   apiKeyHash,
		Name:      "cli-default",
	})
	if err != nil {
		h.logger.Error("failed to create API key", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	// Commit transaction
	if err := tx.Commit(c.Request.Context()); err != nil {
		h.logger.Error("failed to commit transaction", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	// Generate JWT tokens
	jwtToken, err := h.jwt.GenerateToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		h.logger.Error("failed to generate token", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	refreshToken, err := h.jwt.GenerateRefreshToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		h.logger.Error("failed to generate refresh token", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	response.Created(c, cliRegisterResponse{
		Token:        jwtToken,
		RefreshToken: refreshToken,
		APIKey:       apiKey,
		APIKeyPrefix: keyRow.Prefix,
		User: userResponse{
			ID:              user.ID,
			Email:           user.Email,
			FirstName:       user.FirstName,
			LastName:        user.LastName,
			Role:            user.Role,
			CompanyID:       user.CompanyID,
			CompanyCountry:  company.Country,
			CompanyCurrency: company.Currency,
			CompanyTimezone: company.Timezone,
		},
	})
}

type cliLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// CLILogin handles POST /auth/cli-login.
// It allows existing users to login from the CLI and receive a fresh API key.
// It auto-verifies unverified emails and rotates the cli-default API key on each login.
// Uses GetUserByEmailAny (finds users regardless of email-verified status),
// then checks status explicitly. Returns generic 401 for both "not found" and
// "wrong password" to prevent email enumeration.
func (h *Handler) CLILogin(c *gin.Context) {
	var req cliLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Lookup user regardless of email-verified status
	user, err := h.queries.GetUserByEmailAny(c.Request.Context(), req.Email)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	// Check account status before password verification to avoid leaking user existence
	if user.Status != "active" {
		response.Error(c, http.StatusForbidden, "account_disabled", "Account has been disabled")
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		response.Error(c, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	// Auto-verify email if not verified (CLI login skips the email verification flow)
	if !user.EmailVerified {
		if err := h.queries.MarkEmailVerified(c.Request.Context(), user.ID); err != nil {
			h.logger.Error("failed to mark email verified", "user_id", user.ID, "error", err)
			response.InternalError(c, "Login failed")
			return
		}
	}

	// Update last login (synchronous)
	_ = h.queries.UpdateLastLogin(c.Request.Context(), user.ID)

	// Revoke existing cli-default key (best-effort, ignore error)
	_ = h.queries.RevokeAPIKeyByName(c.Request.Context(), store.RevokeAPIKeyByNameParams{
		UserID: user.ID,
		Name:   "cli-default",
	})

	// Create fresh API key
	apiKey := generateAPIKey()
	apiKeyPfx := apiKeyPrefix(apiKey)
	apiKeyHash := hashAPIKey(apiKey)

	keyRow, err := h.queries.CreateAPIKey(c.Request.Context(), store.CreateAPIKeyParams{
		UserID:    user.ID,
		CompanyID: user.CompanyID,
		Prefix:    apiKeyPfx,
		KeyHash:   apiKeyHash,
		Name:      "cli-default",
	})
	if err != nil {
		h.logger.Error("failed to create API key", "user_id", user.ID, "error", err)
		response.InternalError(c, "Login failed")
		return
	}

	// Generate JWT tokens
	jwtToken, err := h.jwt.GenerateToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		h.logger.Error("failed to generate token", "error", err)
		response.InternalError(c, "Login failed")
		return
	}

	refreshToken, err := h.jwt.GenerateRefreshToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		h.logger.Error("failed to generate refresh token", "error", err)
		response.InternalError(c, "Login failed")
		return
	}

	// Enrich response with company info
	loginResp := userResponse{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Role:      user.Role,
		CompanyID: user.CompanyID,
	}
	comp, compErr := h.queries.GetCompanyByID(c.Request.Context(), user.CompanyID)
	if compErr == nil {
		loginResp.CompanyCountry = comp.Country
		loginResp.CompanyCurrency = comp.Currency
		loginResp.CompanyTimezone = comp.Timezone
	}

	response.OK(c, cliRegisterResponse{
		Token:        jwtToken,
		RefreshToken: refreshToken,
		APIKey:       apiKey,
		APIKeyPrefix: keyRow.Prefix,
		User:         loginResp,
	})
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	user, err := h.queries.GetUserByEmailAny(c.Request.Context(), req.Email)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	if user.Status != "active" {
		response.Error(c, http.StatusForbidden, "account_disabled", "Account has been disabled")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		response.Error(c, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	if !user.EmailVerified {
		response.Error(c, http.StatusForbidden, "email_not_verified", "Please verify your email address before logging in")
		return
	}

	token, err := h.jwt.GenerateToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		h.logger.Error("failed to generate token", "error", err)
		response.InternalError(c, "Login failed")
		return
	}

	refreshToken, err := h.jwt.GenerateRefreshToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		h.logger.Error("failed to generate refresh token", "error", err)
		response.InternalError(c, "Login failed")
		return
	}

	// Update last login
	_ = h.queries.UpdateLastLogin(c.Request.Context(), user.ID)

	loginResp := userResponse{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Role:      user.Role,
		CompanyID: user.CompanyID,
	}

	// Enrich with company info
	comp, compErr := h.queries.GetCompanyByID(c.Request.Context(), user.CompanyID)
	if compErr == nil {
		loginResp.CompanyCountry = comp.Country
		loginResp.CompanyCurrency = comp.Currency
		loginResp.CompanyTimezone = comp.Timezone
	}

	response.OK(c, authResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         loginResp,
	})
}

func (h *Handler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Check refresh token blacklist
	if h.redis != nil {
		hash := sha256.Sum256([]byte(req.RefreshToken))
		key := "blacklist:refresh:" + hex.EncodeToString(hash[:])
		if h.redis.Exists(c.Request.Context(), key).Val() > 0 {
			response.Unauthorized(c, "token has been revoked")
			return
		}
	}

	claims, err := h.jwt.ValidateToken(req.RefreshToken)
	if err != nil {
		response.Unauthorized(c, "Invalid refresh token")
		return
	}

	// Get fresh user data
	user, err := h.queries.GetUserByID(c.Request.Context(), claims.UserID)
	if err != nil {
		response.Unauthorized(c, "User not found")
		return
	}

	token, err := h.jwt.GenerateToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		response.InternalError(c, "Token refresh failed")
		return
	}

	refreshToken, err := h.jwt.GenerateRefreshToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		response.InternalError(c, "Token refresh failed")
		return
	}

	response.OK(c, authResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User: userResponse{
			ID:        user.ID,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Role:      user.Role,
			CompanyID: user.CompanyID,
		},
	})
}

func (h *Handler) Logout(c *gin.Context) {
	var req logoutRequest
	_ = c.ShouldBindJSON(&req) // optional body

	if req.RefreshToken != "" && h.redis != nil {
		parser := jwt.NewParser()
		claims := &Claims{}
		_, _, err := parser.ParseUnverified(req.RefreshToken, claims)
		if err == nil && claims.ExpiresAt != nil {
			ttl := time.Until(claims.ExpiresAt.Time)
			if ttl > 0 {
				hash := sha256.Sum256([]byte(req.RefreshToken))
				key := "blacklist:refresh:" + hex.EncodeToString(hash[:])
				h.redis.Set(c.Request.Context(), key, "1", ttl)
			}
		}
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *Handler) Me(c *gin.Context) {
	userID := GetUserID(c)
	user, err := h.queries.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "User not found")
		return
	}

	resp := userResponse{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Role:      user.Role,
		CompanyID: user.CompanyID,
		AvatarUrl: user.AvatarUrl,
	}

	// Enrich with company info
	company, err := h.queries.GetCompanyByID(c.Request.Context(), user.CompanyID)
	if err == nil {
		resp.CompanyCountry = company.Country
		resp.CompanyCurrency = company.Currency
		resp.CompanyTimezone = company.Timezone
	}

	response.OK(c, resp)
}

type forgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type resetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

func (h *Handler) ForgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Valid email is required")
		return
	}

	successMsg := gin.H{"message": "If an account exists, a reset link has been sent"}

	user, err := h.queries.GetUserByEmail(c.Request.Context(), req.Email)
	if err != nil {
		response.OK(c, successMsg)
		return
	}

	token, err := generateVerificationToken()
	if err != nil {
		h.logger.Error("failed to generate reset token", "error", err)
		response.OK(c, successMsg)
		return
	}

	expiresAt := pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true}
	err = h.queries.SetResetToken(c.Request.Context(), store.SetResetTokenParams{
		ID:                  user.ID,
		ResetToken:          &token,
		ResetTokenExpiresAt: expiresAt,
	})
	if err != nil {
		h.logger.Error("failed to set reset token", "error", err)
		response.OK(c, successMsg)
		return
	}

	if h.email != nil && h.email.IsEnabled() {
		go func() {
			if err := h.email.SendPasswordResetEmail(user.Email, user.FirstName, token); err != nil {
				h.logger.Error("failed to send password reset email", "email", user.Email, "error", err)
			}
		}()
	}

	response.OK(c, successMsg)
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Token and password (min 8 characters) are required")
		return
	}

	user, err := h.queries.GetUserByResetToken(c.Request.Context(), &req.Token)
	if err != nil {
		// Differentiate expired vs invalid
		_, err2 := h.queries.GetUserByResetTokenAny(c.Request.Context(), &req.Token)
		if err2 != nil {
			response.Error(c, http.StatusBadRequest, "token_invalid", "This reset link is invalid")
			return
		}
		response.Error(c, http.StatusBadRequest, "token_expired", "This reset link has expired")
		return
	}

	hashedPassword, err := HashPassword(req.Password)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		response.InternalError(c, "Password reset failed")
		return
	}

	err = h.queries.ResetUserPassword(c.Request.Context(), store.ResetUserPasswordParams{
		ID:           user.ID,
		PasswordHash: hashedPassword,
	})
	if err != nil {
		h.logger.Error("failed to reset password", "error", err)
		response.InternalError(c, "Password reset failed")
		return
	}

	err = h.queries.ClearResetToken(c.Request.Context(), user.ID)
	if err != nil {
		h.logger.Error("failed to clear reset token", "error", err)
	}

	// Auto-login
	token, err := h.jwt.GenerateToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		h.logger.Error("failed to generate token after reset", "error", err)
		response.InternalError(c, "Password reset succeeded but login failed")
		return
	}

	refreshToken, err := h.jwt.GenerateRefreshToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		h.logger.Error("failed to generate refresh token after reset", "error", err)
		response.InternalError(c, "Password reset succeeded but login failed")
		return
	}

	_ = h.queries.UpdateLastLogin(c.Request.Context(), user.ID)

	loginResp := userResponse{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Role:      user.Role,
		CompanyID: user.CompanyID,
	}

	comp, compErr := h.queries.GetCompanyByID(c.Request.Context(), user.CompanyID)
	if compErr == nil {
		loginResp.CompanyCountry = comp.Country
		loginResp.CompanyCurrency = comp.Currency
		loginResp.CompanyTimezone = comp.Timezone
	}

	response.OK(c, authResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         loginResp,
	})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	userID := GetUserID(c)
	user, err := h.queries.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "User not found")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		response.BadRequest(c, "Current password is incorrect")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		response.InternalError(c, "Failed to update password")
		return
	}

	if err := h.queries.UpdateUserPassword(c.Request.Context(), store.UpdateUserPasswordParams{
		PasswordHash: string(hashedPassword),
		ID:           userID,
		CompanyID:    GetCompanyID(c),
	}); err != nil {
		response.InternalError(c, "Failed to update password")
		return
	}

	response.OK(c, gin.H{"message": "Password updated"})
}

type updateProfileRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Locale    string `json:"locale"`
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	userID := GetUserID(c)
	user, err := h.queries.UpdateUserProfile(c.Request.Context(), store.UpdateUserProfileParams{
		ID:        userID,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Locale:    req.Locale,
		CompanyID: GetCompanyID(c),
	})
	if err != nil {
		h.logger.Error("failed to update profile", "error", err)
		response.InternalError(c, "Failed to update profile")
		return
	}

	response.OK(c, userResponse{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Role:      user.Role,
		CompanyID: user.CompanyID,
		AvatarUrl: user.AvatarUrl,
	})
}

// UploadAvatar handles profile photo upload.
func (h *Handler) UploadAvatar(c *gin.Context) {
	const maxAvatarSize = 5 << 20 // 5MB

	file, header, err := c.Request.FormFile("avatar")
	if err != nil {
		response.BadRequest(c, "Avatar file is required")
		return
	}
	defer file.Close()

	// Validate file size
	if header.Size > maxAvatarSize {
		response.BadRequest(c, "Avatar file size exceeds 5MB limit")
		return
	}

	// Detect actual MIME from content (not user-supplied header)
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	detectedMIME := http.DetectContentType(buf[:n])
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		response.InternalError(c, "Failed to process file")
		return
	}
	allowedMIME := map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true}
	if !allowedMIME[detectedMIME] {
		response.BadRequest(c, "Only PNG, JPG, and WebP images are allowed")
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowedExt := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true}
	if !allowedExt[ext] {
		response.BadRequest(c, "Only PNG, JPG, and WebP files are allowed")
		return
	}

	userID := GetUserID(c)
	uploadDir := fmt.Sprintf("uploads/avatars/%d", userID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		h.logger.Error("failed to create avatar dir", "error", err)
		response.InternalError(c, "Failed to upload avatar")
		return
	}

	fileName := fmt.Sprintf("avatar_%d%s", time.Now().UnixMilli(), ext)
	filePath := filepath.Join(uploadDir, fileName)

	out, err := os.Create(filePath)
	if err != nil {
		h.logger.Error("failed to create avatar file", "error", err)
		response.InternalError(c, "Failed to upload avatar")
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		h.logger.Error("failed to write avatar file", "error", err)
		response.InternalError(c, "Failed to upload avatar")
		return
	}

	avatarURL := "/" + filePath
	user, err := h.queries.UpdateUserProfile(c.Request.Context(), store.UpdateUserProfileParams{
		ID:        userID,
		AvatarUrl: &avatarURL,
		CompanyID: GetCompanyID(c),
	})
	if err != nil {
		h.logger.Error("failed to update avatar url", "error", err)
		response.InternalError(c, "Failed to update avatar")
		return
	}

	response.OK(c, userResponse{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Role:      user.Role,
		CompanyID: user.CompanyID,
		AvatarUrl: user.AvatarUrl,
	})
}

// VerifyEmail handles GET /auth/verify-email?token=xxx
func (h *Handler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		response.BadRequest(c, "Verification token is required")
		return
	}

	user, err := h.queries.GetUserByVerificationToken(c.Request.Context(), &token)
	if err != nil {
		expiredUser, err2 := h.queries.GetUserByVerificationTokenAny(c.Request.Context(), &token)
		if err2 != nil {
			response.Error(c, http.StatusBadRequest, "token_invalid", "This verification link is invalid")
			return
		}
		if expiredUser.EmailVerified {
			response.OK(c, gin.H{"status": "already_verified", "message": "Email already verified"})
			return
		}
		response.Error(c, http.StatusBadRequest, "token_expired", "This verification link has expired")
		return
	}

	if user.EmailVerified {
		response.OK(c, gin.H{"status": "already_verified", "message": "Email already verified"})
		return
	}

	if err := h.queries.MarkEmailVerified(c.Request.Context(), user.ID); err != nil {
		h.logger.Error("failed to mark email verified", "error", err)
		response.InternalError(c, "Verification failed")
		return
	}

	// Send welcome email in background
	if h.email != nil && h.email.IsEnabled() {
		go func() {
			_ = h.email.SendWelcomeEmail(user.Email, user.FirstName)
		}()
	}

	// Auto-login: generate JWT tokens so the magic link logs the user in
	token, tokenErr := h.jwt.GenerateToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	refreshToken := ""
	if tokenErr == nil {
		refreshToken, _ = h.jwt.GenerateRefreshToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	}

	response.OK(c, gin.H{
		"message":        "Email verified successfully.",
		"email_verified": true,
		"token":          token,
		"refresh_token":  refreshToken,
		"user": userResponse{
			ID:        user.ID,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Role:      user.Role,
			CompanyID: user.CompanyID,
		},
	})
}

type resendVerificationRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResendVerification handles POST /auth/resend-verification
func (h *Handler) ResendVerification(c *gin.Context) {
	var req resendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	user, err := h.queries.GetUserByEmail(c.Request.Context(), req.Email)
	if err != nil {
		// Don't reveal whether email exists
		response.OK(c, gin.H{"message": "If the email exists, a verification link has been sent."})
		return
	}

	if user.EmailVerified {
		response.OK(c, gin.H{"message": "Email is already verified. You can log in."})
		return
	}

	if h.email == nil || !h.email.IsEnabled() {
		response.BadRequest(c, "Email service is not configured")
		return
	}

	verToken, err := generateVerificationToken()
	if err != nil {
		response.InternalError(c, "Failed to generate token")
		return
	}

	expiresAt := pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true}
	if err := h.queries.SetVerificationToken(c.Request.Context(), store.SetVerificationTokenParams{
		ID:                         user.ID,
		VerificationToken:          &verToken,
		VerificationTokenExpiresAt: expiresAt,
	}); err != nil {
		h.logger.Error("failed to save verification token", "error", err)
		response.InternalError(c, "Failed to send verification email")
		return
	}

	go func() {
		_ = h.email.SendVerificationEmail(user.Email, user.FirstName, verToken)
	}()

	response.OK(c, gin.H{"message": "If the email exists, a verification link has been sent."})
}

type switchCompanyRequest struct {
	CompanyID int64 `json:"company_id" binding:"required"`
}

// SwitchCompany switches the authenticated user's active company.
// It verifies membership via user_companies, updates the active company,
// and returns fresh JWT tokens scoped to the new company.
func (h *Handler) SwitchCompany(c *gin.Context) {
	var req switchCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "company_id is required")
		return
	}

	userID := GetUserID(c)
	userEmail := GetEmail(c)
	ctx := c.Request.Context()

	// Verify user is a member of the target company
	membership, err := h.queries.GetUserCompanyMembership(ctx, store.GetUserCompanyMembershipParams{
		UserID:    userID,
		CompanyID: req.CompanyID,
	})
	if err != nil {
		response.Forbidden(c, "access denied")
		return
	}

	// Update the user's active company
	if err := h.queries.UpdateUserActiveCompany(ctx, store.UpdateUserActiveCompanyParams{
		ID:        userID,
		CompanyID: req.CompanyID,
	}); err != nil {
		h.logger.Error("failed to switch company", "user_id", userID, "company_id", req.CompanyID, "error", err)
		response.InternalError(c, "failed to switch company")
		return
	}

	// Issue new tokens with the updated company and role
	role := Role(membership.Role)
	token, err := h.jwt.GenerateToken(userID, userEmail, role, req.CompanyID)
	if err != nil {
		h.logger.Error("failed to generate token", "error", err)
		response.InternalError(c, "failed to generate token")
		return
	}
	refreshToken, err := h.jwt.GenerateRefreshToken(userID, userEmail, role, req.CompanyID)
	if err != nil {
		h.logger.Error("failed to generate refresh token", "error", err)
		response.InternalError(c, "failed to generate refresh token")
		return
	}

	// Enrich response with company info
	company, _ := h.queries.GetCompanyByID(ctx, req.CompanyID)

	resp := authResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User: userResponse{
			ID:        userID,
			Email:     userEmail,
			Role:      string(role),
			CompanyID: req.CompanyID,
		},
	}
	if company.ID != 0 {
		resp.User.CompanyCountry = company.Country
		resp.User.CompanyCurrency = company.Currency
		resp.User.CompanyTimezone = company.Timezone
	}

	response.OK(c, resp)
}

type ssoLoginRequest struct {
	SSOToken string `json:"sso_token" binding:"required"`
}

// SSOLogin handles POST /auth/sso — validates a Finance→HR SSO token and returns auth tokens.
func (h *Handler) SSOLogin(c *gin.Context) {
	var req ssoLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "sso_token is required")
		return
	}

	if h.sso == nil {
		response.InternalError(c, "SSO not configured")
		return
	}

	ssoEmail, err := h.sso.ValidateFinanceEmail(req.SSOToken)
	if err != nil {
		h.logger.Warn("SSO token validation failed", "error", err)
		response.Unauthorized(c, "invalid or expired SSO token")
		return
	}

	user, err := h.queries.GetUserByEmail(c.Request.Context(), ssoEmail)
	if err != nil {
		response.Forbidden(c, "no HR account for this email")
		return
	}

	if user.Status != "active" {
		response.Forbidden(c, "account is not active")
		return
	}

	if !user.EmailVerified {
		response.Forbidden(c, "email not verified")
		return
	}

	token, err := h.jwt.GenerateToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		response.InternalError(c, "failed to generate token")
		return
	}

	refreshToken, err := h.jwt.GenerateRefreshToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		response.InternalError(c, "failed to generate refresh token")
		return
	}

	_ = h.queries.UpdateLastLogin(c.Request.Context(), user.ID)

	company, _ := h.queries.GetCompanyByID(c.Request.Context(), user.CompanyID)

	resp := authResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User: userResponse{
			ID:        user.ID,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Role:      user.Role,
			CompanyID: user.CompanyID,
			AvatarUrl: user.AvatarUrl,
		},
	}
	if company.ID != 0 {
		resp.User.CompanyCountry = company.Country
		resp.User.CompanyCurrency = company.Currency
		resp.User.CompanyTimezone = company.Timezone
	}

	response.OK(c, resp)
}
