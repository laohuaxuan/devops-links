package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	RoleSuperAdmin = "superadmin"
	RoleAdmin      = "admin"
	RoleGuest      = "guest"

	CategoryScopeShared   = "shared"
	CategoryScopePersonal = "personal"
)

func IsPrivilegedRole(role string) bool {
	return role == RoleAdmin || role == RoleSuperAdmin
}

func IsSuperAdminRole(role string) bool {
	return role == RoleSuperAdmin
}

func IsValidRole(role string) bool {
	switch role {
	case RoleSuperAdmin, RoleAdmin, RoleGuest:
		return true
	default:
		return false
	}
}

type User struct {
	ID                uint      `gorm:"primaryKey;autoIncrement"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Name              string    `gorm:"uniqueIndex;size:64;not null"`
	DisplayName       string    `gorm:"size:120"`
	Email             string    `gorm:"uniqueIndex;size:120"`
	PasswordHash      string    `gorm:"size:255"`
	PasswordChangedAt time.Time `gorm:"index"`
	AuthSource        string    `gorm:"size:20;default:local;index"`
	Role              string    `gorm:"size:16;not null;index"`
	Status            string    `gorm:"size:20;default:active;index"`
}

type Category struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time
	UpdatedAt time.Time
	ParentID  uint   `gorm:"index;default:0;uniqueIndex:idx_category_scope_owner_parent_name"`
	Name      string `gorm:"size:128;not null;uniqueIndex:idx_category_scope_owner_parent_name"`
	Scope     string `gorm:"size:16;not null;default:shared;index;uniqueIndex:idx_category_scope_owner_parent_name"`
	OwnerID   uint   `gorm:"index;default:0;uniqueIndex:idx_category_scope_owner_parent_name"`
	SortOrder int    `gorm:"default:0"`
	CreatedBy uint   `gorm:"index;default:0"`
}

func (c *Category) IsRoot() bool {
	return c.ParentID == 0
}

func (c *Category) IsPersonal() bool {
	return c.Scope == CategoryScopePersonal
}

type Link struct {
	ID         uint      `gorm:"primaryKey;autoIncrement"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	CategoryID uint   `gorm:"index;not null"`
	Name       string `gorm:"size:128;not null"`
	URL        string `gorm:"size:512;not null"`
	IconPath   string `gorm:"size:512"`
	Maintainer string `gorm:"size:128"`
	Remark     string `gorm:"type:text"`
	SortOrder  int    `gorm:"default:0"`
	CreatedBy  uint   `gorm:"index;default:0"`
}

type ResetToken struct {
	ID        uint `gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time
	UpdatedAt time.Time
	UserID    uint   `gorm:"uniqueIndex"`
	Token     string `gorm:"uniqueIndex;size:64"`
	ExpiresAt int64  `gorm:"index"`
}

type PasswordResetRequestLog struct {
	ID        uint `gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time
	UserID    uint   `gorm:"index"`
	Email     string `gorm:"size:120;index"`
	IP        string `gorm:"size:64"`
	Device    string `gorm:"size:512"`
}

type Store struct {
	db *gorm.DB
}

func EnsureDatabase(host string, port int, user, password, name, charset string) error {
	if charset == "" {
		charset = "utf8mb4"
	}
	if port <= 0 {
		port = 3306
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=%s&parseTime=True&loc=Local", user, password, host, port, charset)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("database name is required")
	}
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET %s", name, charset))
	return err
}

func New(dsn string) (*Store, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.AutoMigrate(&User{}, &Category{}, &Link{}, &AuditLog{}, &ResetToken{}, &PasswordResetRequestLog{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	_ = db.Model(&Category{}).Where("scope = '' OR scope IS NULL").Update("scope", CategoryScopeShared).Error
	var zero time.Time
	_ = db.Model(&User{}).Where("password_changed_at IS NULL OR password_changed_at = ?", zero).
		Update("password_changed_at", time.Now()).Error
	var users []User
	if err := db.Where("email = '' OR email IS NULL").Find(&users).Error; err == nil {
		for _, u := range users {
			email := strings.ToLower(strings.TrimSpace(u.Name)) + "@local.dev"
			_ = db.Model(&User{}).Where("id = ?", u.ID).Update("email", email).Error
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) CountUsers() (int64, error) {
	var n int64
	err := s.db.Model(&User{}).Count(&n).Error
	return n, err
}

func (s *Store) CreateUser(u *User) error {
	if u.PasswordChangedAt.IsZero() {
		u.PasswordChangedAt = time.Now()
	}
	return s.db.Create(u).Error
}

func (s *Store) GetUserByName(name string) (*User, error) {
	var u User
	if err := s.db.Where("name = ?", name).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUserByID(id uint) (*User, error) {
	var u User
	if err := s.db.First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) ListCategoriesByScope(scope string, ownerID uint) ([]Category, error) {
	var items []Category
	query := s.db.Where("scope = ?", scope)
	if scope == CategoryScopePersonal {
		query = query.Where("owner_id = ?", ownerID)
	}
	if err := query.Order("sort_order asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ListCategories() ([]Category, error) {
	return s.ListCategoriesByScope(CategoryScopeShared, 0)
}

func (s *Store) GetCategory(id uint) (*Category, error) {
	var item Category
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) GetCategoryByParentAndName(parentID uint, name, scope string, ownerID uint) (*Category, error) {
	var item Category
	if err := s.db.Where(
		"parent_id = ? AND name = ? AND scope = ? AND owner_id = ?",
		parentID, strings.TrimSpace(name), scope, ownerID,
	).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) ListChildCategories(parentID uint) ([]Category, error) {
	var items []Category
	if err := s.db.Where("parent_id = ?", parentID).Order("sort_order asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) CreateCategory(item *Category) error {
	return s.db.Create(item).Error
}

func (s *Store) UpdateCategory(item *Category) error {
	return s.db.Model(&Category{}).Where("id = ?", item.ID).Updates(map[string]any{
		"name":       item.Name,
		"sort_order": item.SortOrder,
	}).Error
}

func (s *Store) DeleteCategory(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var childIDs []uint
		if err := tx.Model(&Category{}).Where("parent_id = ?", id).Pluck("id", &childIDs).Error; err != nil {
			return err
		}
		allIDs := append([]uint{id}, childIDs...)
		if err := tx.Where("category_id IN ?", allIDs).Delete(&Link{}).Error; err != nil {
			return err
		}
		if len(childIDs) > 0 {
			if err := tx.Where("parent_id = ?", id).Delete(&Category{}).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&Category{}, id).Error
	})
}

func (s *Store) ListLinksByCategoryID(categoryID uint) ([]Link, error) {
	var items []Link
	if err := s.db.Where("category_id = ?", categoryID).Order("sort_order asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) GetLink(id uint) (*Link, error) {
	var item Link
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) CreateLink(item *Link) error {
	return s.db.Create(item).Error
}

func (s *Store) UpdateLink(item *Link) error {
	return s.db.Model(&Link{}).Where("id = ?", item.ID).Updates(map[string]any{
		"category_id": item.CategoryID,
		"name":        item.Name,
		"url":         item.URL,
		"icon_path":   item.IconPath,
		"maintainer":  item.Maintainer,
		"remark":      item.Remark,
		"sort_order":  item.SortOrder,
	}).Error
}

func (s *Store) DeleteLink(id uint) error {
	return s.db.Delete(&Link{}, id).Error
}
