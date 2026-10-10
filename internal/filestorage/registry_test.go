package filestorage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/sensitive"
)

func TestRegistrySensitiveFieldsRemainMaskedAfterEmptyAuditConfig(t *testing.T) {
	previous := sensitive.GetSensitiveFieldsConfig()
	t.Cleanup(func() { sensitive.LoadSensitiveFieldsConfig(previous) })
	const fieldName = "storage_fixture_material"
	const credential = "StorageCredentialFixtureOnly"
	definition := testDriverDefinition("private_fixture")
	definition.SensitiveFields = []SensitiveField{{
		Name: fieldName,
		Get:  func(config Config) string { return config.ProviderString("private_fixture", fieldName) },
		Set:  func(config *Config, value string) { config.SetProviderString("private_fixture", fieldName, value) },
	}}
	registry := NewRegistry()
	if err := registry.Register(definition); err != nil {
		t.Fatal(err)
	}
	registered, ok := registry.Definition("private_fixture")
	if !ok {
		t.Fatal("expected registered storage driver")
	}
	config := DefaultConfig()
	registered.SensitiveFields[0].Set(&config, credential)
	if registered.SensitiveFields[0].Get(config) != credential {
		t.Fatal("registration changed provider credential accessors")
	}
	payload, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	sensitive.LoadSensitiveFieldsConfig(sensitive.SensitiveFieldsConfig{})
	for scope, output := range map[string]string{
		"request":     sensitive.GetMaskedRequestBody(payload, "application/json"),
		"response":    sensitive.GetMaskedResponseBody(payload),
		"change_diff": auditdiff.Marshal(auditdiff.BuildFieldDiff(nil, map[string]any{fieldName: credential}, []auditdiff.FieldRule{{Field: fieldName}})),
	} {
		if strings.Contains(output, credential) {
			t.Errorf("%s exposed registered storage credential", scope)
		}
	}
}

func TestRegistryRejectsDuplicateDriver(t *testing.T) {
	registry := NewRegistry()
	definition := testDriverDefinition("custom")
	if err := registry.Register(definition); err != nil {
		t.Fatalf("register driver: %v", err)
	}
	if err := registry.Register(definition); err == nil {
		t.Fatal("expected duplicate registration error")
	}
}

func TestRegistryRejectsInvalidExtensionDefinitions(t *testing.T) {
	tests := []struct {
		name       string
		definition DriverDefinition
	}{
		{name: "invalid name", definition: testDriverDefinition("Custom Driver")},
		{name: "name too long", definition: testDriverDefinition(strings.Repeat("a", 21))},
		{name: "incomplete sensitive field", definition: func() DriverDefinition {
			definition := testDriverDefinition("custom")
			definition.SensitiveFields = []SensitiveField{{Name: "token"}}
			return definition
		}()},
		{name: "duplicate sensitive field", definition: func() DriverDefinition {
			definition := testDriverDefinition("custom")
			field := SensitiveField{Name: "token", Get: func(Config) string { return "" }, Set: func(*Config, string) {}}
			definition.SensitiveFields = []SensitiveField{field, field}
			return definition
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := NewRegistry().Register(tt.definition); err == nil {
				t.Fatal("expected invalid extension definition to be rejected")
			}
		})
	}
}

func TestRegistryBuildValidateAndResolveBucket(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(testDriverDefinition("custom")); err != nil {
		t.Fatal(err)
	}
	config := DefaultConfig()
	config.Local.BasePath = "configured"
	driver, err := registry.Build(context.Background(), "custom", config)
	if err != nil {
		t.Fatal(err)
	}
	if driver.Name() != "custom" {
		t.Fatalf("unexpected driver name %q", driver.Name())
	}
	bucket, err := registry.ResolveBucket("custom", config, true)
	if err != nil || bucket != "public-custom" {
		t.Fatalf("unexpected bucket %q, err=%v", bucket, err)
	}
	if _, err := registry.Build(context.Background(), "missing", config); err == nil {
		t.Fatal("expected unknown driver error")
	}
}

