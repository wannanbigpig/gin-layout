package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/filestorage"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"gorm.io/gorm"
)

const (
	StorageActiveDriverConfigKey = "storage.active_driver"
	StorageConfigConfigKey       = "storage.config"
	storageConfigManageTab       = "storage"
)

type StorageConfigService struct{}

func NewStorageConfigService() *StorageConfigService {
	return &StorageConfigService{}
}

type StorageSettings struct {
	ActiveDriver string             `json:"active_driver"`
	Config       filestorage.Config `json:"config"`
}

func (s *StorageConfigService) Get(maskSensitive bool) (StorageSettings, error) {
	settings := StorageSettings{ActiveDriver: model.StorageDriverLocal, Config: filestorage.DefaultConfig()}
	if value, err := storageSysConfigValue(StorageActiveDriverConfigKey); err == nil && strings.TrimSpace(value) != "" {
		settings.ActiveDriver = strings.TrimSpace(value)
	}
	if value, err := storageSysConfigValue(StorageConfigConfigKey); err == nil && strings.TrimSpace(value) != "" {
		_ = json.Unmarshal([]byte(value), &settings.Config)
	}
	if settings.Config.SignedURLTTLSeconds <= 0 {
		settings.Config.SignedURLTTLSeconds = 300
	}
	if settings.Config.MaxFileSizeMB <= 0 {
		settings.Config.MaxFileSizeMB = 10
	}
	applyLocalStorageDefaults(&settings)
	if maskSensitive {
		maskStorageSettings(&settings)
	}
	return settings, nil
}

func (s *StorageConfigService) Save(next StorageSettings) error {
	if err := validateStorageDriver(next.ActiveDriver); err != nil {
		return err
	}
	current, _ := s.Get(false)
	mergeSensitiveStorageConfig(&next.Config, current.Config)
	payload, err := json.Marshal(next.Config)
	if err != nil {
		return err
	}
	db, err := model.GetDB()
	if err != nil {
		return err
	}
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		if err := upsertStorageSysConfig(tx, StorageActiveDriverConfigKey, next.ActiveDriver, model.SysConfigValueTypeString, 0, "当前启用的文件存储驱动", map[string]string{
			i18n.LocaleZhCN: "当前存储驱动",
			i18n.LocaleEnUS: "Active Storage Driver",
		}); err != nil {
			return err
		}
		return upsertStorageSysConfig(tx, StorageConfigConfigKey, string(payload), model.SysConfigValueTypeJSON, 1, "文件存储配置", map[string]string{
			i18n.LocaleZhCN: "文件存储配置",
			i18n.LocaleEnUS: "File Storage Config",
		})
	})
	if err != nil {
		return err
	}
	return nil
}

func storageSysConfigValue(key string) (string, error) {
	configModel := model.NewSysConfig()
	if err := configModel.FindByKey(key); err != nil {
		return "", err
	}
	if configModel.Status != global.Yes {
		return "", gorm.ErrRecordNotFound
	}
	return configModel.ConfigValue, nil
}

func (s *StorageConfigService) Test(ctx context.Context, settings StorageSettings) error {
	driver, err := buildStorageDriver(ctx, settings.ActiveDriver, settings.Config)
	if err != nil {
		return err
	}
	key := "storage-test/" + time.Now().Format("20060102150405") + ".txt"
	bucket := bucketForDriver(settings.ActiveDriver, settings.Config, global.Yes)
	if _, err := driver.Put(ctx, filestorage.PutInput{Bucket: bucket, ObjectKey: key, Reader: strings.NewReader("ok"), Size: 2, ContentType: "text/plain"}); err != nil {
		return err
	}
	exists, err := driver.Exists(ctx, bucket, key)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("storage object not found after put")
	}
	if _, err := driver.SignedURL(ctx, bucket, key, time.Minute); err != nil {
		return err
	}
	return driver.Delete(ctx, bucket, key)
}

func NewActiveStorageDriver(ctx context.Context) (filestorage.Driver, filestorage.Config, string, error) {
	settings, err := NewStorageConfigService().Get(false)
	if err != nil {
		return nil, filestorage.Config{}, "", err
	}
	driver, err := buildStorageDriver(ctx, settings.ActiveDriver, settings.Config)
	return driver, settings.Config, settings.ActiveDriver, err
}

func NewStorageDriverByName(ctx context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
	settings, err := NewStorageConfigService().Get(false)
	if err != nil {
		return nil, filestorage.Config{}, err
	}
	driver, err := buildStorageDriver(ctx, driverName, settings.Config)
	return driver, settings.Config, err
}

