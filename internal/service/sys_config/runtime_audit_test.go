package sys_config

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/sensitive"
)

func TestResolveAuditSensitiveFieldsConfigReturnsDefaultWhenMissing(t *testing.T) {
	got, err := resolveAuditSensitiveFieldsConfig(nil)
	if err != nil {
		t.Fatalf("resolveAuditSensitiveFieldsConfig returned error: %v", err)
	}
	defaultConfig := sensitive.DefaultSensitiveFieldsConfig()
	if len(got.Common) != len(defaultConfig.Common) {
		t.Fatalf("expected default common fields, got %#v", got.Common)
	}
}

func TestResolveAuditSensitiveFieldsConfigNormalizesValues(t *testing.T) {
	got, err := resolveAuditSensitiveFieldsConfig([]model.SysConfig{
		{
			ConfigKey:   AuditSensitiveFieldsConfigKey,
			ValueType:   model.SysConfigValueTypeJSON,
			ConfigValue: `{"common":[" Password ","token","password"],"request_body":[" Phone ","phone"],"response_header":[" Set-Cookie "]}`,
		},
	})
	if err != nil {
		t.Fatalf("resolveAuditSensitiveFieldsConfig returned error: %v", err)
	}

	assertStringSliceEqual(t, got.Common, []string{"password", "token"})
	assertStringSliceEqual(t, got.RequestBody, []string{"phone"})
	assertStringSliceEqual(t, got.ResponseHeader, []string{"set-cookie"})
}

func TestResolveAuditSensitiveFieldsConfigRejectsNonJSONValueType(t *testing.T) {
	_, err := resolveAuditSensitiveFieldsConfig([]model.SysConfig{
		{
			ConfigKey:   AuditSensitiveFieldsConfigKey,
			ValueType:   model.SysConfigValueTypeString,
			ConfigValue: `{"common":["password"]}`,
		},
	})
	if err == nil {
		t.Fatal("expected non-json audit config to fail")
	}
}

func TestApplyRuntimeConfigLoadsSensitiveManager(t *testing.T) {
	previous := loadSensitiveFieldsConfigFn
	t.Cleanup(func() {
		loadSensitiveFieldsConfigFn = previous
	})

	var captured sensitive.SensitiveFieldsConfig
	loadSensitiveFieldsConfigFn = func(config sensitive.SensitiveFieldsConfig) {
		captured = config
	}

	err := applyRuntimeConfig([]model.SysConfig{
		{
			ConfigKey:   AuditSensitiveFieldsConfigKey,
			ValueType:   model.SysConfigValueTypeJSON,
			ConfigValue: `{"common":["password"],"request_header":["authorization"]}`,
		},
	})
	if err != nil {
		t.Fatalf("applyRuntimeConfig returned error: %v", err)
	}

	assertStringSliceEqual(t, captured.Common, []string{"password"})
	assertStringSliceEqual(t, captured.RequestHeader, []string{"authorization"})
}

func assertStringSliceEqual(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unexpected slice length: got=%#v want=%#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected slice content: got=%#v want=%#v", got, want)
		}
	}
}
