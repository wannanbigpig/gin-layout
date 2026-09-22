package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/wannanbigpig/gin-layout/config"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/sms"
	cryptox "github.com/wannanbigpig/gin-layout/pkg/utils/crypto"
)

func TestBuildSMSTestDispatcherRequiresEnabledDraftConfig(t *testing.T) {
	_, err := buildSMSTestDispatcher(sms.DefaultConfig())
	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) || businessErr.GetCode() != e.InvalidParameter {
		t.Fatalf("expected InvalidParameter business error, got %T %v", err, err)
	}
	if !strings.Contains(businessErr.GetMessage(), "当前测试配置") {
		t.Fatalf("expected draft-config contract in client message, got %q", businessErr.GetMessage())
	}
}

func TestSMSTestDraftCanReuseStoredSecretWithoutMutatingStoredConfig(t *testing.T) {
	stored := sms.DefaultConfig()
	draft := sms.DefaultConfig()
	setTestAliyunSecret(t, &stored, "stored-secret")
	setTestAliyunSecret(t, &draft, sms.MaskPlaceholder)
	if err := mergeSMSProviderSecrets(&draft, stored, sms.DefaultRegistry()); err != nil {
		t.Fatalf("merge draft secret: %v", err)
	}
	if got := testSMSProviderSecret(t, draft, sms.ProviderAliyun, "access_key_secret"); got != "stored-secret" {
		t.Fatalf("draft did not reuse stored secret: %q", got)
	}
	if got := testSMSProviderSecret(t, stored, sms.ProviderAliyun, "access_key_secret"); got != "stored-secret" {
		t.Fatalf("stored config was mutated: %q", got)
	}
}

func TestSMSTestDraftKeepsNewSecretWithoutMutatingStoredConfig(t *testing.T) {
	stored := sms.DefaultConfig()
	draft := sms.DefaultConfig()
	setTestAliyunSecret(t, &stored, "stored-secret")
	setTestAliyunSecret(t, &draft, "draft-only-secret")
	if err := mergeSMSProviderSecrets(&draft, stored, sms.DefaultRegistry()); err != nil {
		t.Fatalf("merge draft secret: %v", err)
	}
	if got := testSMSProviderSecret(t, draft, sms.ProviderAliyun, "access_key_secret"); got != "draft-only-secret" {
		t.Fatalf("new draft secret was replaced: %q", got)
	}
	if got := testSMSProviderSecret(t, stored, sms.ProviderAliyun, "access_key_secret"); got != "stored-secret" {
		t.Fatalf("stored config was mutated: %q", got)
	}
}

func TestMergeAliyunSecretKeepsExplicitEmptyValue(t *testing.T) {
	stored := sms.DefaultConfig()
	draft := sms.DefaultConfig()
	setTestAliyunSecret(t, &stored, "stored-secret")
	setTestAliyunSecret(t, &draft, "")
	if err := mergeSMSProviderSecrets(&draft, stored, sms.DefaultRegistry()); err != nil {
		t.Fatalf("merge draft secret: %v", err)
	}
	if got := testSMSProviderSecret(t, draft, sms.ProviderAliyun, "access_key_secret"); got != "" {
		t.Fatalf("explicitly cleared secret was restored: %q", got)
	}
	if got := testSMSProviderSecret(t, stored, sms.ProviderAliyun, "access_key_secret"); got != "stored-secret" {
		t.Fatalf("stored config was mutated: %q", got)
	}
}

func TestSMSCredentialSecretKeyMissingIsDependencyNotReady(t *testing.T) {
	restore := config.UpdateConfigForTesting(func(cfg *config.Conf) { cfg.Security.CredentialEncryptionKey = "" })
	t.Cleanup(restore)
	_, err := smsCredentialSecretKey()
	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) || businessErr.GetCode() != e.ServiceDependencyNotReady {
		t.Fatalf("expected ServiceDependencyNotReady, got %T %v", err, err)
	}
	if !strings.Contains(businessErr.GetMessage(), "security.credential_encryption_key") || strings.Contains(businessErr.GetMessage(), "provider-secret") {
		t.Fatalf("unexpected deployment error message %q", businessErr.GetMessage())
	}
}

func TestSMSTestConfigErrorReturnsSafeActionableValidation(t *testing.T) {
	settings := sms.DefaultConfig()
	settings.Enabled = true
	secret := "provider-secret-must-not-leak"
	settings.Providers[sms.ProviderAliyun] = json.RawMessage(`{"enabled":false,"access_key_id":"id","access_key_secret":"` + secret + `"}`)
	_, err := buildSMSTestDispatcher(settings)
	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) || businessErr.GetCode() != e.InvalidParameter {
		t.Fatalf("expected InvalidParameter business error, got %T %v", err, err)
	}
	if strings.Contains(businessErr.GetMessage(), secret) {
		t.Fatalf("client message leaked provider secret: %q", businessErr.GetMessage())
	}
	if !strings.Contains(businessErr.GetMessage(), "阿里云短信供应商未启用") {
		t.Fatalf("expected actionable provider validation, got %q", businessErr.GetMessage())
	}
}

func TestSMSProviderFailureClientMessageDoesNotExposeRawResponse(t *testing.T) {
	rawProviderError := errors.New("provider raw response contains access_key_secret=should-not-leak")
	businessErr := e.NewBusinessError(e.SMSProviderFailed)
	businessErr.AppendContextErr(rawProviderError)
	if strings.Contains(businessErr.GetMessage(), "access_key_secret") || strings.Contains(businessErr.GetMessage(), "should-not-leak") {
		t.Fatalf("client message leaked provider response: %q", businessErr.GetMessage())
	}
}

