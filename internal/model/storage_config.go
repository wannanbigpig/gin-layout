package model

import (
	"errors"

	"gorm.io/gorm"
)

// SysStorageConfig 系统文件存储配置实体。
type SysStorageConfig struct {
	ContainsDeleteBaseModel
	ActiveDriver        string `json:"active_driver" gorm:"column:active_driver;type:varchar(32);not null;default:'local';comment:当前激活存储驱动"`
	Config              string `json:"config" gorm:"column:config;type:mediumtext;comment:各驱动配置及凭证密文 JSON"`
	SignedURLTTLSeconds int    `json:"signed_url_ttl_seconds" gorm:"column:signed_url_ttl_seconds;type:int unsigned;not null;default:300;comment:签名URL有效期秒数"`
	MaxFileSizeMB       int    `json:"max_file_size_mb" gorm:"column:max_file_size_mb;type:int unsigned;not null;default:10;comment:最大上传大小MB"`
	AllowedMIMETypes    string `json:"allowed_mime_types" gorm:"column:allowed_mime_types;type:text;comment:允许的MIME类型 JSON"`
}

func NewSysStorageConfig() *SysStorageConfig {
	return BindModel(&SysStorageConfig{})
}

func (m *SysStorageConfig) TableName() string {
	return "sys_storage_config"
}

// GetConfig 获取当前存储配置记录。
func (m *SysStorageConfig) GetConfig(tx ...*gorm.DB) (*SysStorageConfig, error) {
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
	var record SysStorageConfig
	if err := db.Model(&SysStorageConfig{}).Order("id ASC").First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// SaveConfig 保存或更新存储配置。
func (m *SysStorageConfig) SaveConfig(tx ...*gorm.DB) error {
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