func TestDefaultRegistryRejectsIncompleteActiveAliyunConfig(t *testing.T) {
	if err := DefaultRegistry().Validate(DriverAliyunOSS, DefaultConfig()); err == nil {
		t.Fatal("expected incomplete Aliyun OSS configuration to be rejected")
	}
	config := DefaultConfig()
	config.AliyunOSS = AliyunOSSConfig{
		Endpoint: "https://oss-cn-hangzhou.aliyuncs.com", Region: "cn-hangzhou", Bucket: "assets", AccessKeyID: "id", AccessKeySecret: "secret",
	}
	if err := DefaultRegistry().Validate(DriverAliyunOSS, config); err != nil {
		t.Fatalf("valid Aliyun OSS configuration rejected: %v", err)
	}
}

func TestCustomProviderConfigRoundTripsAndBuildsWithoutChangingConfigStruct(t *testing.T) {
	type customS3Config struct {
		Endpoint  string `json:"endpoint"`
		SecretKey string `json:"secret_key"`
	}
	config := DefaultConfig()
	if err := config.SetProviderConfig("custom_s3", customS3Config{Endpoint: "https://s3.example.com", SecretKey: "plain-secret"}); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"custom_s3"`) {
		t.Fatalf("custom provider config was not serialized at the config root: %s", payload)
	}
	var decoded Config
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	var built customS3Config
	registry := NewRegistry()
	err = registry.Register(DriverDefinition{
		Name: "custom_s3",
		Build: func(_ context.Context, config Config) (Driver, error) {
			if err := config.DecodeProviderConfig("custom_s3", &built); err != nil {
				return nil, err
			}
			return registryTestDriver{name: "custom_s3"}, nil
		},
		Validate: func(config Config) error {
			var provider customS3Config
			if err := config.DecodeProviderConfig("custom_s3", &provider); err != nil {
				return err
			}
			if provider.Endpoint == "" || provider.SecretKey == "" {
				return errors.New("custom S3 config is incomplete")
			}
			return nil
		},
		ResolveBucket: func(Config, bool) string { return "custom-bucket" },
		SensitiveFields: []SensitiveField{{
			Name: "secret_key",
			Get:  func(config Config) string { return config.ProviderString("custom_s3", "secret_key") },
			Set:  func(config *Config, value string) { config.SetProviderString("custom_s3", "secret_key", value) },
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Build(context.Background(), "custom_s3", decoded); err != nil {
		t.Fatal(err)
	}
	if built.Endpoint != "https://s3.example.com" || built.SecretKey != "plain-secret" {
		t.Fatalf("custom provider received unexpected config: %#v", built)
	}
	decoded.SetProviderString("custom_s3", "secret_key", "changed-secret")
	if decoded.ProviderString("custom_s3", "secret_key") != "changed-secret" {
		t.Fatal("custom sensitive field accessor did not update provider config")
	}
}

func testDriverDefinition(name string) DriverDefinition {
	return DriverDefinition{
		Name: name,
		Build: func(context.Context, Config) (Driver, error) {
			return registryTestDriver{name: name}, nil
		},
		Validate: func(config Config) error {
			if strings.TrimSpace(config.Local.BasePath) == "" {
				return errors.New("missing config")
			}
			return nil
		},
		ResolveBucket: func(_ Config, public bool) string {
			if public {
				return "public-custom"
			}
			return "private-custom"
		},
	}
}

type registryTestDriver struct{ name string }

func (d registryTestDriver) Name() string { return d.name }
func (registryTestDriver) Put(context.Context, PutInput) (PutResult, error) {
	return PutResult{}, nil
}
func (registryTestDriver) Open(context.Context, string, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (registryTestDriver) Exists(context.Context, string, string) (bool, error) { return true, nil }
func (registryTestDriver) Delete(context.Context, string, string) error         { return nil }
func (registryTestDriver) URL(string, string, bool) string                      { return "" }
func (registryTestDriver) SignedURL(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}
