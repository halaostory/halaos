package auth

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// HashPassword hashes a plain-text password using bcrypt.
func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hashed), err
}

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	jwt     *JWTService
	logger  *slog.Logger
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, jwt *JWTService, logger *slog.Logger) *Handler {
	return &Handler{
		queries: queries,
		pool:    pool,
		jwt:     jwt,
		logger:  logger,
	}
}

type registerRequest struct {
	CompanyName string `json:"company_name" binding:"required,min=2"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	FirstName   string `json:"first_name" binding:"required"`
	LastName    string `json:"last_name" binding:"required"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type authResponse struct {
	Token        string       `json:"token"`
	RefreshToken string       `json:"refresh_token"`
	User         userResponse `json:"user"`
}

type userResponse struct {
	ID        int64   `json:"id"`
	Email     string  `json:"email"`
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Role      string  `json:"role"`
	CompanyID int64   `json:"company_id"`
	AvatarUrl *string `json:"avatar_url,omitempty"`
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

	// Create company
	company, err := qtx.CreateCompany(c.Request.Context(), req.CompanyName)
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
		// Non-fatal: continue registration even if token balance creation fails
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		h.logger.Error("failed to commit transaction", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	// Generate tokens
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
			ID:        user.ID,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Role:      user.Role,
			CompanyID: user.CompanyID,
		},
	})
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	user, err := h.queries.GetUserByEmail(c.Request.Context(), req.Email)
	if err != nil {
		response.Unauthorized(c, "Invalid email or password")
		return
	}

	if user.Status != "active" {
		response.Forbidden(c, "Account is not active")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		response.Unauthorized(c, "Invalid email or password")
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

func (h *Handler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
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

func (h *Handler) Me(c *gin.Context) {
	userID := GetUserID(c)
	user, err := h.queries.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "User not found")
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
