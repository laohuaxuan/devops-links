package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"devops-links/internal/ldapauth"
	"devops-links/internal/store"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type loginRequest struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	CaptchaToken string `json:"captcha_token"`
	CaptchaCode  string `json:"captcha_code"`
}

type ldapLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.CaptchaToken = strings.TrimSpace(req.CaptchaToken)
	req.CaptchaCode = strings.TrimSpace(req.CaptchaCode)
	if req.CaptchaToken == "" || req.CaptchaCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入验证码"})
		return
	}
	if len(req.CaptchaCode) != 4 || !isNumeric(req.CaptchaCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码必须为4位数字"})
		return
	}
	if !h.verifyCaptcha(req.CaptchaToken, req.CaptchaCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码无效或已过期"})
		return
	}
	if msg := validateUsername(req.Username); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if msg := validatePassword(req.Password); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	token, user, err := h.auth.Login(req.Username, req.Password)
	if err != nil {
		_ = h.store.WriteAudit(&store.AuditLog{
			Username: req.Username, DisplayName: req.Username,
			Action: "login", Result: "failed", IP: c.ClientIP(), Detail: err.Error(),
		})
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	if !user.IsActive() {
		_ = h.store.WriteAudit(&store.AuditLog{
			UserID: user.ID, Username: user.Name, DisplayName: user.DisplayName,
			Action: "login", Result: "failed", IP: c.ClientIP(), Detail: "user disabled",
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "账号已禁用"})
		return
	}
	h.writeAuditForUser(c, user, "login", "success", "login success")
	c.JSON(http.StatusOK, userJSON(token, user))
}

func (h *Handler) authProviders(c *gin.Context) {
	providers := make([]gin.H, 0, 1)
	if h.ldap != nil && h.ldap.Enabled() {
		providers = append(providers, gin.H{"id": "ldap", "label": h.ldap.Label(), "type": "ldap"})
	}
	c.JSON(http.StatusOK, gin.H{"providers": providers})
}

func (h *Handler) loginLDAP(c *gin.Context) {
	if h.ldap == nil || !h.ldap.Enabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "LDAP 登录未启用"})
		return
	}
	var req ldapLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名和密码不能为空"})
		return
	}
	info, err := h.ldap.Authenticate(req.Username, req.Password)
	if err != nil {
		_ = h.store.WriteAudit(&store.AuditLog{
			Username: req.Username, DisplayName: req.Username,
			Action: "login_ldap", Result: "failed", IP: c.ClientIP(), Detail: err.Error(),
		})
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	user, created, err := h.ensureLDAPUser(info)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !user.IsActive() {
		c.JSON(http.StatusForbidden, gin.H{"error": "账号已禁用"})
		return
	}
	token, err := h.auth.GenerateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "generate token failed"})
		return
	}
	detail := "ldap login success"
	if created {
		detail = "ldap login success (auto created)"
	}
	h.writeAuditForUser(c, user, "login_ldap", "success", detail)
	c.JSON(http.StatusOK, userJSON(token, user))
}