func buildStorageDriver(ctx context.Context, driverName string, config filestorage.Config) (filestorage.Driver, error) {
	if err := validateStorageDriver(driverName); err != nil {
		return nil, err
	}
	switch driverName {
	case model.StorageDriverLocal:
		return filestorage.NewLocalDriver(config.Local, storageBasePath(true), storageBasePath(false)), nil
	case model.StorageDriverAliyunOSS:
		if strings.TrimSpace(config.AliyunOSS.Bucket) == "" {
			return nil, fmt.Errorf("aliyun_oss.bucket is required")
		}
		return filestorage.NewAliyunOSSDriver(config.AliyunOSS), nil
	case model.StorageDriverS3:
		if strings.TrimSpace(config.S3.Bucket) == "" {
			return nil, fmt.Errorf("s3.bucket is required")
		}
		return filestorage.NewS3Driver(ctx, config.S3)
	default:
		return nil, fmt.Errorf("unsupported storage driver: %s", driverName)
	}
}

func bucketForDriver(driverName string, config filestorage.Config, isPublic uint8) string {
	switch driverName {
	case model.StorageDriverAliyunOSS:
		return config.AliyunOSS.Bucket
	case model.StorageDriverS3:
		return config.S3.Bucket
	default:
		if isPublic == global.Yes {
			return "public"
		}
		return "private"
	}
}

func validateStorageDriver(driverName string) error {
	switch driverName {
	case model.StorageDriverLocal, model.StorageDriverAliyunOSS, model.StorageDriverS3:
		return nil
	default:
		return fmt.Errorf("unsupported storage driver: %s", driverName)
	}
}

func upsertStorageSysConfig(tx *gorm.DB, key, value, valueType string, sensitive uint8, remark string, names map[string]string) error {
	configModel := model.NewSysConfig()
	configModel.SetDB(tx)
	err := configModel.FindByKey(key)
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if err == gorm.ErrRecordNotFound {
		configModel.ConfigKey = key
	}
	configModel.ConfigValue = value
	configModel.ValueType = valueType
	configModel.GroupCode = "storage"
	configModel.IsSystem = 1
	configModel.IsSensitive = sensitive
	configModel.IsVisible = 0
	configModel.ManageTab = storageConfigManageTab
	configModel.Status = 1
	configModel.Sort = 90
	configModel.Remark = remark
	if err := tx.Save(configModel).Error; err != nil {
		return err
	}
	return model.NewSysConfigI18n().UpsertConfigNames(configModel.ID, names, tx)
}

func maskStorageSettings(settings *StorageSettings) {
	settings.Config.AliyunOSS.AccessKeyID = maskIfNotEmpty(settings.Config.AliyunOSS.AccessKeyID)
	settings.Config.AliyunOSS.AccessKeySecret = maskIfNotEmpty(settings.Config.AliyunOSS.AccessKeySecret)
	settings.Config.S3.AccessKeyID = maskIfNotEmpty(settings.Config.S3.AccessKeyID)
	settings.Config.S3.SecretAccessKey = maskIfNotEmpty(settings.Config.S3.SecretAccessKey)
}

func mergeSensitiveStorageConfig(next *filestorage.Config, current filestorage.Config) {
	if shouldKeepSecret(next.AliyunOSS.AccessKeyID) {
		next.AliyunOSS.AccessKeyID = current.AliyunOSS.AccessKeyID
	}
	if shouldKeepSecret(next.AliyunOSS.AccessKeySecret) {
		next.AliyunOSS.AccessKeySecret = current.AliyunOSS.AccessKeySecret
	}
	if shouldKeepSecret(next.S3.AccessKeyID) {
		next.S3.AccessKeyID = current.S3.AccessKeyID
	}
	if shouldKeepSecret(next.S3.SecretAccessKey) {
		next.S3.SecretAccessKey = current.S3.SecretAccessKey
	}
}

func applyLocalStorageDefaults(settings *StorageSettings) {
	if settings == nil {
		return
	}
	cfg := c.GetConfig()
	if cfg == nil {
		return
	}
	basePath := filepath.Join(cfg.BasePath, "storage")
	if strings.TrimSpace(settings.Config.Local.BasePath) == "" {
		settings.Config.Local.BasePath = basePath
	}
	if strings.TrimSpace(settings.Config.Local.PublicBasePath) == "" {
		settings.Config.Local.PublicBasePath = filepath.Join(basePath, "public")
	}
	if strings.TrimSpace(settings.Config.Local.PrivateBasePath) == "" {
		settings.Config.Local.PrivateBasePath = filepath.Join(basePath, "private")
	}
}

func maskIfNotEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return filestorage.MaskPlaceholder
}

func shouldKeepSecret(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || value == filestorage.MaskPlaceholder
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func ensureLocalStorageDirs() error {
	cfg := c.GetConfig()
	if cfg == nil {
		return nil
	}
	for _, path := range []string{filepath.Join(cfg.BasePath, "storage/public"), filepath.Join(cfg.BasePath, "storage/private")} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}
	return nil
}
