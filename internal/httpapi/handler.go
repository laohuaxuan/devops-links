package httpapi

import (
	"devops-links/internal/auth"
	"devops-links/internal/config"
	"devops-links/internal/ldapauth"
	"devops-links/internal/store"
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
	count, err := h.store.CountUsersByRole(store.RoleSuperAdmin)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := h.auth.HashPassword(h.cfg.Auth.SuperAdminInitialPassword)
	if err != nil {
		return err
	}
	return h.store.CreateUser(&store.User{
		Name: h.cfg.Auth.SuperAdminInitialName, DisplayName: h.cfg.Auth.SuperAdminInitialName,
		Email: localPlaceholderEmail(h.cfg.Auth.SuperAdminInitialName),
		PasswordHash: hash, AuthSource: store.AuthSourceLocal, Role: store.RoleSuperAdmin, Status: store.UserStatusActive,
	})
}
