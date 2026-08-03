package httpapi

import (
	"strconv"
	"strings"
	"time"

	"devops-links/internal/store"

	"github.com/gin-gonic/gin"
)

func (h *Handler) writeAudit(c *gin.Context, action, result, detail string) {
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")
	displayName, _ := c.Get("display_name")
	uid, _ := userID.(uint)
	uname, _ := username.(string)
	dname, _ := displayName.(string)
	if strings.TrimSpace(dname) == "" {
		dname = uname
	}
	_ = h.store.WriteAudit(&store.AuditLog{
		UserID:      uid,
		Username:    uname,
		DisplayName: dname,
		Action:      action,
		Result:      result,
		IP:          c.ClientIP(),
		Detail:      detail,
	})
}

func (h *Handler) writeAuditForUser(c *gin.Context, user *store.User, action, result, detail string) {
	dname := strings.TrimSpace(user.DisplayName)
	if dname == "" {
		dname = user.Name
	}
	_ = h.store.WriteAudit(&store.AuditLog{
		UserID:      user.ID,
		Username:    user.Name,
		DisplayName: dname,
		Action:      action,
		Result:      result,
		IP:          c.ClientIP(),
		Detail:      detail,
	})
}

func parseQueryTime(raw string, endOfDay bool) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			if endOfDay && (layout == "2006-01-02") {
				t = t.Add(24*time.Hour - time.Nanosecond)
			}
			return &t, nil
		}
	}
	return nil, strconv.ErrSyntax
}
