package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	ContextKeyUserID    = "user_id"
	ContextKeyEmail     = "email"
	ContextKeyRole      = "role"
	ContextKeyCompanyID = "company_id"
)

func JWTMiddleware(jwtSvc *JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractBearerToken(c)
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": "unauthorized", "message": "Missing authorization token"},
			})
			return
		}

		claims, err := jwtSvc.ValidateToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": "invalid_token", "message": "Invalid or expired token"},
			})
			return
		}

		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyEmail, claims.Email)
		c.Set(ContextKeyRole, claims.Role)
		c.Set(ContextKeyCompanyID, claims.CompanyID)
		c.Next()
	}
}

func RoleMiddleware(roles ...Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := GetRole(c)
		for _, r := range roles {
			if userRole == r {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": gin.H{"code": "forbidden", "message": "Insufficient permissions"},
		})
	}
}

func AdminOnly() gin.HandlerFunc {
	return RoleMiddleware(RoleSuperAdmin, RoleAdmin)
}

func ManagerOrAbove() gin.HandlerFunc {
	return RoleMiddleware(RoleSuperAdmin, RoleAdmin, RoleManager)
}

func extractBearerToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func GetUserID(c *gin.Context) int64 {
	v, _ := c.Get(ContextKeyUserID)
	if id, ok := v.(int64); ok {
		return id
	}
	return 0
}

func GetEmail(c *gin.Context) string {
	v, _ := c.Get(ContextKeyEmail)
	if email, ok := v.(string); ok {
		return email
	}
	return ""
}

func GetRole(c *gin.Context) Role {
	v, _ := c.Get(ContextKeyRole)
	if role, ok := v.(Role); ok {
		return role
	}
	return RoleEmployee
}

func GetCompanyID(c *gin.Context) int64 {
	v, _ := c.Get(ContextKeyCompanyID)
	if id, ok := v.(int64); ok {
		return id
	}
	return 0
}
