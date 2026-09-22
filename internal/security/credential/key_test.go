package credential

import (
	"errors"
	"testing"

	"github.com/wannanbigpig/gin-layout/config"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestDeriveKeySeparatesPurposes(t *testing.T) {
	restore := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Security.CredentialEncryptionKey = "deployment-master-key-for-tests-32"
	})
	t.Cleanup(restore)

	seen := map[string]Purpose{}
	for _, purpose := range []Purpose{PurposeSMSProvider, PurposeSMSOTP, PurposeSMSQuota, PurposeStorageAliyunOSS} {
		key, err := DeriveKey(purpose)
		if err != nil {
			t.Fatalf("derive %s: %v", purpose, err)
		}
		if key == "deployment-master-key-for-tests-32" {
			t.Fatalf("purpose %s exposed the master key", purpose)
		}
		if previous, exists := seen[key]; exists {
			t.Fatalf("purposes %s and %s derived the same key", previous, purpose)
		}
		seen[key] = purpose
	}
}

func TestDeriveKeyMissingMasterKeyIsDependencyNotReady(t *testing.T) {
	restore := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Security.CredentialEncryptionKey = ""
	})
	t.Cleanup(restore)

	_, err := DeriveKey(PurposeSMSProvider)
	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) || businessErr.GetCode() != e.ServiceDependencyNotReady {
		t.Fatalf("expected ServiceDependencyNotReady, got %T %v", err, err)
	}
}

func TestDeriveKeyRejectsShortMasterKey(t *testing.T) {
	restore := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Security.CredentialEncryptionKey = "short-key"
	})
	t.Cleanup(restore)

	_, err := DeriveKey(PurposeStorageAliyunOSS)
	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) || businessErr.GetCode() != e.ServiceDependencyNotReady {
		t.Fatalf("expected ServiceDependencyNotReady, got %T %v", err, err)
	}
}
