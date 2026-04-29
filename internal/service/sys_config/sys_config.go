package sys_config

import (
	"encoding/json"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// SysConfigService 系统参数服务。
type SysConfigService struct {
	service.Base
}

const maskedConfigValue = "******"

func NewSysConfigService() *SysConfigService {
	return &SysConfigService{}
}

func (s *SysConfigService) List(params *form.SysConfigList, locale string) *resources.Collection {
	condition, args := s.buildListCondition(params)
	configModel := model.NewSysConfig()
	total, collection, err := model.ListPageE(configModel, params.Page, params.PerPage, condition, args, model.ListOptionalParams{
		OrderBy: "sort desc, id desc",
	})
	if err != nil {
		return resources.NewSysConfigTransformer().ToCollection(params.Page, params.PerPage, 0, nil)
	}
	s.maskSensitiveValues(collection)
	s.fillLocalizedNames(collection, locale)
	return resources.NewSysConfigTransformer().ToCollection(params.Page, params.PerPage, total, collection)
}

func (s *SysConfigService) Detail(id uint, locale string) (any, error) {
	config := model.NewSysConfig()
	if err := config.GetById(id); err != nil || config.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	translations, err := model.NewSysConfigI18n().LocaleNameMapByConfigID(config.ID)
	if err != nil {
		return nil, err
	}
	config.ConfigNameI18n = translations
	config.ConfigName = service.ResolveLocaleText(translations, locale)
	if config.IsSensitive == 1 {
		config.ConfigValue = maskedConfigValue
	}
	return resources.NewSysConfigTransformer().ToStruct(config), nil
}

func (s *SysConfigService) Create(params *form.CreateSysConfig) error {
	_, err := s.CreateWithAuditDiff(params)
	return err
}

func (s *SysConfigService) Update(params *form.UpdateSysConfig) error {
	_, err := s.UpdateWithAuditDiff(params)
	return err
}

func (s *SysConfigService) Delete(id uint) error {
	_, err := s.DeleteWithAuditDiff(id)
	return err
}

func (s *SysConfigService) Value(key string) (ConfigCacheItem, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return ConfigCacheItem{}, e.NewBusinessError(e.InvalidParameter)
	}
	if !cacheLoaded() {
		if err := s.RefreshCache(); err != nil {
			return ConfigCacheItem{}, err
		}
	}
	if item, ok := getCachedValue(key); ok {
		return item, nil
	}
	return ConfigCacheItem{}, e.NewBusinessError(e.NotFound)
}

// PublicValue 对外暴露的系统参数值接口，敏感参数自动脱敏。
func (s *SysConfigService) PublicValue(key string) (ConfigCacheItem, error) {
	item, err := s.Value(key)
	if err != nil {
		return item, err
	}
	if item.IsSensitive == 1 {
		item.ConfigValue = maskedConfigValue
	}
	return item, nil
}

// RefreshCache 刷新本进程参数缓存；加载失败时保留旧缓存。
func (s *SysConfigService) RefreshCache() error {
	return s.refreshCache(true)
}

func (s *SysConfigService) refreshCache(notify bool) error {
	configs, err := model.NewSysConfig().EnabledConfigs()
	if err != nil {
		return err
	}
	if err := applyRuntimeConfig(configs); err != nil {
		return err
	}
	replaceCache(configs)
	if notify {
		notifySysConfigCacheRefreshed()
	}
	return nil
}

