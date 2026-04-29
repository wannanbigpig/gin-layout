package sys_config

import (
	"strings"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestMaskSensitiveValues(t *testing.T) {
	service := NewSysConfigService()
	configs := []*model.SysConfig{
		{
			ConfigKey:   "system.site_name",
			ConfigValue: "gin-layout",
			IsSensitive: 0,
		},
		{
			ConfigKey:   "audit.sensitive_fields",
			ConfigValue: `{"common":["password"]}`,
			IsSensitive: 1,
		},
	}

	service.maskSensitiveValues(configs)

	if configs[0].ConfigValue != "gin-layout" {
		t.Fatalf("expected non-sensitive value unchanged, got %q", configs[0].ConfigValue)
	}
	if configs[1].ConfigValue != maskedConfigValue {
		t.Fatalf("expected sensitive value masked, got %q", configs[1].ConfigValue)
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
