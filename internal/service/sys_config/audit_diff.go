package sys_config

import (
	"strings"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

var sysConfigDiffRules = []auditdiff.FieldRule{
	{Field: "config_key", Label: "参数键名"},
	{Field: "config_name_i18n", Label: "参数名称"},
	{Field: "config_value", Label: "参数值"},
	{
		Field: "value_type",
		Label: "值类型",
		ValueLabels: map[string]string{
			model.SysConfigValueTypeString: "字符串",
			model.SysConfigValueTypeNumber: "数字",
			model.SysConfigValueTypeBool:   "布尔",
			model.SysConfigValueTypeJSON:   "JSON",
		},
	},
	{Field: "group_code", Label: "参数分组"},
	{
		Field: "is_sensitive",
		Label: "敏感参数",
		ValueLabels: map[string]string{
			"0": "否",
			"1": "是",
		},
	},
	{
		Field: "status",
		Label: "状态",
		ValueLabels: map[string]string{
			"0": "禁用",
			"1": "启用",
		},
	},
	{Field: "sort", Label: "排序"},
	{Field: "remark", Label: "备注"},
}

// CreateWithAuditDiff 创建系统参数并返回字段级 change_diff JSON。
func (s *SysConfigService) CreateWithAuditDiff(params *form.CreateSysConfig) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	if err := s.applyMutation(0, &params.SysConfigPayload); err != nil {
		return "", err
	}
	after, err := s.snapshotConfigByKey(params.ConfigKey)
	if err != nil {
		return "", nil
	}
	return buildSysConfigDiffJSON(nil, after), nil
}

// UpdateWithAuditDiff 更新系统参数并返回字段级 change_diff JSON。
func (s *SysConfigService) UpdateWithAuditDiff(params *form.UpdateSysConfig) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	before, err := s.snapshotConfigByID(params.Id)
	if err != nil {
		return "", err
	}
	if err := s.applyMutation(params.Id, &params.SysConfigPayload); err != nil {
		return "", err
	}
	after, err := s.snapshotConfigByID(params.Id)
	if err != nil {
		return "", nil
	}
	return buildSysConfigDiffJSON(before, after), nil
}

// DeleteWithAuditDiff 删除系统参数并返回字段级 change_diff JSON。
func (s *SysConfigService) DeleteWithAuditDiff(id uint) (string, error) {
	before, err := s.snapshotConfigByID(id)
	if err != nil {
		return "", err
	}
	if err := s.deleteConfig(id); err != nil {
		return "", err
	}
	return buildSysConfigDiffJSON(before, nil), nil
}

func (s *SysConfigService) deleteConfig(id uint) error {
	config := model.NewSysConfig()
	if err := config.GetById(id); err != nil || config.ID == 0 {
		return e.NewBusinessError(e.NotFound)
	}
	if config.IsProtected() {
		return e.NewBusinessError(e.InvalidParameter)
	}
	db, err := config.GetDB()
	if err != nil {
		return err
	}
	if err := access.RunInTransaction(db, func(tx *gorm.DB) error {
		config.SetDB(tx)
		if _, deleteErr := config.DeleteByID(id); deleteErr != nil {
			return deleteErr
		}
		return model.NewSysConfigI18n().DeleteByConfigIDs([]uint{id}, tx)
	}); err != nil {
		return err
	}
	return s.RefreshCache()
}

func (s *SysConfigService) snapshotConfigByID(id uint) (map[string]any, error) {
	config := model.NewSysConfig()
	if err := config.GetById(id); err != nil || config.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	return snapshotConfig(config)
}

func (s *SysConfigService) snapshotConfigByKey(key string) (map[string]any, error) {
	config := model.NewSysConfig()
	if err := config.FindByKey(strings.TrimSpace(key)); err != nil {
		return nil, err
	}
	return snapshotConfig(config)
}

func snapshotConfig(config *model.SysConfig) (map[string]any, error) {
	if config == nil || config.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	nameI18n, err := model.NewSysConfigI18n().LocaleNameMapByConfigID(config.ID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"config_key":       config.ConfigKey,
		"config_name_i18n": nameI18n,
		"config_value":     config.ConfigValue,
		"value_type":       model.NormalizeValueType(config.ValueType),
		"group_code":       config.GroupCode,
		"is_sensitive":     config.IsSensitive,
		"status":           config.Status,
		"sort":             config.Sort,
		"remark":           config.Remark,
	}, nil
}

func buildSysConfigDiffJSON(before, after map[string]any) string {
	return auditdiff.Marshal(auditdiff.BuildFieldDiff(before, after, sysConfigDiffRules))
}
