package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"devops-links/internal/auth"
	"devops-links/internal/store"

	"github.com/gin-gonic/gin"
)

type registerRequest struct {
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
	CaptchaToken    string `json:"captcha_token"`
	CaptchaCode     string `json:"captcha_code"`
}

type resetPasswordRequest struct {
	Account string `json:"account"`
}

type changePasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func buildResetPasswordURL(c *gin.Context, token string) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if proto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); proto != "" {
		scheme = strings.ToLower(strings.Split(proto, ",")[0])
	}
	host := c.Request.Host
	if host == "" {
		host = "localhost:8090"
	}
	return fmt.Sprintf("%s://%s/reset-password?token=%s", scheme, host, url.QueryEscape(token))
}

func localPlaceholderEmail(username string) string {
	return strings.ToLower(strings.TrimSpace(username)) + "@local.dev"
}

// resolveLDAPUserEmail returns LDAP mail when available, otherwise username@local.dev.
// Platform-managed emails are always replaced on LDAP login/profile sync.
func resolveLDAPUserEmail(username, fromLDAP string) string {
	fromLDAP = normalizeEmail(fromLDAP)
	if fromLDAP != "" && emailRegex.MatchString(fromLDAP) {
		return fromLDAP
	}
	return localPlaceholderEmail(username)
}

func (h *Handler) checkExists(c *gin.Context) {
	field := strings.TrimSpace(c.Query("field"))
	value := strings.TrimSpace(c.Query("value"))
	if field == "" || value == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var err error
	switch field {
	case "username":
		_, err = h.store.GetUserByName(value)
	case "email":
		_, err = h.store.GetUserByEmail(normalizeEmail(value))
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid field"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"exists": err == nil})
}

func (h *Handler) register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Email = normalizeEmail(req.Email)
	req.Password = strings.TrimSpace(req.Password)
	req.ConfirmPassword = strings.TrimSpace(req.ConfirmPassword)
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
	if isReservedUsername(req.Username) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该用户名不允许注册"})
		return
	}
	if req.DisplayName == "" {
		req.DisplayName = req.Username
	}
	if len(req.DisplayName) > 120 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "显示名最长 120 字符"})
		return
	}
	if msg := validateEmail(req.Email); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
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
	if _, err := h.store.GetUserByName(req.Username); err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名已存在"})
		return
	}
	if _, err := h.store.GetUserByEmail(req.Email); err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱已被注册"})
		return
	}

	role := strings.TrimSpace(h.cfg.Auth.RegistrationRole)
	if role != store.RoleGuest {
		role = store.RoleGuest
	}

	hash, err := h.auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "hash password failed"})
		return
	}
	user := &store.User{
		Name: req.Username, DisplayName: req.DisplayName, Email: req.Email,
		PasswordHash: hash, AuthSource: store.AuthSourceLocal, Role: role, Status: store.UserStatusActive,
	}
	if err := h.store.CreateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = h.store.WriteAudit(&store.AuditLog{
		UserID: user.ID, Username: user.Name, DisplayName: user.DisplayName,
		Action: "register", Result: "success", IP: c.ClientIP(), Detail: "registered",
	})
	c.JSON(http.StatusOK, gin.H{"message": "registered"})
}

