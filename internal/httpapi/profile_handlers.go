package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type updateProfileRequest struct {
	DisplayName     string `json:"display_name"`
	Email           string `json:"email"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (h *Handler) updateProfile(c *gin.Context) {
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	displayName := strings.TrimSpace(req.DisplayName)
	email := normalizeEmail(req.Email)
	newPassword := strings.TrimSpace(req.NewPassword)
	confirmPassword := strings.TrimSpace(req.ConfirmPassword)

	if displayName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "显示名不能为空"})
		return
	}
	if len(displayName) > 120 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "显示名最长 120 字符"})
		return
	}

	userID, _ := c.Get("user_id")
	id, _ := userID.(uint)
	user, err := h.store.GetUserByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	user = h.refreshLDAPUserFromAD(user)

	updates := map[string]any{"display_name": displayName}
	passwordChanged := false

	if user.IsLDAP() {
		email = normalizeEmail(user.Email)
		if email == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "LDAP 账号邮箱为空，请重新登录同步 AD 邮箱"})
			return
		}
		if normalizeEmail(req.Email) != email {
			c.JSON(http.StatusBadRequest, gin.H{"error": "LDAP 用户不能修改邮箱"})
			return
		}
		if newPassword != "" || confirmPassword != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": ldapPasswordHint})
			return
		}
	} else {
		if msg := validateEmail(email); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
		existing, err := h.store.GetUserByEmail(email)
		if err == nil && existing.ID != id {
			c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱已被其他用户使用"})
			return
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		updates["email"] = email

		if newPassword != "" || confirmPassword != "" {
			if msg := validatePassword(newPassword); msg != "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": msg})
				return
			}
			if newPassword != confirmPassword {
				c.JSON(http.StatusBadRequest, gin.H{"error": "两次输入的新密码不一致"})
				return
			}
			if h.auth.ComparePassword(user.PasswordHash, newPassword) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "新密码不能与旧密码相同"})
				return
			}
			hash, err := h.auth.HashPassword(newPassword)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "hash password failed"})
				return
			}
			if err := h.store.UpdateUserPassword(id, hash); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			passwordChanged = true
		}
	}

	if err := h.store.UpdateUserFields(id, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	detail := fmt.Sprintf("updated profile for %s", user.Name)
	if passwordChanged {
		detail += " (password changed)"
	}
	h.writeAudit(c, "update_profile", "success", detail)
	c.JSON(http.StatusOK, gin.H{
		"message":          "updated",
		"display_name":     displayName,
		"email":            email,
		"password_changed": passwordChanged,
	})
}
