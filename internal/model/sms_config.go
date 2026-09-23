package model

import (
	"errors"

	"gorm.io/gorm"
)

// SysSmsConfig 系统短信服务配置实体。
type SysSmsConfig struct {
	ContainsDeleteBaseModel
	Enabled   bool   `json:"enabled" gorm:"column:enabled;type:tinyint(1);not null;default:0;comment:是否启用短信服务: 0=否, 1=是"`
	Providers string `json:"providers" gorm:"column:providers;type:mediumtext;comment:供应商凭证与配置 JSON（密文保存）"`
	Templates string `json:"templates" gorm:"column:templates;type:text;comment:场景模板配置 JSON"`
	Routes    string `json:"routes" gorm:"column:routes;type:text;comment:场景路由映射 JSON"`
	OTPConfig string `json:"otp_config" gorm:"column:otp_config;type:text;comment:验证码与防刷限制 JSON"`
}

func NewSysSmsConfig() *SysSmsConfig {
	return BindModel(&SysSmsConfig{})
}

func (m *SysSmsConfig) TableName() string {
	return "sys_sms_config"
}

// GetConfig 获取当前短信配置记录。
func (m *SysSmsConfig) GetConfig(tx ...*gorm.DB) (*SysSmsConfig, error) {
	var db *gorm.DB
	if len(tx) > 0 && tx[0] != nil {
		db = tx[0]
	} else {
		var err error
		db, err = m.GetDB()
		if err != nil {
			return nil, err
		}
	}
	var record SysSmsConfig
	if err := db.Model(&SysSmsConfig{}).Order("id ASC").First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// SaveConfig 保存或更新短信配置。
func (m *SysSmsConfig) SaveConfig(tx ...*gorm.DB) error {
	var db *gorm.DB
	if len(tx) > 0 && tx[0] != nil {
		db = tx[0]
	} else {
		var err error
		db, err = m.GetDB()
		if err != nil {
			return err
		}
	}
	current, err := m.GetConfig(db)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if current != nil && current.ID > 0 {
		m.ID = current.ID
		m.CreatedAt = current.CreatedAt
	}
	return db.Save(m).Error
}
