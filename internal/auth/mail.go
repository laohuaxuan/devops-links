package auth

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

var (
	ErrSMTPConfigInvalid = errors.New("smtp config invalid")
	ErrSMTPConnect       = errors.New("smtp connect failed")
	ErrSMTPAuth          = errors.New("smtp auth failed")
	ErrSMTPRecipient     = errors.New("smtp recipient rejected")
	ErrSMTPSend          = errors.New("smtp send failed")
)

func (a *Service) GenerateResetToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (a *Service) SendPasswordResetEmail(to, resetURL string, ttl time.Duration) error {
	return a.sendMail(
		to,
		"DevOps链接导航平台密码重置通知",
		fmt.Sprintf(
			"您好，\n\n我们收到了您的密码重置请求。请点击下面的链接设置新密码：\n%s\n\n该链接将在 %d 分钟后失效，且只能使用一次。\n如果这不是您的操作，请忽略本邮件并尽快检查账号安全。\n",
			resetURL,
			int(ttl.Minutes()),
		),
	)
}

func (a *Service) SendPasswordChangedEmail(to, ip, device string, changedAt time.Time) error {
	return a.sendMail(
		to,
		"DevOps链接导航平台密码修改成功通知",
		fmt.Sprintf(
			"您好，\n\n您的账号密码已于 %s 修改成功。\n登录 IP：%s\n设备信息：%s\n\n如果这不是您的操作，请立即联系管理员。\n",
			changedAt.Format("2006-01-02 15:04:05"),
			ip,
			device,
		),
	)
}

func (a *Service) sendMail(to, subject, body string) error {
	if a.smtpHost == "" || a.smtpPort == 0 || a.smtpFrom == "" {
		return fmt.Errorf("%w: smtp_host/smtp_port/smtp_from required", ErrSMTPConfigInvalid)
	}

	headers := map[string]string{
		"From":         a.smtpFrom,
		"To":           to,
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/plain; charset=UTF-8",
	}

	var msg bytes.Buffer
	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	addr := fmt.Sprintf("%s:%d", a.smtpHost, a.smtpPort)
	if a.smtpPort == 465 {
		return a.sendMailWithTLS(addr, to, msg.Bytes())
	}

	var auth smtp.Auth
	if a.smtpUser != "" && a.smtpPass != "" {
		auth = smtp.PlainAuth("", a.smtpUser, a.smtpPass, a.smtpHost)
	}
	if err := smtp.SendMail(addr, auth, a.smtpFrom, []string{to}, msg.Bytes()); err != nil {
		return wrapSMTPError(err)
	}
	return nil
}

func (a *Service) sendMailWithTLS(addr, to string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: a.smtpHost})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSMTPConnect, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, a.smtpHost)
	if err != nil {
		return err
	}
	defer client.Close()

	if a.smtpUser != "" && a.smtpPass != "" {
		auth := smtp.PlainAuth("", a.smtpUser, a.smtpPass, a.smtpHost)
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(auth); err != nil {
				return wrapSMTPError(err)
			}
		}
	}

	if err := client.Mail(a.smtpFrom); err != nil {
		return wrapSMTPError(err)
	}
	if err := client.Rcpt(to); err != nil {
		return wrapSMTPError(err)
	}

	wc, err := client.Data()
	if err != nil {
		return wrapSMTPError(err)
	}
	if _, err := wc.Write(msg); err != nil {
		return wrapSMTPError(err)
	}
	if err := wc.Close(); err != nil {
		return wrapSMTPError(err)
	}
	return client.Quit()
}

func wrapSMTPError(err error) error {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "535"), strings.Contains(msg, "authentication failed"), strings.Contains(msg, "auth"):
		return fmt.Errorf("%w: %v", ErrSMTPAuth, err)
	case strings.Contains(msg, "550"), strings.Contains(msg, "553"), strings.Contains(msg, "rcpt"), strings.Contains(msg, "recipient"):
		return fmt.Errorf("%w: %v", ErrSMTPRecipient, err)
	case strings.Contains(msg, "connect"), strings.Contains(msg, "dial"), strings.Contains(msg, "timeout"), strings.Contains(msg, "refused"):
		return fmt.Errorf("%w: %v", ErrSMTPConnect, err)
	default:
		return fmt.Errorf("%w: %v", ErrSMTPSend, err)
	}
}
