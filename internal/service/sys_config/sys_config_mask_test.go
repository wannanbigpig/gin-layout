package sys_config

import (
	"strings"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestMaskSensitiveValues(t *testing.T) {
	service := NewSysConfigService()
	configs := []*model.SysConfig{
		{
			ConfigKey:   AuthLoginLockEnabledConfigKey,
			ConfigValue: "true",
			IsSensitive: 0,
		},
		{
			ConfigKey:   "audit.sensitive_fields",
			ConfigValue: `{"common":["password"]}`,
			IsSensitive: 1,
		},
	}

	service.maskSensitiveValues(configs)

	if configs[0].ConfigValue != "true" {
		t.Fatalf("expected non-sensitive value unchanged, got %q", configs[0].ConfigValue)
	}
	if configs[1].ConfigValue != maskedConfigValue {
		t.Fatalf("expected sensitive value masked, got %q", configs[1].ConfigValue)
	}
}

func TestSysConfigDetailResourceKeepsSensitiveConfigValue(t *testing.T) {
	config := &model.SysConfig{
		ConfigKey:   "audit.sensitive_fields",
		ConfigValue: `{"common":["password"]}`,
		IsSensitive: 1,
	}

	detail := resources.NewSysConfigTransformer().ToStruct(config)

	if detail.ConfigValue != config.ConfigValue {
		t.Fatalf("expected detail value to remain unmasked, got %q", detail.ConfigValue)
	}
}

func TestMaskedAuditRequestBodyMasksSensitiveConfigValue(t *testing.T) {
	isSensitive := uint8(1)
	status := uint8(1)
	raw := NewSysConfigService().MaskedAuditRequestBody(0, &form.SysConfigPayload{
		ConfigKey:      "secret.demo",
		ConfigNameI18n: map[string]string{"zh-CN": "密钥"},
		ConfigValue:    "plain-secret",
		ValueType:      model.SysConfigValueTypeString,
		IsSensitive:    &isSensitive,
		Status:         &status,
	})

	if raw == "" {
		t.Fatal("expected masked audit request body")
	}
	if strings.Contains(raw, "plain-secret") {
		t.Fatalf("expected raw secret to be masked, got %s", raw)
	}
	if !strings.Contains(raw, maskedConfigValue) {
		t.Fatalf("expected masked value in audit request body, got %s", raw)
	}
}

func TestMaskedAuditRequestBodySkipsNonSensitiveConfig(t *testing.T) {
	isSensitive := uint8(0)
	raw := NewSysConfigService().MaskedAuditRequestBody(0, &form.SysConfigPayload{
		ConfigKey:   "feature.demo",
		ConfigValue: "visible",
		ValueType:   model.SysConfigValueTypeString,
		IsSensitive: &isSensitive,
	})
	if raw != "" {
		t.Fatalf("expected no override for non-sensitive config, got %s", raw)
	}
}

func TestResolveMutationConfigValueKeepsSensitiveValueForMaskedPlaceholder(t *testing.T) {
	existing := &model.SysConfig{
		ConfigValue: "real-secret",
		IsSensitive: 1,
	}
	existing.ID = 1

	got := resolveMutationConfigValue(existing, maskedConfigValue)

	if got != "real-secret" {
		t.Fatalf("expected existing sensitive value to be kept, got %q", got)
	}
}

func TestResolveMutationConfigValueUsesIncomingValueWhenChanged(t *testing.T) {
	existing := &model.SysConfig{
		ConfigValue: "real-secret",
		IsSensitive: 1,
	}
	existing.ID = 1

	got := resolveMutationConfigValue(existing, "new-secret")

	if got != "new-secret" {
		t.Fatalf("expected incoming value, got %q", got)
	}
}

func TestResolveMutationConfigValuePreservesStringWhitespace(t *testing.T) {
	got := resolveMutationConfigValue(&model.SysConfig{}, "  display value  ")

	if got != "  display value  " {
		t.Fatalf("expected incoming whitespace to be preserved, got %q", got)
	}
}