func (h *Handler) requestResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	account := strings.TrimSpace(req.Account)
	if account == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入用户名或邮箱"})
		return
	}

	var user *store.User
	var err error
	if strings.Contains(account, "@") {
		user, err = h.store.GetUserByEmail(normalizeEmail(account))
	} else {
		user, err = h.store.GetUserByName(account)
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "重置密码已发送到您的注册邮箱" + maskEmail("xx@xxx.xx")})
		return
	}
	if user.IsLDAP() {
		c.JSON(http.StatusBadRequest, gin.H{"error": ldapPasswordHint})
		return
	}
	if strings.TrimSpace(user.Email) == "" ||
		strings.HasSuffix(strings.ToLower(user.Email), "@ldap.local") ||
		strings.HasSuffix(strings.ToLower(user.Email), "@local.dev") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该账号未绑定有效邮箱，请联系管理员"})
		return
	}

	expireDuration := 30 * time.Minute
	nowUnix := time.Now().Unix()
	if existing, err := h.store.GetResetTokenByUserID(user.ID); err == nil {
		if existing.ExpiresAt >= nowUnix {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "重置邮件已发送，30分钟内请勿重复点击，请查收邮箱"})
			return
		}
		_ = h.store.DeleteResetToken(user.ID)
	}

	if err := h.store.DeleteResetToken(user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "发送重置链接失败"})
		return
	}

	token := h.auth.GenerateResetToken()
	if err := h.store.SaveResetToken(&store.ResetToken{
		UserID: user.ID, Token: token, ExpiresAt: time.Now().Add(expireDuration).Unix(),
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "发送重置链接失败"})
		return
	}

	clientIP := c.ClientIP()
	clientDevice := c.Request.UserAgent()
	resetURL := buildResetPasswordURL(c, token)
	if err := h.auth.SendPasswordResetEmail(user.Email, resetURL, expireDuration); err != nil {
		_ = h.store.DeleteResetToken(user.ID)
		switch {
		case errors.Is(err, auth.ErrSMTPConfigInvalid):
			c.JSON(http.StatusInternalServerError, gin.H{"error": "邮件服务未配置"})
		case errors.Is(err, auth.ErrSMTPConnect):
			c.JSON(http.StatusBadGateway, gin.H{"error": "邮件服务不可用"})
		case errors.Is(err, auth.ErrSMTPAuth):
			c.JSON(http.StatusBadGateway, gin.H{"error": "邮件服务认证失败"})
		case errors.Is(err, auth.ErrSMTPRecipient):
			c.JSON(http.StatusBadRequest, gin.H{"error": "注册邮箱不可用"})
		default:
			c.JSON(http.StatusBadGateway, gin.H{"error": "发送重置邮件失败"})
		}
		return
	}

	if err := h.store.CreatePasswordResetRequestLog(&store.PasswordResetRequestLog{
		UserID: user.ID, Email: user.Email, IP: clientIP, Device: clientDevice,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "发送重置链接失败"})
		return
	}

	_ = h.store.WriteAudit(&store.AuditLog{
		UserID: user.ID, Username: user.Name, DisplayName: user.DisplayName,
		Action: "reset_password_request", Result: "success", IP: clientIP, Detail: "password reset link sent",
	})
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("重置密码已发送到您的注册邮箱%s", user.Email)})
}

func (h *Handler) changePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	req.NewPassword = strings.TrimSpace(req.NewPassword)
	if msg := validatePassword(req.NewPassword); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	resetToken, err := h.store.GetResetToken(req.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该重置链接已失效或已使用，请重新发起找回密码"})
		return
	}
	if time.Now().Unix() > resetToken.ExpiresAt {
		_ = h.store.DeleteResetToken(resetToken.UserID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "该重置链接已过期，请重新发起找回密码"})
		return
	}

	user, err := h.store.GetUserByID(resetToken.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该重置链接已失效，请重新发起找回密码"})
		return
	}
	if user.IsLDAP() {
		_ = h.store.DeleteResetToken(resetToken.UserID)
		c.JSON(http.StatusBadRequest, gin.H{"error": ldapPasswordHint})
		return
	}
	if h.auth.ComparePassword(user.PasswordHash, req.NewPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码不能与旧密码相同"})
		return
	}

	hash, err := h.auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新密码失败"})
		return
	}
	if err := h.store.UpdateUserPassword(resetToken.UserID, hash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新密码失败"})
		return
	}
	if err := h.store.DeleteResetToken(resetToken.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新密码失败"})
		return
	}

	clientIP := c.ClientIP()
	clientDevice := c.Request.UserAgent()
	_ = h.auth.SendPasswordChangedEmail(user.Email, clientIP, clientDevice, time.Now())
	_ = h.store.WriteAudit(&store.AuditLog{
		UserID: user.ID, Username: user.Name, DisplayName: user.DisplayName,
		Action: "change_password", Result: "success", IP: clientIP, Detail: "password changed via reset link",
	})
	c.JSON(http.StatusOK, gin.H{"message": "password changed"})
}
