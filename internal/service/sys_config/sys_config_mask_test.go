package sys_config

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
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
