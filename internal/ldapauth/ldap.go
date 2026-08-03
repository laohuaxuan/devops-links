package ldapauth

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"devops-links/internal/config"

	ldap "github.com/go-ldap/ldap/v3"
)

type Config = config.LDAPConfig

type UserInfo struct {
	Username    string
	DisplayName string
	Email       string
	DN          string
}

type Client struct {
	cfg Config
}

func New(cfg Config) *Client {
	return &Client{cfg: cfg}
}

func (c *Client) Enabled() bool {
	return c != nil && c.cfg.Enabled && strings.TrimSpace(c.cfg.Host) != "" && strings.TrimSpace(c.cfg.BaseDN) != ""
}

func (c *Client) Label() string {
	if c == nil {
		return "LDAP"
	}
	label := strings.TrimSpace(c.cfg.Label)
	if label == "" {
		return "LDAP"
	}
	return label
}

func (c *Client) Authenticate(username, password string) (*UserInfo, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("LDAP 未启用")
	}
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return nil, fmt.Errorf("LDAP 用户名和密码不能为空")
	}
	if strings.ContainsAny(username, `*()&\`) {
		return nil, fmt.Errorf("LDAP 用户名包含非法字符")
	}

	conn, err := c.dial()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := c.bindService(conn); err != nil {
		return nil, fmt.Errorf("LDAP 服务账号绑定失败: %w", err)
	}

	info, err := c.lookup(conn, username)
	if err != nil {
		return nil, err
	}
	if err := conn.Bind(info.DN, password); err != nil {
		return nil, fmt.Errorf("LDAP 用户名或密码错误")
	}
	return info, nil
}

// LookupUser reads LDAP profile attributes without validating the user's password.
func (c *Client) LookupUser(username string) (*UserInfo, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("LDAP 未启用")
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("LDAP 用户名不能为空")
	}
	if strings.ContainsAny(username, `*()&\`) {
		return nil, fmt.Errorf("LDAP 用户名包含非法字符")
	}

	conn, err := c.dial()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := c.bindService(conn); err != nil {
		return nil, fmt.Errorf("LDAP 服务账号绑定失败: %w", err)
	}
	return c.lookup(conn, username)
}

func (c *Client) lookup(conn *ldap.Conn, username string) (*UserInfo, error) {
	userDN, attrs, err := c.searchUser(conn, username)
	if err != nil {
		return nil, err
	}

	info := &UserInfo{
		Username:    firstAttr(attrs, c.cfg.UsernameAttr, username),
		DisplayName: firstAttr(attrs, c.cfg.DisplayNameAttr, username),
		Email:       firstAttr(attrs, c.cfg.EmailAttr, ""),
		DN:          userDN,
	}
	info.Username = sanitizeUsername(info.Username)
	if info.Username == "" {
		info.Username = sanitizeUsername(username)
	}
	if info.DisplayName == "" {
		info.DisplayName = info.Username
	}
	return info, nil
}

func (c *Client) dial() (*ldap.Conn, error) {
	host := strings.TrimSpace(c.cfg.Host)
	port := c.cfg.Port
	if port <= 0 {
		if c.cfg.UseSSL {
			port = 636
		} else {
			port = 389
		}
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	timeout := 10 * time.Second

	var conn *ldap.Conn
	var err error
	tlsConfig := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: c.cfg.SkipTLSVerify, //nolint:gosec // optional for internal LDAP
	}
	if c.cfg.UseSSL {
		conn, err = ldap.DialTLS("tcp", addr, tlsConfig)
	} else {
		conn, err = ldap.Dial("tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("连接 LDAP 失败: %w", err)
	}
	conn.SetTimeout(timeout)
	if !c.cfg.UseSSL && c.cfg.StartTLS {
		if err := conn.StartTLS(tlsConfig); err != nil {
			conn.Close()
			return nil, fmt.Errorf("LDAP StartTLS 失败: %w", err)
		}
	}
	return conn, nil
}

func (c *Client) bindService(conn *ldap.Conn) error {
	bindDN := strings.TrimSpace(c.cfg.BindDN)
	if bindDN == "" {
		return conn.UnauthenticatedBind("")
	}
	return conn.Bind(bindDN, c.cfg.BindPassword)
}

func (c *Client) searchUser(conn *ldap.Conn, username string) (string, map[string][]string, error) {
	filterTpl := strings.TrimSpace(c.cfg.UserFilter)
	if filterTpl == "" {
		filterTpl = "(uid=%s)"
	}
	if !strings.Contains(filterTpl, "%s") {
		return "", nil, fmt.Errorf("ldap.user_filter 必须包含 %%s 占位符")
	}
	filter := fmt.Sprintf(filterTpl, ldap.EscapeFilter(username))

	usernameAttr := strings.TrimSpace(c.cfg.UsernameAttr)
	if usernameAttr == "" {
		usernameAttr = "uid"
	}
	emailAttr := strings.TrimSpace(c.cfg.EmailAttr)
	if emailAttr == "" {
		emailAttr = "mail"
	}
	displayAttr := strings.TrimSpace(c.cfg.DisplayNameAttr)
	if displayAttr == "" {
		displayAttr = "cn"
	}

	req := ldap.NewSearchRequest(
		c.cfg.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		2,
		10,
		false,
		filter,
		[]string{"dn", usernameAttr, emailAttr, displayAttr},
		nil,
	)
	res, err := conn.Search(req)
	if err != nil {
		return "", nil, fmt.Errorf("LDAP 搜索用户失败: %w", err)
	}
	if len(res.Entries) == 0 {
		return "", nil, fmt.Errorf("LDAP 用户不存在")
	}
	if len(res.Entries) > 1 {
		return "", nil, fmt.Errorf("LDAP 用户匹配到多个结果")
	}
	entry := res.Entries[0]
	attrs := map[string][]string{}
	for _, attr := range entry.Attributes {
		attrs[strings.ToLower(attr.Name)] = attr.Values
	}
	return entry.DN, attrs, nil
}

func firstAttr(attrs map[string][]string, key, fallback string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	if key != "" {
		if vals := attrs[key]; len(vals) > 0 && strings.TrimSpace(vals[0]) != "" {
			return strings.TrimSpace(vals[0])
		}
	}
	return strings.TrimSpace(fallback)
}

func sanitizeUsername(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) > 100 {
		out = out[:100]
	}
	return out
}
