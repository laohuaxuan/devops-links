package httpapi

import (
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.Use(cors())
	r.GET("/api/health", h.health)
	r.GET("/api/captcha", h.captchaImage)
	r.GET("/api/check-exists", h.checkExists)
	r.POST("/api/login", h.login)
	r.POST("/api/login/ldap", h.loginLDAP)
	r.GET("/api/auth/providers", h.authProviders)

	r.Static(h.cfg.Upload.URLPrefix, h.cfg.Upload.AbsDir())

	api := r.Group("/api")
	api.Use(h.requireAuth())
	{
		api.GET("/me", h.me)
		api.POST("/logout", h.logout)
		api.PUT("/profile", h.updateProfile)
		api.GET("/categories", h.listCategories)

		api.POST("/categories", h.createCategory)
		api.PUT("/categories/:id", h.updateCategory)
		api.DELETE("/categories/:id", h.deleteCategory)
		api.POST("/links", h.createLink)
		api.PUT("/links/:id", h.updateLink)
		api.DELETE("/links/:id", h.deleteLink)
		api.POST("/upload/icon", h.uploadIcon)

		superAdmin := api.Group("")
		superAdmin.Use(h.requireSuperAdmin())
		{
			superAdmin.GET("/users", h.listUsers)
			superAdmin.POST("/users", h.createUser)
			superAdmin.PUT("/users/:id", h.updateUser)
			superAdmin.DELETE("/users/:id", h.deleteUser)
			superAdmin.POST("/users/:id/reset-password", h.resetUserPassword)
			superAdmin.GET("/audit-logs", h.auditLogs)
		}
	}
}
