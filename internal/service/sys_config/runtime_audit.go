package sys_config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/sensitive"
)

const AuditSensitiveFieldsConfigKey = "audit.sensitive_fields"

var loadSensitiveFieldsConfigFn = sensitive.LoadSensitiveFieldsConfig

// WarmupRuntimeConfigIfAvailable 在表存在时预热系统参数缓存和运行时配置。
func (s *SysConfigService) WarmupRuntimeConfigIfAvailable() error {
	db, err := model.GetDB()
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable("sys_config") {
		return nil
	}
	if err := s.refreshCache(false); err != nil {
		return err
	}
	ensureSysConfigCacheSyncSubscriber()
	return nil
}

func applyRuntimeConfig(configs []model.SysConfig) error {
	config, err := resolveAuditSensitiveFieldsConfig(configs)
	if err != nil {
		return err
	}
	loadSensitiveFieldsConfigFn(config)
	return nil
}

func resolveAuditSensitiveFieldsConfig(configs []model.SysConfig) (sensitive.SensitiveFieldsConfig, error) {
	defaultConfig := sensitive.DefaultSensitiveFieldsConfig()
	for _, item := range configs {
		if item.ConfigKey != AuditSensitiveFieldsConfigKey {
			continue
		}
		if model.NormalizeValueType(item.ValueType) != model.SysConfigValueTypeJSON {
			return defaultConfig, fmt.Errorf("%s value_type must be json", AuditSensitiveFieldsConfigKey)
		}
		raw := strings.TrimSpace(item.ConfigValue)
		if raw == "" {
			return defaultConfig, nil
		}

		var runtimeConfig sensitive.SensitiveFieldsConfig
		if err := json.Unmarshal([]byte(raw), &runtimeConfig); err != nil {
			return defaultConfig, fmt.Errorf("decode %s failed: %w", AuditSensitiveFieldsConfigKey, err)
		}
		return normalizeSensitiveFieldsConfig(runtimeConfig), nil
	}
	return defaultConfig, nil
}

func normalizeSensitiveFieldsConfig(config sensitive.SensitiveFieldsConfig) sensitive.SensitiveFieldsConfig {
	return sensitive.SensitiveFieldsConfig{
		Common:         normalizeStringList(config.Common),
		RequestHeader:  normalizeStringList(config.RequestHeader),
		RequestBody:    normalizeStringList(config.RequestBody),
		ResponseHeader: normalizeStringList(config.ResponseHeader),
		ResponseBody:   normalizeStringList(config.ResponseBody),
	}
}

func normalizeStringList(input []string) []string {
	if len(input) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, item := range input {
		trimmed := strings.ToLower(strings.TrimSpace(item))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
