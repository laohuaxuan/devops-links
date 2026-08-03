package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"devops-links/internal/auth"
	"devops-links/internal/config"
	"devops-links/internal/ldapauth"
	"devops-links/internal/store"

	"github.com/gin-gonic/gin"
)

func TestUpdateProfileEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg, err := config.Load("../../config.yaml")
	if err != nil {
		t.Skip(err)
	}
	st, err := store.New(cfg.Database.DSN())
	if err != nil {
		t.Fatal(err)
	}
	expiry, _ := time.ParseDuration(cfg.Auth.TokenExpiry)
	authSvc := auth.New(st, auth.Config{JWTSecret: cfg.Auth.JWTSecret, TokenExpiry: expiry})
	h := NewHandler(cfg, st, authSvc, ldapauth.New(cfg.Auth.LDAP))

	user, err := st.GetUserByName("root")
	if err != nil {
		t.Fatal(err)
	}
	origEmail := user.Email
	t.Cleanup(func() {
		_ = st.UpdateUserFields(user.ID, map[string]any{
			"display_name": user.DisplayName,
			"email":        origEmail,
		})
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body, _ := json.Marshal(map[string]string{
		"display_name": "root",
		"email":        "profile-test@example.com",
	})
	c.Request = httptest.NewRequest(http.MethodPut, "/api/profile", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", user.ID)
	c.Set("username", user.Name)
	c.Set("role", user.Role)
	h.updateProfile(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	updated, err := st.GetUserByID(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Email != "profile-test@example.com" {
		t.Fatalf("email not updated, got %q", updated.Email)
	}
}
