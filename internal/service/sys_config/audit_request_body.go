package sys_config

import (
	"encoding/json"
	"strings"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// MaskedAuditRequestBody builds a request-body snapshot for request logs.
// It only overrides the generic body mask when the target config is sensitive.
func (s *SysConfigService) MaskedAuditRequestBody(id uint, params *form.SysConfigPayload) string {
	if params == nil {
		return ""
	}
	if !s.shouldMaskConfigMutationValue(id, params) {
		return ""
	}

	payload := map[string]any{
		"config_key":       strings.TrimSpace(params.ConfigKey),
		"config_name_i18n": params.ConfigNameI18n,
		"config_value":     maskedConfigValue,
		"value_type":       model.NormalizeValueType(params.ValueType),
		"group_code":       strings.TrimSpace(params.GroupCode),
		"is_sensitive":     params.IsSensitive,
		"status":           params.Status,
		"sort":             params.Sort,
		"remark":           strings.TrimSpace(params.Remark),
	}
	if id > 0 {
		payload["id"] = id
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}

func (s *SysConfigService) shouldMaskConfigMutationValue(id uint, params *form.SysConfigPayload) bool {
	if params != nil && params.IsSensitive != nil && *params.IsSensitive == 1 {
		return true
	}
	if id == 0 {
		return false
	}

	config := model.NewSysConfig()
	if err := config.GetById(id); err != nil || config.ID == 0 {
		return false
	}
	return config.IsSensitive == 1
}
