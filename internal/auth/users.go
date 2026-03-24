package auth

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// CreateEmployeeUser creates a login account for an existing employee.
func (h *Handler) CreateEmployeeUser(c *gin.Context) {
	companyID := GetCompanyID(c)
	var req struct {
		EmployeeID int64  `json:"employee_id" binding:"required"`
		Email      string `json:"email" binding:"required,email"`
		Password   string `json:"password" binding:"required"`
		Role       string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if len(req.Password) < 8 {
		response.BadRequest(c, "Password must be at least 8 characters")
		return
	}
	role := req.Role
	if role == "" {
		role = "employee"
	}
	allowed := map[string]bool{"admin": true, "manager": true, "employee": true}
	if !allowed[role] {
		response.BadRequest(c, "Invalid role. Must be admin, manager, or employee")
		return
	}

	// Verify employee exists and belongs to this company
	emp, err := h.queries.GetEmployeeByID(c.Request.Context(), store.GetEmployeeByIDParams{
		ID: req.EmployeeID, CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Employee not found")
		return
	}
	if emp.UserID != nil {
		response.Conflict(c, "Employee already has an account")
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		response.InternalError(c, "Failed to hash password")
		return
	}

	tx, err := h.pool.Begin(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Failed to start transaction")
		return
	}
	defer tx.Rollback(c.Request.Context())

	qtx := h.queries.WithTx(tx)

	user, err := qtx.CreateUser(c.Request.Context(), store.CreateUserParams{
		CompanyID:    companyID,
		Email:        req.Email,
		PasswordHash: hash,
		FirstName:    emp.FirstName,
		LastName:     emp.LastName,
		Role:         role,
	})
	if err != nil {
		response.Conflict(c, "Email already in use")
		return
	}

	// Admin-created employee accounts are pre-verified (no email confirmation needed).
	if err := qtx.MarkEmailVerified(c.Request.Context(), user.ID); err != nil {
		response.InternalError(c, "Failed to verify email")
		return
	}

	if err := qtx.LinkUserToEmployee(c.Request.Context(), store.LinkUserToEmployeeParams{
		UserID:    &user.ID,
		ID:        emp.ID,
		CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to link user to employee")
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		response.InternalError(c, "Failed to commit")
		return
	}

	response.Created(c, gin.H{
		"user_id":     user.ID,
		"employee_id": emp.ID,
		"email":       user.Email,
		"role":        user.Role,
	})
}

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
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	companyID := GetCompanyID(c)
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
		ID: id, Role: req.Role, CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to update role")
		return
	}
	response.OK(c, gin.H{"message": "Role updated"})
}

func (h *Handler) UpdateUserStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	companyID := GetCompanyID(c)
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
		ID: id, Status: req.Status, CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to update status")
		return
	}
	response.OK(c, gin.H{"message": "Status updated"})
}

func (h *Handler) AdminResetPassword(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
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
