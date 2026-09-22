package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/filestorage"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestStorageSettingsAuditDiffRedactsChangedSecrets(t *testing.T) {
	before := StorageSettings{ActiveDriver: "local"}
	before.Config.AliyunOSS.AccessKeySecret = "old-private-key"
	after := before
	after.ActiveDriver = "aliyun_oss"
	after.Config.AliyunOSS.AccessKeySecret = "new-private-key"
	raw := storageSettingsAuditDiff(before, after)
	if strings.Contains(raw, "old-private-key") || strings.Contains(raw, "new-private-key") {
		t.Fatalf("secret leaked: %s", raw)
	}
	if !strings.Contains(raw, "access_key_secret") || !strings.Contains(raw, "active_driver") {
		t.Fatalf("missing changed fields: %s", raw)
	}
	if raw := storageSettingsAuditDiff(before, before); raw != "[]" {
		t.Fatalf("unchanged settings produce diff: %s", raw)
	}
}

func TestStorageSettingsDoNotFallBackOnDatabaseFailure(t *testing.T) {
	_, err := NewStorageConfigService().Get(false)
	if !errors.Is(err, model.ErrDBUninitialized) {
		t.Fatalf("expected database error, got %v", err)
	}
}

func TestStorageSecretEncryptedAtRestAndDecryptsForRuntime(t *testing.T) {
	restore := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Security.CredentialEncryptionKey = "storage-test-master-key-for-tests-32"
	})
	t.Cleanup(restore)

	storageConfig := filestorage.DefaultConfig()
	storageConfig.AliyunOSS.AccessKeySecret = "oss-plaintext-secret"
	if err := encryptStorageSecret(&storageConfig); err != nil {
		t.Fatalf("encrypt storage secret: %v", err)
	}
	payload, err := json.Marshal(storageConfig)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "oss-plaintext-secret") {
		t.Fatalf("storage payload contains plaintext secret: %s", payload)
	}
	if !strings.Contains(storageConfig.AliyunOSS.AccessKeySecret, storageEncryptedPrefix) {
		t.Fatalf("storage secret is missing encryption prefix: %q", storageConfig.AliyunOSS.AccessKeySecret)
	}
	if err := decryptStorageSecret(&storageConfig); err != nil {
		t.Fatalf("decrypt storage secret: %v", err)
	}
	if storageConfig.AliyunOSS.AccessKeySecret != "oss-plaintext-secret" {
		t.Fatalf("unexpected decrypted storage secret %q", storageConfig.AliyunOSS.AccessKeySecret)
	}
}

func TestStorageSecretMaskReusesStoredPlaintextAndEmptyDraftClears(t *testing.T) {
	current := filestorage.DefaultConfig()
	current.AliyunOSS.AccessKeySecret = "stored-secret"
	next := filestorage.DefaultConfig()
	next.AliyunOSS.AccessKeySecret = filestorage.MaskPlaceholder
	mergeSensitiveStorageConfig(&next, current)
	if next.AliyunOSS.AccessKeySecret != "stored-secret" {
		t.Fatalf("masked draft did not reuse stored plaintext secret: %q", next.AliyunOSS.AccessKeySecret)
	}
	next.AliyunOSS.AccessKeySecret = ""
	mergeSensitiveStorageConfig(&next, current)
	if next.AliyunOSS.AccessKeySecret != "" {
		t.Fatalf("empty draft should clear stored plaintext secret: %q", next.AliyunOSS.AccessKeySecret)
	}

	masked := current
	maskStorageSecret(&masked)
	if masked.AliyunOSS.AccessKeySecret != filestorage.MaskPlaceholder {
		t.Fatalf("expected masked storage secret, got %q", masked.AliyunOSS.AccessKeySecret)
	}
	if current.AliyunOSS.AccessKeySecret != "stored-secret" {
		t.Fatal("masking mutated the source storage config")
	}
}

func TestStorageSecretRequiresCredentialEncryptionKey(t *testing.T) {
	restore := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Security.CredentialEncryptionKey = ""
	})
	t.Cleanup(restore)

	storageConfig := filestorage.DefaultConfig()
	storageConfig.AliyunOSS.AccessKeySecret = "oss-secret"
	err := encryptStorageSecret(&storageConfig)
	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) || businessErr.GetCode() != e.ServiceDependencyNotReady {
		t.Fatalf("expected ServiceDependencyNotReady, got %T %v", err, err)
	}
	if !strings.Contains(businessErr.GetMessage(), "security.credential_encryption_key") {
		t.Fatalf("unexpected deployment error message %q", businessErr.GetMessage())
	}
}

func TestCustomStorageProviderSensitiveFieldUsesRegisteredLifecycle(t *testing.T) {
	restore := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Security.CredentialEncryptionKey = "custom-storage-provider-key-for-tests"
	})
	t.Cleanup(restore)

	type customS3Config struct {
		Endpoint  string `json:"endpoint"`
		SecretKey string `json:"secret_key"`
	}
	registry := filestorage.NewRegistry()
	err := registry.Register(filestorage.DriverDefinition{
		Name: "custom_s3",
		Build: func(_ context.Context, config filestorage.Config) (filestorage.Driver, error) {
			var provider customS3Config
			if err := config.DecodeProviderConfig("custom_s3", &provider); err != nil {
				return nil, err
			}
			return fakeFileResourceDriver{name: "custom_s3"}, nil
		},
		Validate: func(config filestorage.Config) error {
			var provider customS3Config
			return config.DecodeProviderConfig("custom_s3", &provider)
		},
		ResolveBucket: func(filestorage.Config, bool) string { return "custom" },
		SensitiveFields: []filestorage.SensitiveField{{
			Name:  "secret_key",
			Label: "自定义存储密钥",
			Get:   func(cfg filestorage.Config) string { return cfg.ProviderString("custom_s3", "secret_key") },
			Set:   func(cfg *filestorage.Config, value string) { cfg.SetProviderString("custom_s3", "secret_key", value) },
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	storageConfig := filestorage.DefaultConfig()
	if err := storageConfig.SetProviderConfig("custom_s3", customS3Config{Endpoint: "https://s3.example.com", SecretKey: "custom-plaintext-token"}); err != nil {
		t.Fatal(err)
	}
	if err := encryptStorageSecretWithRegistry(&storageConfig, registry); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(storageConfig)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "custom-plaintext-token") {
		t.Fatalf("custom provider secret leaked in persisted payload: %s", payload)
	}
	if err := decryptStorageSecretWithRegistry(&storageConfig, registry); err != nil {
		t.Fatal(err)
	}
	if secret := storageConfig.ProviderString("custom_s3", "secret_key"); secret != "custom-plaintext-token" {
		t.Fatalf("unexpected decrypted custom secret %q", secret)
	}

	current := storageConfig
	next := storageConfig
	next.SetProviderString("custom_s3", "secret_key", filestorage.MaskPlaceholder)
	mergeSensitiveStorageConfigWithRegistry(&next, current, registry)
	if secret := next.ProviderString("custom_s3", "secret_key"); secret != "custom-plaintext-token" {
		t.Fatalf("custom provider mask did not retain the secret: %q", secret)
	}
	next.SetProviderString("custom_s3", "secret_key", "")
	mergeSensitiveStorageConfigWithRegistry(&next, current, registry)
	if secret := next.ProviderString("custom_s3", "secret_key"); secret != "" {
		t.Fatalf("custom provider empty secret was not cleared: %q", secret)
	}
}
