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
	isVisible := uint8(0)
	status := uint8(1)
	raw := NewSysConfigService().MaskedAuditRequestBody(0, &form.SysConfigPayload{
		ConfigKey:      "secret.demo",
		ConfigNameI18n: map[string]string{"zh-CN": "密钥"},
		ConfigValue:    "plain-secret",
		ValueType:      model.SysConfigValueTypeString,
		IsSensitive:    &isSensitive,
		IsVisible:      &isVisible,
		ManageTab:      "audit_mask",
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
	if !strings.Contains(raw, `"is_visible":0`) {
		t.Fatalf("expected is_visible in audit request body, got %s", raw)
	}
	if !strings.Contains(raw, `"manage_tab":"audit_mask"`) {
		t.Fatalf("expected manage_tab in audit request body, got %s", raw)
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

func TestBuildSysConfigDiffIncludesVisibilityFields(t *testing.T) {
	before := map[string]any{
		"config_key":       "audit.sensitive_fields",
		"config_name_i18n": map[string]string{"zh-CN": "请求日志脱敏配置"},
		"config_value":     "before",
		"value_type":       model.SysConfigValueTypeJSON,
		"group_code":       "audit",
		"is_sensitive":     uint8(1),
		"is_visible":       uint8(1),
		"manage_tab":       "",
		"status":           uint8(1),
		"sort":             uint(95),
		"remark":           "before",
	}
	after := map[string]any{
		"config_key":       "audit.sensitive_fields",
		"config_name_i18n": map[string]string{"zh-CN": "请求日志脱敏配置"},
		"config_value":     maskedConfigValue,
		"value_type":       model.SysConfigValueTypeJSON,
		"group_code":       "audit",
		"is_sensitive":     uint8(1),
		"is_visible":       uint8(0),
		"manage_tab":       "audit_mask",
		"status":           uint8(1),
		"sort":             uint(95),
		"remark":           "before",
	}

	raw := buildSysConfigDiffJSON(before, after)
	if !strings.Contains(raw, `"field":"is_visible"`) {
		t.Fatalf("expected diff to include is_visible, got %s", raw)
	}
	if !strings.Contains(raw, `"field":"manage_tab"`) {
		t.Fatalf("expected diff to include manage_tab, got %s", raw)
	}
}
