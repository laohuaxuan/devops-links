package httpapi

import (
	"crypto/rand"
	"math/big"
	"regexp"
	"strings"
)

const (
	usernameRuleError = "用户名仅支持字母、数字、点、下划线、连字符，长度 2-64"
	passwordRuleError = "密码不符合规范（需包含大小写字母、数字、特殊字符，6-24 位）"
	ldapPasswordHint  = "LDAP 用户请在域内修改密码"
)

var (
	usernameRegex     = regexp.MustCompile(`^[a-zA-Z0-9._-]{2,64}$`)
	hasUppercaseRegex = regexp.MustCompile(`[A-Z]`)
	hasLowercaseRegex = regexp.MustCompile(`[a-z]`)
	hasDigitRegex     = regexp.MustCompile(`[0-9]`)
	hasSpecialRegex   = regexp.MustCompile(`[!@#$%^&*]`)
	emailRegex        = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
)

var reservedUsernames = []string{
	"superadmin", "admin", "guest", "administrator", "root", "system",
}

func validateUsername(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "用户名不能为空"
	}
	if !usernameRegex.MatchString(name) {
		return usernameRuleError
	}
	return ""
}

func validatePassword(password string) string {
	password = strings.TrimSpace(password)
	if password == "" {
		return "密码不能为空"
	}
	if !isPasswordValid(password) {
		return passwordRuleError
	}
	return ""
}

func isPasswordValid(password string) bool {
	if len(password) < 6 || len(password) > 24 {
		return false
	}
	return hasUppercaseRegex.MatchString(password) &&
		hasLowercaseRegex.MatchString(password) &&
		hasDigitRegex.MatchString(password) &&
		hasSpecialRegex.MatchString(password)
}

func validateEmail(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return "邮箱不能为空"
	}
	if !emailRegex.MatchString(email) {
		return "邮箱格式无效"
	}
	return ""
}

func isReservedUsername(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, reserved := range reservedUsernames {
		if lower == reserved {
			return true
		}
	}
	return false
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func normalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}

func maskEmail(email string) string {
	email = strings.TrimSpace(email)
	at := strings.LastIndex(email, "@")
	if at <= 1 {
		return "xxxx@xxx.xx"
	}
	local := email[:at]
	domain := email[at+1:]
	if len(local) <= 2 {
		return local[:1] + "***@" + domain
	}
	return local[:2] + "***@" + domain
}

func pickCharset(set string) (byte, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
	if err != nil {
		return 0, err
	}
	return set[n.Int64()], nil
}

func generateRandomPassword(length int) (string, error) {
	if length < 6 {
		length = 12
	}
	if length > 24 {
		length = 24
	}
	const (
		upper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		lower   = "abcdefghijklmnopqrstuvwxyz"
		digits  = "0123456789"
		special = "!@#$%^&*"
	)
	all := upper + lower + digits + special
	b := make([]byte, length)
	var err error
	if b[0], err = pickCharset(upper); err != nil {
		return "", err
	}
	if b[1], err = pickCharset(lower); err != nil {
		return "", err
	}
	if b[2], err = pickCharset(digits); err != nil {
		return "", err
	}
	if b[3], err = pickCharset(special); err != nil {
		return "", err
	}
	for i := 4; i < length; i++ {
		if b[i], err = pickCharset(all); err != nil {
			return "", err
		}
	}
	for i := len(b) - 1; i > 0; i-- {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		j := int(jBig.Int64())
		b[i], b[j] = b[j], b[i]
	}
	pwd := string(b)
	if !isPasswordValid(pwd) {
		return generateRandomPassword(length)
	}
	return pwd, nil
}