func TestMergeAliyunSecretKeepsStoredValueForMask(t *testing.T) {
	current := sms.DefaultConfig()
	next := sms.DefaultConfig()
	setTestAliyunSecret(t, &current, "stored-secret")
	setTestAliyunSecret(t, &next, sms.MaskPlaceholder)
	if err := mergeSMSProviderSecrets(&next, current, sms.DefaultRegistry()); err != nil {
		t.Fatalf("merge secret: %v", err)
	}
	if got := testSMSProviderSecret(t, next, sms.ProviderAliyun, "access_key_secret"); got != "stored-secret" {
		t.Fatalf("expected stored secret, got %q", got)
	}
}

func TestDecryptStoredSMSSecretRejectsPlaintextAndUsesDedicatedKey(t *testing.T) {
	restore := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Jwt.SecretKey = "jwt-secret-must-not-be-used"
		cfg.Security.CredentialEncryptionKey = "dedicated-credential-secret-for-test"
	})
	t.Cleanup(restore)

	if _, err := decryptStoredSMSSecret(sms.ProviderAliyun, "access_key_secret", "plaintext-secret"); err == nil {
		t.Fatal("expected plaintext stored credential to be rejected")
	}
	key, err := smsCredentialSecretKey()
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := cryptox.Encrypt(key, "provider-secret")
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := decryptStoredSMSSecret(sms.ProviderAliyun, "access_key_secret", smsEncryptedPrefix+encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "provider-secret" {
		t.Fatalf("unexpected decrypted value %q", decrypted)
	}
	if strings.Contains(key, "jwt-secret") {
		t.Fatal("SMS credential key must not derive from JWT secret")
	}
}

func TestSMSSecretEncryptedAtRestDoesNotContainPlaintext(t *testing.T) {
	restore := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Security.CredentialEncryptionKey = "sms-storage-master-key-for-tests-32"
	})
	t.Cleanup(restore)

	settings := sms.DefaultConfig()
	setTestAliyunSecret(t, &settings, "sms-plaintext-secret")
	if err := encryptSMSSecrets(&settings); err != nil {
		t.Fatalf("encrypt SMS secret: %v", err)
	}
	payload, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "sms-plaintext-secret") {
		t.Fatalf("SMS payload contains plaintext secret: %s", payload)
	}
	encrypted := testSMSProviderSecret(t, settings, sms.ProviderAliyun, "access_key_secret")
	if !strings.HasPrefix(encrypted, smsEncryptedPrefix) {
		t.Fatalf("SMS secret is missing encryption prefix: %q", encrypted)
	}
	decrypted, err := decryptStoredSMSSecret(sms.ProviderAliyun, "access_key_secret", encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "sms-plaintext-secret" {
		t.Fatalf("unexpected decrypted SMS secret %q", decrypted)
	}
}

func TestCustomProviderSensitiveFieldsUseGenericLifecycle(t *testing.T) {
	restore := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Security.CredentialEncryptionKey = "custom-provider-master-key-for-tests"
	})
	t.Cleanup(restore)

	registry := sms.NewRegistry()
	if err := registry.Register(sms.ProviderDefinition{
		Name:            "custom",
		SensitiveFields: []string{"api_token"},
		Factory: func(json.RawMessage) (sms.Provider, error) {
			return nil, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	settings := sms.Config{Providers: map[string]json.RawMessage{
		"custom": json.RawMessage(`{"enabled":true,"api_token":"custom-plaintext-token","region":"cn-test"}`),
	}}
	if err := encryptSMSSecrets(&settings, registry); err != nil {
		t.Fatalf("encrypt custom provider secret: %v", err)
	}
	payload, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "custom-plaintext-token") {
		t.Fatalf("custom provider secret leaked in persisted payload: %s", payload)
	}
	if err := transformSMSProviderSecrets(&settings, registry, decryptStoredSMSSecret); err != nil {
		t.Fatalf("decrypt custom provider secret: %v", err)
	}
	if got := testSMSProviderSecret(t, settings, "custom", "api_token"); got != "custom-plaintext-token" {
		t.Fatalf("unexpected decrypted custom secret %q", got)
	}
	current := settings
	encodedSettings, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	var next sms.Config
	if err := json.Unmarshal(encodedSettings, &next); err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(next.Providers["custom"], &object); err != nil {
		t.Fatal(err)
	}
	object["api_token"] = json.RawMessage(`"********"`)
	next.Providers["custom"], _ = json.Marshal(object)
	if err := mergeSMSProviderSecrets(&next, current, registry); err != nil {
		t.Fatal(err)
	}
	if got := testSMSProviderSecret(t, next, "custom", "api_token"); got != "custom-plaintext-token" {
		t.Fatalf("masked custom secret did not reuse stored value: %q", got)
	}
}

func TestClearAliyunSecretRemovesValueFromAuditCopy(t *testing.T) {
	settings := sms.DefaultConfig()
	setTestAliyunSecret(t, &settings, "must-not-leak")
	if err := clearSMSProviderSecrets(&settings, sms.DefaultRegistry()); err != nil {
		t.Fatal(err)
	}
	if got := testSMSProviderSecret(t, settings, sms.ProviderAliyun, "access_key_secret"); got != "" {
		t.Fatalf("expected cleared secret, got %q", got)
	}
}

func setTestAliyunSecret(t *testing.T, settings *sms.Config, secret string) {
	t.Helper()
	var cfg sms.AliyunConfig
	if err := json.Unmarshal(settings.Providers[sms.ProviderAliyun], &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.AccessKeySecret = secret
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	settings.Providers[sms.ProviderAliyun] = raw
}

func testSMSProviderSecret(t *testing.T, settings sms.Config, provider, field string) string {
	t.Helper()
	value, err := smsProviderSecretValue(settings, provider, field)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
