package auth

import (
	"fmt"
	"time"

	"devops-links/internal/store"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Config struct {
	JWTSecret   string
	TokenExpiry time.Duration
	SMTPHost    string
	SMTPPort    int
	SMTPUser    string
	SMTPPass    string
	SMTPFrom    string
}

type Service struct {
	store    *store.Store
	secret   string
	expiry   time.Duration
	smtpHost string
	smtpPort int
	smtpUser string
	smtpPass string
	smtpFrom string
}

type Claims struct {
	UserID     uint   `json:"user_id"`
	Username   string `json:"username"`
	Role       string `json:"role"`
	PasswordAt int64  `json:"pwd_at"`
	jwt.RegisteredClaims
}

func New(s *store.Store, cfg Config) *Service {
	return &Service{
		store:    s,
		secret:   cfg.JWTSecret,
		expiry:   cfg.TokenExpiry,
		smtpHost: cfg.SMTPHost,
		smtpPort: cfg.SMTPPort,
		smtpUser: cfg.SMTPUser,
		smtpPass: cfg.SMTPPass,
		smtpFrom: cfg.SMTPFrom,
	}
}

func (a *Service) HashPassword(password string) (string, error) {
	raw, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (a *Service) ComparePassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func (a *Service) Login(name, password string) (string, *store.User, error) {
	user, err := a.store.GetUserByName(name)
	if err != nil {
		return "", nil, fmt.Errorf("用户名或密码错误")
	}
	if user.IsLDAP() {
		return "", nil, fmt.Errorf("请使用 LDAP 登录")
	}
	if user.Role != store.RoleSuperAdmin {
		return "", nil, fmt.Errorf("请使用 LDAP 登录")
	}
	if !a.ComparePassword(user.PasswordHash, password) {
		return "", nil, fmt.Errorf("用户名或密码错误")
	}
	token, err := a.GenerateToken(user)
	if err != nil {
		return "", nil, err
	}
	return token, user, nil
}

func (a *Service) GenerateToken(user *store.User) (string, error) {
	pwdAt := user.PasswordChangedAt.Unix()
	if pwdAt <= 0 {
		pwdAt = time.Now().Unix()
	}
	claims := Claims{
		UserID:     user.ID,
		Username:   user.Name,
		Role:       user.Role,
		PasswordAt: pwdAt,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(a.secret))
}

func (a *Service) ParseToken(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		return []byte(a.secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
