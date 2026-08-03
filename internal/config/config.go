package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Upload   UploadConfig   `yaml:"upload"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
	Charset  string `yaml:"charset"`
}

type AuthConfig struct {
	JWTSecret            string     `yaml:"jwt_secret"`
	TokenExpiry          string     `yaml:"token_expiry"`
	SuperAdminInitialName     string     `yaml:"superadmin_initial_name"`
	SuperAdminInitialPassword string     `yaml:"superadmin_initial_password"`
	SMTPHost             string     `yaml:"smtp_host"`
	SMTPPort             int        `yaml:"smtp_port"`
	SMTPUser             string     `yaml:"smtp_user"`
	SMTPPass             string     `yaml:"smtp_pass"`
	SMTPFrom             string     `yaml:"smtp_from"`
	RegistrationRole     string     `yaml:"registration_default_role"`
	LDAP                 LDAPConfig `yaml:"ldap"`
}

type LDAPConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	UseSSL          bool   `yaml:"use_ssl"`
	StartTLS        bool   `yaml:"start_tls"`
	SkipTLSVerify   bool   `yaml:"skip_tls_verify"`
	BindDN          string `yaml:"bind_dn"`
	BindPassword    string `yaml:"bind_password"`
	BaseDN          string `yaml:"base_dn"`
	UserFilter      string `yaml:"user_filter"`
	UsernameAttr    string `yaml:"username_attr"`
	EmailAttr       string `yaml:"email_attr"`
	DisplayNameAttr string `yaml:"display_name_attr"`
	Label           string `yaml:"label"`
	DefaultRole     string `yaml:"default_role"`
}

type UploadConfig struct {
	Dir         string `yaml:"dir"`
	MaxSizeMB   int    `yaml:"max_size_mb"`
	URLPrefix   string `yaml:"url_prefix"`
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	normalize(cfg)
	return cfg, nil
}

func normalize(cfg *Config) {
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8090
	}
	if cfg.Database.Port == 0 {
		cfg.Database.Port = 3306
	}
	if cfg.Database.Charset == "" {
		cfg.Database.Charset = "utf8mb4"
	}
	if cfg.Database.Host == "" {
		cfg.Database.Host = "127.0.0.1"
	}
	if cfg.Database.User == "" {
		cfg.Database.User = "root"
	}
	if cfg.Database.Name == "" {
		cfg.Database.Name = "devops_links"
	}
	if cfg.Auth.JWTSecret == "" {
		cfg.Auth.JWTSecret = "devops-links-secret"
	}
	if cfg.Auth.TokenExpiry == "" {
		cfg.Auth.TokenExpiry = "24h"
	}
	if cfg.Auth.SuperAdminInitialName == "" {
		cfg.Auth.SuperAdminInitialName = "root"
	}
	if cfg.Auth.SuperAdminInitialPassword == "" {
		cfg.Auth.SuperAdminInitialPassword = "Admin@123456"
	}
	if cfg.Auth.RegistrationRole == "" {
		cfg.Auth.RegistrationRole = "guest"
	}
	if cfg.Auth.LDAP.DefaultRole == "" {
		cfg.Auth.LDAP.DefaultRole = "guest"
	}
	if cfg.Upload.Dir == "" {
		cfg.Upload.Dir = "data/uploads/icons"
	}
	if cfg.Upload.MaxSizeMB <= 0 {
		cfg.Upload.MaxSizeMB = 2
	}
	if cfg.Upload.URLPrefix == "" {
		cfg.Upload.URLPrefix = "/uploads/icons"
	}
}

func (d DatabaseConfig) DSN() string {
	port := d.Port
	if port <= 0 {
		port = 3306
	}
	charset := d.Charset
	if charset == "" {
		charset = "utf8mb4"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		d.User, d.Password, d.Host, port, d.Name, charset)
}

func (d DatabaseConfig) RootDSN() string {
	port := d.Port
	if port <= 0 {
		port = 3306
	}
	charset := d.Charset
	if charset == "" {
		charset = "utf8mb4"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=%s&parseTime=True&loc=Local",
		d.User, d.Password, d.Host, port, charset)
}

func (u UploadConfig) MaxBytes() int64 {
	return int64(u.MaxSizeMB) * 1024 * 1024
}

func (u UploadConfig) AbsDir() string {
	return strings.TrimSpace(u.Dir)
}
