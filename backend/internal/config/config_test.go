package config

import (
	"strings"
	"testing"
)

func TestLoadRejectsImplicitOrPublicCredentials(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://fixture@127.0.0.1/fixture?sslmode=disable")
	t.Setenv("BOOTSTRAP_ADMIN_USERNAME", "")
	t.Setenv("BOOTSTRAP_ADMIN_EMAIL", "")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "")
	for _, key := range []string{"", "short", "agentrix-secret-jwt-key-super-secure-change-in-prod", "agentrix-enterprise-jwt-secret-replace-in-production"} {
		t.Setenv("JWT_SECRET", key)
		if _, err := Load(); err == nil {
			t.Fatal("accepted missing, short or public JWT secret")
		}
	}
	t.Setenv("JWT_SECRET", strings.Repeat("x", 64))
	t.Setenv("AGENTRIX_RUNTIME_SHA256", strings.Repeat("a", 64))
	cfg, err := Load()
	if err != nil || cfg.BootstrapAdminUsername != "" {
		t.Fatalf("bootstrap must be absent by default: %v", err)
	}
	t.Setenv("BOOTSTRAP_ADMIN_USERNAME", "judge")
	if _, err := Load(); err == nil {
		t.Fatal("accepted partial bootstrap")
	}
	t.Setenv("BOOTSTRAP_ADMIN_EMAIL", "judge@example.test")
	for _, password := range []string{"admin123", strings.Repeat("a", 73)} {
		t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", password)
		if _, err := Load(); err == nil {
			t.Fatal("accepted weak or bcrypt-truncated bootstrap password")
		}
	}
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "private-test-password-2026")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
}
