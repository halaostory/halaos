package auth

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all auth routes on the given router groups.
func (h *Handler) RegisterRoutes(public, protected *gin.RouterGroup) {
	// Public auth routes
	auth := public.Group("/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.Refresh)
	}

	// Protected auth routes
	protected.GET("/auth/me", h.Me)
	protected.PUT("/auth/password", h.ChangePassword)
	protected.PUT("/auth/profile", h.UpdateProfile)
	protected.POST("/auth/avatar", h.UploadAvatar)

	// User Management (admin)
	protected.GET("/users", AdminOnly(), h.ListUsers)
	protected.PUT("/users/:id/role", AdminOnly(), h.UpdateUserRole)
	protected.PUT("/users/:id/status", AdminOnly(), h.UpdateUserStatus)
	protected.POST("/users/:id/reset-password", AdminOnly(), h.AdminResetPassword)
}
