package model

import (
	"strings"

	"gorm.io/gorm"
)

const (
	SysConfigValueTypeString = "string"
	SysConfigValueTypeNumber = "number"
	SysConfigValueTypeBool   = "bool"
	SysConfigValueTypeJSON   = "json"
)

// SysConfig 系统参数表。
type SysConfig struct {
	ContainsDeleteBaseModel
	ConfigKey      string            `json:"config_key" gorm:"column:config_key;type:varchar(100);not null;default:'';comment:参数键名"`
	ConfigName     string            `json:"config_name" gorm:"-:all"`
	ConfigNameI18n map[string]string `json:"config_name_i18n" gorm:"-:all"`
	ConfigValue    string            `json:"config_value" gorm:"column:config_value;type:text;comment:参数值"`
	ValueType      string            `json:"value_type" gorm:"column:value_type;type:varchar(20);not null;default:'string';comment:值类型"`
	GroupCode      string            `json:"group_code" gorm:"column:group_code;type:varchar(60);not null;default:'default';comment:参数分组"`
	IsSystem       uint8             `json:"is_system" gorm:"column:is_system;type:tinyint unsigned;not null;default:0;comment:是否系统内置"`
	IsSensitive    uint8             `json:"is_sensitive" gorm:"column:is_sensitive;type:tinyint unsigned;not null;default:0;comment:是否敏感配置"`
	Status         uint8             `json:"status" gorm:"column:status;type:tinyint unsigned;not null;default:1;comment:状态"`
	Sort           uint              `json:"sort" gorm:"column:sort;type:int unsigned;not null;default:0;comment:排序"`
	Remark         string            `json:"remark" gorm:"column:remark;type:varchar(255);not null;default:'';comment:备注"`
}

func NewSysConfig() *SysConfig {
	return BindModel(&SysConfig{})
}

func (m *SysConfig) TableName() string {
	return "sys_config"
}

// IsProtected 判断参数是否为系统内置保护项。
func (m *SysConfig) IsProtected() bool {
	return m != nil && m.IsSystem == 1
}

// NormalizeValueType 归一化参数值类型。
func NormalizeValueType(valueType string) string {
	valueType = strings.TrimSpace(strings.ToLower(valueType))
	if valueType == "" {
		return SysConfigValueTypeString
	}
	return valueType
}

// FindByKey 根据参数键查询未删除参数。
func (m *SysConfig) FindByKey(key string) error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Where("config_key = ? AND deleted_at = 0", key).First(m).Error
}

// ExistsByKeyExcludeID 检查参数键是否已被其他记录占用。
func (m *SysConfig) ExistsByKeyExcludeID(key string, excludeID uint) (bool, error) {
	db, err := m.GetDB()
	if err != nil {
		return false, err
	}
	var count int64
	query := db.Model(m).Where("config_key = ? AND deleted_at = 0", key)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// EnabledConfigs 查询所有启用参数，用于刷新缓存。
func (m *SysConfig) EnabledConfigs(tx ...*gorm.DB) ([]SysConfig, error) {
	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}
	if len(tx) > 0 && tx[0] != nil {
		db = tx[0]
	}
	var configs []SysConfig
	err = db.Where("status = 1 AND deleted_at = 0").Order("sort desc, id desc").Find(&configs).Error
	return configs, err
}
