package auth

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all auth routes on the given router groups.
// loginLimiter is applied to login and register endpoints to prevent brute-force attacks.
func (h *Handler) RegisterRoutes(public, protected *gin.RouterGroup, loginLimiter gin.HandlerFunc) {
	// Public auth routes
	auth := public.Group("/auth")
	{
		auth.POST("/register", loginLimiter, h.Register)
		auth.POST("/login", loginLimiter, h.Login)
		auth.POST("/refresh", h.Refresh)
		auth.GET("/verify-email", h.VerifyEmail)
		auth.POST("/resend-verification", loginLimiter, h.ResendVerification)
		auth.POST("/sso", loginLimiter, h.SSOLogin)
	}

	// Protected auth routes
	protected.GET("/auth/me", h.Me)
	protected.PUT("/auth/password", h.ChangePassword)
	protected.PUT("/auth/profile", h.UpdateProfile)
	protected.POST("/auth/avatar", h.UploadAvatar)
	protected.POST("/auth/switch-company", h.SwitchCompany)
	protected.POST("/auth/logout", h.Logout)

	// User Management (admin)
	protected.GET("/users", AdminOnly(), h.ListUsers)
	protected.POST("/users/employee-account", AdminOnly(), h.CreateEmployeeUser)
	protected.PUT("/users/:id/role", AdminOnly(), h.UpdateUserRole)
	protected.PUT("/users/:id/status", AdminOnly(), h.UpdateUserStatus)
	protected.POST("/users/:id/reset-password", AdminOnly(), h.AdminResetPassword)
}
