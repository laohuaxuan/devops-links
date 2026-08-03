package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"devops-links/internal/store"

	"github.com/gin-gonic/gin"
)

type createUserRequest struct {
	Name            string `json:"name"`
	DisplayName     string `json:"display_name"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
	Role            string `json:"role"`
}

type updateUserRequest struct {
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
}

func (h *Handler) listUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	keyword := strings.TrimSpace(c.Query("keyword"))
	users, total, err := h.store.ListUsers(page, size, keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	result := make([]gin.H, 0, len(users))
	for _, u := range users {
		result = append(result, userToJSON(&u))
	}
	c.JSON(http.StatusOK, gin.H{"users": result, "total": total, "page": page, "size": size})
}

func userToJSON(u *store.User) gin.H {
	displayName := strings.TrimSpace(u.DisplayName)
	if displayName == "" {
		displayName = u.Name
	}
	authSource := strings.TrimSpace(u.AuthSource)
	if authSource == "" {
		authSource = store.AuthSourceLocal
	}
	return gin.H{
		"id": u.ID, "name": u.Name, "display_name": displayName, "email": strings.TrimSpace(u.Email),
		"role": u.Role, "status": u.Status, "auth_source": authSource,
		"created_at": u.CreatedAt,
	}
}

func (h *Handler) createUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Email = normalizeEmail(req.Email)
	req.Password = strings.TrimSpace(req.Password)
	req.ConfirmPassword = strings.TrimSpace(req.ConfirmPassword)
	req.Role = strings.TrimSpace(req.Role)

	if msg := validateUsername(req.Name); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if req.DisplayName == "" {
		req.DisplayName = req.Name
	}
	if len(req.DisplayName) > 120 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "显示名最长 120 字符"})
		return
	}
	if msg := validatePassword(req.Password); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if req.Password != req.ConfirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "两次输入的密码不一致"})
		return
	}
	if !store.IsValidRole(req.Role) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "角色无效"})
		return
	}
	if _, err := h.store.GetUserByName(req.Name); err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名已存在"})
		return
	}
	if req.Email == "" {
		req.Email = localPlaceholderEmail(req.Name)
	} else if msg := validateEmail(req.Email); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if _, err := h.store.GetUserByEmail(req.Email); err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱已被使用"})
		return
	}
	hash, err := h.auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "hash password failed"})
		return
	}
	user := &store.User{
		Name: req.Name, DisplayName: req.DisplayName, Email: req.Email,
		PasswordHash: hash, AuthSource: store.AuthSourceLocal, Role: req.Role, Status: store.UserStatusActive,
	}
	if err := h.store.CreateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeAudit(c, "create_user", "success", fmt.Sprintf("created user %s role=%s", user.Name, user.Role))
	c.JSON(http.StatusOK, gin.H{"message": "created", "id": user.ID})
}

func (h *Handler) updateUser(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	displayName := strings.TrimSpace(req.DisplayName)
	role := strings.TrimSpace(req.Role)
	status := strings.TrimSpace(req.Status)

	if displayName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "显示名不能为空"})
		return
	}
	if len(displayName) > 120 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "显示名最长 120 字符"})
		return
	}
	if !store.IsValidRole(role) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "角色无效"})
		return
	}
	if status != store.UserStatusActive && status != store.UserStatusDisabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "状态无效"})
		return
	}

	target, err := h.store.GetUserByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	currentID, _ := c.Get("user_id")
	if uid, ok := currentID.(uint); ok && uid == id {
		if target.Role == store.RoleSuperAdmin && role != store.RoleSuperAdmin {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不能取消自己的超级管理员权限"})
			return
		}
	}
	if err := h.ensureRoleChangeAllowed(target.Role, role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.store.UpdateUserInfo(id, displayName, role, status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeAudit(c, "update_user", "success", fmt.Sprintf("updated user %s", target.Name))
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *Handler) deleteUser(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	currentID, _ := c.Get("user_id")
	if uid, ok := currentID.(uint); ok && uid == id {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除当前登录用户"})
		return
	}
	target, err := h.store.GetUserByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if err := h.ensureRoleChangeAllowed(target.Role, ""); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.store.DeleteUser(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeAudit(c, "delete_user", "success", fmt.Sprintf("deleted user %s", target.Name))
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *Handler) resetUserPassword(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	target, err := h.store.GetUserByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if target.IsLDAP() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "LDAP 用户请在域内修改密码"})
		return
	}
	password, err := generateRandomPassword(12)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "generate password failed"})
		return
	}
	hash, err := h.auth.HashPassword(password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "hash password failed"})
		return
	}
	if err := h.store.UpdateUserPassword(id, hash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeAudit(c, "reset_user_password", "success", fmt.Sprintf("reset password for user %s", target.Name))
	c.JSON(http.StatusOK, gin.H{"message": "password reset", "password": password})
}

func (h *Handler) ensureRoleChangeAllowed(currentRole, newRole string) error {
	if newRole != "" && currentRole == newRole {
		return nil
	}
	switch currentRole {
	case store.RoleSuperAdmin:
		count, err := h.store.CountUsersByRole(store.RoleSuperAdmin)
		if err != nil {
			return err
		}
		if count <= 1 {
			return fmt.Errorf("至少保留一名超级管理员")
		}
	}
	return nil
}
