package store

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	AuthSourceLocal = "local"
	AuthSourceLDAP  = "ldap"

	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"
)

type AuditLog struct {
	ID          uint `gorm:"primaryKey;autoIncrement"`
	CreatedAt   time.Time
	UserID      uint   `gorm:"index"`
	Username    string `gorm:"size:100;index"`
	DisplayName string `gorm:"size:120;index"`
	Action      string `gorm:"size:60;index"`
	Result      string `gorm:"size:20;index"`
	IP          string `gorm:"size:64"`
	Detail      string `gorm:"size:1000"`
}

func (u *User) IsLDAP() bool {
	if u == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(u.AuthSource), AuthSourceLDAP)
}

func (u *User) IsActive() bool {
	if u == nil {
		return false
	}
	return strings.ToLower(strings.TrimSpace(u.Status)) != UserStatusDisabled
}

func (s *Store) GetUserByEmail(email string) (*User, error) {
	var u User
	if err := s.db.Where("email = ?", strings.TrimSpace(strings.ToLower(email))).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) SaveResetToken(token *ResetToken) error {
	return s.db.Create(token).Error
}

func (s *Store) GetResetToken(token string) (*ResetToken, error) {
	var rt ResetToken
	if err := s.db.Where("token = ?", token).First(&rt).Error; err != nil {
		return nil, err
	}
	return &rt, nil
}

func (s *Store) GetResetTokenByUserID(userID uint) (*ResetToken, error) {
	var rt ResetToken
	if err := s.db.Where("user_id = ?", userID).First(&rt).Error; err != nil {
		return nil, err
	}
	return &rt, nil
}

func (s *Store) DeleteResetToken(userID uint) error {
	return s.db.Where("user_id = ?", userID).Delete(&ResetToken{}).Error
}

func (s *Store) CreatePasswordResetRequestLog(log *PasswordResetRequestLog) error {
	return s.db.Create(log).Error
}

func (s *Store) ListUsers(page, pageSize int, keyword string) ([]User, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	var items []User
	var total int64
	query := s.db.Model(&User{}).Order("id asc")
	if kw := strings.TrimSpace(keyword); kw != "" {
		like := "%" + kw + "%"
		query = query.Where("name LIKE ? OR display_name LIKE ? OR email LIKE ?", like, like, like)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *Store) CountUsersByRole(role string) (int64, error) {
	var total int64
	err := s.db.Model(&User{}).Where("role = ?", role).Count(&total).Error
	return total, err
}

func (s *Store) UpdateUserFields(id uint, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return s.db.Model(&User{}).Where("id = ?", id).Updates(fields).Error
}

func (s *Store) UpdateUserInfo(id uint, displayName, role, status string) error {
	return s.db.Model(&User{}).Where("id = ?", id).Updates(map[string]any{
		"display_name": displayName,
		"role":         role,
		"status":       status,
	}).Error
}

func (s *Store) UpdateUserPassword(id uint, hash string) error {
	return s.db.Model(&User{}).Where("id = ?", id).Updates(map[string]any{
		"password_hash":       hash,
		"password_changed_at": time.Now(),
	}).Error
}

func (s *Store) DeleteUser(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&ResetToken{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&PasswordResetRequestLog{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&User{}, id).Error
	})
}

func (s *Store) WriteAudit(log *AuditLog) error {
	return s.db.Create(log).Error
}

func auditKeywordVariants(keyword string) []string {
	seen := map[string]struct{}{keyword: {}}
	out := []string{keyword}
	aliases := map[string]string{
		"登录": "login",
		"登出": "logout",
		"成功": "success",
		"失败": "failed",
	}
	for zh, en := range aliases {
		if strings.Contains(keyword, zh) || strings.Contains(zh, keyword) {
			if _, ok := seen[en]; !ok {
				seen[en] = struct{}{}
				out = append(out, en)
			}
		}
		if strings.Contains(keyword, en) || strings.EqualFold(keyword, en) {
			if _, ok := seen[zh]; !ok {
				seen[zh] = struct{}{}
				out = append(out, zh)
			}
		}
	}
	return out
}

func (s *Store) ListAuditLogs(page, pageSize int, keyword string, startAt, endAt *time.Time) ([]AuditLog, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	var items []AuditLog
	var total int64
	query := s.db.Model(&AuditLog{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		likes := auditKeywordVariants(keyword)
		var parts []string
		var args []any
		for _, term := range likes {
			like := "%" + term + "%"
			parts = append(parts, "(username LIKE ? OR display_name LIKE ? OR action LIKE ? OR result LIKE ? OR ip LIKE ? OR detail LIKE ?)")
			args = append(args, like, like, like, like, like, like)
		}
		query = query.Where(strings.Join(parts, " OR "), args...)
	}
	if startAt != nil {
		query = query.Where("created_at >= ?", *startAt)
	}
	if endAt != nil {
		query = query.Where("created_at <= ?", *endAt)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := query.Order("created_at desc").Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
