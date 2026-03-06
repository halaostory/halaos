package auth

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

func (h *Handler) ListUsers(c *gin.Context) {
	companyID := GetCompanyID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize
	users, err := h.queries.ListUsersByCompany(c.Request.Context(), store.ListUsersByCompanyParams{
		CompanyID: companyID,
		Limit:     int32(pageSize),
		Offset:    int32(offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list users")
		return
	}
	total, _ := h.queries.CountUsersByCompany(c.Request.Context(), companyID)
	type safeUser struct {
		ID          int64       `json:"id"`
		Email       string      `json:"email"`
		FirstName   string      `json:"first_name"`
		LastName    string      `json:"last_name"`
		Role        string      `json:"role"`
		Status      string      `json:"status"`
		AvatarUrl   *string     `json:"avatar_url"`
		Locale      string      `json:"locale"`
		LastLoginAt interface{} `json:"last_login_at"`
		CreatedAt   interface{} `json:"created_at"`
	}
	safeUsers := make([]safeUser, len(users))
	for i, u := range users {
		safeUsers[i] = safeUser{
			ID: u.ID, Email: u.Email, FirstName: u.FirstName, LastName: u.LastName,
			Role: u.Role, Status: u.Status, AvatarUrl: u.AvatarUrl, Locale: u.Locale,
			LastLoginAt: u.LastLoginAt, CreatedAt: u.CreatedAt,
		}
	}
	response.OK(c, gin.H{"users": safeUsers, "total": total})
}

func (h *Handler) UpdateUserRole(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Role is required")
		return
	}
	allowed := map[string]bool{"admin": true, "manager": true, "employee": true}
	if !allowed[req.Role] {
		response.BadRequest(c, "Invalid role")
		return
	}
	if err := h.queries.UpdateUserRole(c.Request.Context(), store.UpdateUserRoleParams{
		ID: id, Role: req.Role,
	}); err != nil {
		response.InternalError(c, "Failed to update role")
		return
	}
	response.OK(c, gin.H{"message": "Role updated"})
}

func (h *Handler) UpdateUserStatus(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Status is required")
		return
	}
	allowed := map[string]bool{"active": true, "inactive": true, "suspended": true}
	if !allowed[req.Status] {
		response.BadRequest(c, "Invalid status")
		return
	}
	if err := h.queries.UpdateUserStatus(c.Request.Context(), store.UpdateUserStatusParams{
		ID: id, Status: req.Status,
	}); err != nil {
		response.InternalError(c, "Failed to update status")
		return
	}
	response.OK(c, gin.H{"message": "Status updated"})
}

func (h *Handler) AdminResetPassword(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	companyID := GetCompanyID(c)
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Password is required")
		return
	}
	if len(req.Password) < 8 {
		response.BadRequest(c, "Password must be at least 8 characters")
		return
	}
	hash, err := HashPassword(req.Password)
	if err != nil {
		response.InternalError(c, "Failed to hash password")
		return
	}
	if err := h.queries.AdminResetPassword(c.Request.Context(), store.AdminResetPasswordParams{
		ID: id, PasswordHash: hash, CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to reset password")
		return
	}
	response.OK(c, gin.H{"message": "Password reset"})
}