func (s *SysConfigService) applyMutation(id uint, params *form.SysConfigPayload) error {
	params.ConfigKey = strings.TrimSpace(params.ConfigKey)
	params.ConfigNameI18n = service.NormalizeLocaleTextMap(params.ConfigNameI18n)
	params.GroupCode = strings.TrimSpace(params.GroupCode)
	params.ValueType = model.NormalizeValueType(params.ValueType)
	params.Remark = strings.TrimSpace(params.Remark)
	if params.GroupCode == "" {
		params.GroupCode = "default"
	}
	if len(params.ConfigNameI18n) == 0 {
		return e.NewBusinessError(e.InvalidParameter)
	}
	if err := validateConfigValue(params.ValueType, params.ConfigValue); err != nil {
		return err
	}

	config := model.NewSysConfig()
	if id > 0 {
		if err := config.GetById(id); err != nil || config.ID == 0 {
			return e.NewBusinessError(e.NotFound)
		}
		// 系统内置参数禁止修改稳定字段。
		if config.IsProtected() {
			if params.ConfigKey != config.ConfigKey {
				return e.NewBusinessError(e.InvalidParameter)
			}
			if model.NormalizeValueType(params.ValueType) != config.ValueType {
				return e.NewBusinessError(e.InvalidParameter)
			}
			if strings.TrimSpace(params.GroupCode) != config.GroupCode {
				return e.NewBusinessError(e.InvalidParameter)
			}
			// 敏感标记禁止降级。
			if params.IsSensitive != nil && *params.IsSensitive == 0 && config.IsSensitive == 1 {
				return e.NewBusinessError(e.InvalidParameter)
			}
		}
	}
	if exists, err := model.NewSysConfig().ExistsByKeyExcludeID(params.ConfigKey, id); err != nil {
		return err
	} else if exists {
		return e.NewBusinessError(e.InvalidParameter)
	}

	config.ConfigKey = params.ConfigKey
	config.ConfigValue = strings.TrimSpace(params.ConfigValue)
	config.ValueType = params.ValueType
	config.GroupCode = params.GroupCode
	config.IsSensitive = valueOrDefault(params.IsSensitive, config.IsSensitive)
	config.Status = valueOrDefault(params.Status, defaultStatus(config.Status, id))
	config.Sort = params.Sort
	config.Remark = params.Remark

	db, err := config.GetDB()
	if err != nil {
		return err
	}
	if err := access.RunInTransaction(db, func(tx *gorm.DB) error {
		config.SetDB(tx)
		if saveErr := config.Save(); saveErr != nil {
			return saveErr
		}
		return model.NewSysConfigI18n().UpsertConfigNames(config.ID, params.ConfigNameI18n, tx)
	}); err != nil {
		return err
	}
	return s.RefreshCache()
}

func (s *SysConfigService) buildListCondition(params *form.SysConfigList) (string, []any) {
	qb := query_builder.New().
		AddLike("config_key", params.ConfigKey).
		AddEq("group_code", params.GroupCode).
		AddEq("value_type", params.ValueType).
		AddEq("status", params.Status)
	if keyword := strings.TrimSpace(params.ConfigName); keyword != "" {
		qb.AddCondition("id IN (SELECT config_id FROM sys_config_i18n WHERE config_name like ?)", "%"+keyword+"%")
	}
	return qb.Build()
}

func (s *SysConfigService) fillLocalizedNames(configs []*model.SysConfig, locale string) {
	ids := make([]uint, 0, len(configs))
	for _, config := range configs {
		if config == nil {
			continue
		}
		ids = append(ids, config.ID)
	}
	if len(ids) == 0 {
		return
	}
	nameMap, err := model.NewSysConfigI18n().LocalizedNameMapByConfigIDs(ids, service.LocalePriority(locale))
	if err != nil {
		return
	}
	for _, config := range configs {
		if config == nil {
			continue
		}
		config.ConfigName = nameMap[config.ID]
	}
}

func (s *SysConfigService) maskSensitiveValues(configs []*model.SysConfig) {
	for _, config := range configs {
		if config == nil {
			continue
		}
		if config.IsSensitive == 1 {
			config.ConfigValue = maskedConfigValue
		}
	}
}

func validateConfigValue(valueType, value string) error {
	value = strings.TrimSpace(value)
	switch valueType {
	case model.SysConfigValueTypeString:
		return nil
	case model.SysConfigValueTypeNumber:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return e.NewBusinessError(e.InvalidParameter)
		}
	case model.SysConfigValueTypeBool:
		if _, err := strconv.ParseBool(value); err != nil {
			return e.NewBusinessError(e.InvalidParameter)
		}
	case model.SysConfigValueTypeJSON:
		if !json.Valid([]byte(value)) {
			return e.NewBusinessError(e.InvalidParameter)
		}
	default:
		return e.NewBusinessError(e.InvalidParameter)
	}
	return nil
}

func valueOrDefault(value *uint8, fallback uint8) uint8 {
	if value == nil {
		return fallback
	}
	return *value
}

func defaultStatus(current uint8, id uint) uint8 {
	if id == 0 && current == 0 {
		return 1
	}
	return current
}
