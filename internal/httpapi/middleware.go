package httpapi

import (
	"net/http"
	"strings"

	"devops-links/internal/store"

	"github.com/gin-gonic/gin"
)

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (h *Handler) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		token := strings.TrimSpace(header[7:])
		claims, err := h.auth.ParseToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		user, err := h.store.GetUserByID(claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		if !user.IsActive() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "账号已禁用"})
			return
		}
		pwdAt := user.PasswordChangedAt.Unix()
		if pwdAt > 0 && claims.PasswordAt < pwdAt {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "登录已失效，请重新登录"})
			return
		}
		c.Set("user_id", user.ID)
		c.Set("username", user.Name)
		c.Set("display_name", user.DisplayName)
		c.Set("role", user.Role)
		c.Next()
	}
}

func (h *Handler) requirePrivileged() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		r, _ := role.(string)
		if !store.IsPrivilegedRole(r) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
			return
		}
		c.Next()
	}
}

func (h *Handler) requireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		r, _ := role.(string)
		if !store.IsSuperAdminRole(r) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "需要超级管理员权限"})
			return
		}
		c.Next()
	}
}

func (h *Handler) isPrivileged(c *gin.Context) bool {
	role, _ := c.Get("role")
	r, _ := role.(string)
	return store.IsPrivilegedRole(r)
}

func userRoleFlags(role string) (isAdmin, isSuperAdmin bool) {
	return store.IsPrivilegedRole(role), store.IsSuperAdminRole(role)
}
