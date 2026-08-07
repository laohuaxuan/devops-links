package httpapi

import (
	"errors"
	"strings"

	"devops-links/internal/auth"
	"devops-links/internal/config"
	"devops-links/internal/ldapauth"
	"devops-links/internal/store"

	"gorm.io/gorm"
)

type Handler struct {
	cfg   *config.Config
	store *store.Store
	auth  *auth.Service
	ldap  *ldapauth.Client
}

func NewHandler(cfg *config.Config, st *store.Store, authSvc *auth.Service, ldapClient *ldapauth.Client) *Handler {
	return &Handler{cfg: cfg, store: st, auth: authSvc, ldap: ldapClient}
}

func (h *Handler) EnsureInitialUsers() error {
	count, err := h.store.CountUsers()
	if err != nil {
		return err
	}
	if count > 0 {
		return h.EnsureSuperAdmin()
	}
	superHash, err := h.auth.HashPassword(h.cfg.Auth.SuperAdminInitialPassword)
	if err != nil {
		return err
	}
	return h.store.CreateUser(&store.User{
		Name: h.cfg.Auth.SuperAdminInitialName, DisplayName: h.cfg.Auth.SuperAdminInitialName,
		Email: localPlaceholderEmail(h.cfg.Auth.SuperAdminInitialName),
		PasswordHash: superHash, AuthSource: store.AuthSourceLocal, Role: store.RoleSuperAdmin, Status: store.UserStatusActive,
	})
}

func (h *Handler) EnsureSuperAdmin() error {
	name := strings.TrimSpace(h.cfg.Auth.SuperAdminInitialName)
	if name == "" {
		name = "root"
	}
	if _, err := h.store.GetUserByName(name); err == nil {
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	hash, err := h.auth.HashPassword(h.cfg.Auth.SuperAdminInitialPassword)
	if err != nil {
		return err
	}
	return h.store.CreateUser(&store.User{
		Name: name, DisplayName: name,
		Email: localPlaceholderEmail(name),
		PasswordHash: hash, AuthSource: store.AuthSourceLocal, Role: store.RoleSuperAdmin, Status: store.UserStatusActive,
	})
}