func (h *Handler) ensureLDAPUser(info *ldapauth.UserInfo) (*store.User, bool, error) {
	username := strings.TrimSpace(info.Username)
	if username == "" {
		return nil, false, fmt.Errorf("LDAP 用户名无效")
	}
	user, err := h.store.GetUserByName(username)
	if err == nil {
		updated, err := h.applyLDAPProfile(user, info, !user.IsLDAP())
		if err != nil {
			return nil, false, err
		}
		return updated, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	randPwd, err := generateRandomPassword(16)
	if err != nil {
		return nil, false, fmt.Errorf("generate password failed: %w", err)
	}
	hash, err := h.auth.HashPassword(randPwd)
	if err != nil {
		return nil, false, fmt.Errorf("hash password failed: %w", err)
	}
	displayName := strings.TrimSpace(info.DisplayName)
	if displayName == "" {
		displayName = username
	}
	if len(displayName) > 120 {
		displayName = displayName[:120]
	}
	role := strings.TrimSpace(h.cfg.Auth.LDAP.DefaultRole)
	if role != store.RoleAdmin && role != store.RoleGuest {
		role = store.RoleGuest
	}
	newUser := &store.User{
		Name: username, DisplayName: displayName, Email: resolveLDAPUserEmail(username, info.Email),
		PasswordHash: hash, AuthSource: store.AuthSourceLDAP, Role: role, Status: store.UserStatusActive,
	}
	if err := h.store.CreateUser(newUser); err != nil {
		return nil, false, err
	}
	return newUser, true, nil
}

func (h *Handler) applyLDAPProfile(user *store.User, info *ldapauth.UserInfo, setAuthSource bool) (*store.User, error) {
	username := strings.TrimSpace(user.Name)
	if username == "" {
		username = strings.TrimSpace(info.Username)
	}
	updates := map[string]any{}
	if setAuthSource {
		updates["auth_source"] = store.AuthSourceLDAP
	}
	if dn := strings.TrimSpace(info.DisplayName); dn != "" && dn != user.DisplayName {
		updates["display_name"] = dn
	}
	email := resolveLDAPUserEmail(username, info.Email)
	if normalizeEmail(user.Email) != email {
		updates["email"] = email
	}
	if len(updates) == 0 {
		return user, nil
	}
	if err := h.store.UpdateUserFields(user.ID, updates); err != nil {
		return user, err
	}
	return h.store.GetUserByID(user.ID)
}

func (h *Handler) refreshLDAPUserFromAD(user *store.User) *store.User {
	if user == nil || !user.IsLDAP() || h.ldap == nil || !h.ldap.Enabled() {
		return user
	}
	info, err := h.ldap.LookupUser(user.Name)
	if err != nil {
		return user
	}
	updated, err := h.applyLDAPProfile(user, info, false)
	if err != nil {
		return user
	}
	return updated
}

func userJSON(token string, user *store.User) gin.H {
	displayName := strings.TrimSpace(user.DisplayName)
	if displayName == "" {
		displayName = user.Name
	}
	authSource := strings.TrimSpace(user.AuthSource)
	if authSource == "" {
		authSource = store.AuthSourceLocal
	}
	isAdmin, isSuperAdmin := userRoleFlags(user.Role)
	return gin.H{
		"token": token,
		"user": gin.H{
			"id":             user.ID,
			"name":           user.Name,
			"display_name":   displayName,
			"email":          strings.TrimSpace(user.Email),
			"role":           user.Role,
			"is_admin":       isAdmin,
			"is_super_admin": isSuperAdmin,
			"auth_source":    authSource,
			"status":         user.Status,
		},
	}
}

func (h *Handler) me(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id, _ := userID.(uint)
	user, err := h.store.GetUserByID(id)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	user = h.refreshLDAPUserFromAD(user)
	displayName := strings.TrimSpace(user.DisplayName)
	if displayName == "" {
		displayName = user.Name
	}
	authSource := strings.TrimSpace(user.AuthSource)
	if authSource == "" {
		authSource = store.AuthSourceLocal
	}
	isAdmin, isSuperAdmin := userRoleFlags(user.Role)
	c.JSON(http.StatusOK, gin.H{
		"id":             user.ID,
		"name":           user.Name,
		"display_name":   displayName,
		"email":          strings.TrimSpace(user.Email),
		"role":           user.Role,
		"is_admin":       isAdmin,
		"is_super_admin": isSuperAdmin,
		"auth_source":    authSource,
		"status":         user.Status,
	})
}

func (h *Handler) logout(c *gin.Context) {
	h.writeAudit(c, "logout", "success", "user logout")
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func (h *Handler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) auditLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	keyword := strings.TrimSpace(c.Query("keyword"))
	startAt, err := parseQueryTime(c.Query("start_at"), false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_at"})
		return
	}
	endAt, err := parseQueryTime(c.Query("end_at"), true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_at"})
		return
	}
	items, total, err := h.store.ListAuditLogs(page, size, keyword, startAt, endAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	result := make([]gin.H, 0, len(items))
	for _, item := range items {
		displayName := strings.TrimSpace(item.DisplayName)
		if displayName == "" {
			displayName = item.Username
		}
		result = append(result, gin.H{
			"id": item.ID, "created_at": item.CreatedAt.Format(time.RFC3339),
			"user_id": item.UserID, "username": item.Username, "display_name": displayName,
			"action": item.Action, "result": item.Result, "ip": item.IP, "detail": item.Detail,
		})
	}
	c.JSON(http.StatusOK, gin.H{"total": total, "items": result, "page": page, "size": size})
}
