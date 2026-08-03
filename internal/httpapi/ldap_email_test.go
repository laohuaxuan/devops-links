package httpapi

import "testing"

func TestResolveLDAPUserEmail(t *testing.T) {
	tests := []struct {
		username string
		fromLDAP string
		want     string
	}{
		{"laohuaxuan", "user@company.com", "user@company.com"},
		{"laohuaxuan", "  User@Company.COM  ", "user@company.com"},
		{"laohuaxuan", "", "laohuaxuan@local.dev"},
		{"Laohuaxuan", "", "laohuaxuan@local.dev"},
	}
	for _, tc := range tests {
		got := resolveLDAPUserEmail(tc.username, tc.fromLDAP)
		if got != tc.want {
			t.Fatalf("resolveLDAPUserEmail(%q, %q) = %q, want %q", tc.username, tc.fromLDAP, got, tc.want)
		}
	}
}

func TestResolveLDAPUserEmailOverwritesPlatformEmail(t *testing.T) {
	current := normalizeEmail("admin-set@example.com")
	fromLDAP := resolveLDAPUserEmail("laohuaxuan", "")
	if current == fromLDAP {
		t.Fatal("platform-managed email should be replaced on ldap sync")
	}
	if fromLDAP != "laohuaxuan@local.dev" {
		t.Fatalf("missing ldap mail should fall back to @local.dev, got %q", fromLDAP)
	}

	withMail := resolveLDAPUserEmail("laohuaxuan", "real@company.com")
	if withMail != "real@company.com" {
		t.Fatalf("ldap mail should win over platform email, got %q", withMail)
	}
}
