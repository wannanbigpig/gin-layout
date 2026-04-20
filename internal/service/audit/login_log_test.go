package audit

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/config/autoload"
	"github.com/wannanbigpig/gin-layout/internal/pkg/testkit"
	"github.com/wannanbigpig/gin-layout/pkg/utils/crypto"
)

func TestDecryptLoginTokenIfNeeded(t *testing.T) {
	key := testkit.SecretKey("audit-login-log")
	const plain = "header.payload.signature"

	encrypted, err := crypto.Encrypt(key, plain)
	if err != nil {
		t.Fatalf("encrypt token failed: %v", err)
	}

	if got := decryptLoginTokenIfNeeded(encrypted, key); got != plain {
		t.Fatalf("expected decrypted token %q, got %q", plain, got)
	}
}

func TestDecryptLoginTokenIfNeededFallbackOnDecryptError(t *testing.T) {
	key := testkit.SecretKey("audit-login-log")
	const raw = "not-encrypted-token"

	if got := decryptLoginTokenIfNeeded(raw, key); got != raw {
		t.Fatalf("expected fallback raw token %q, got %q", raw, got)
	}
}

func TestCurrentAuditConfigUsesInjectedProvider(t *testing.T) {
	service := NewAdminLoginLogServiceWithDeps(AdminLoginLogServiceDeps{
		ConfigProvider: func() *config.Conf {
			return &config.Conf{
				Jwt: autoload.JwtConfig{
					SecretKey: "audit-key",
				},
			}
		},
	})

	if got := service.currentConfig().Jwt.SecretKey; got != "audit-key" {
		t.Fatalf("expected injected key %q, got %q", "audit-key", got)
	}
}
