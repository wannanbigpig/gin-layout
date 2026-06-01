package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/filestorage"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"go.uber.org/zap"
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
		if unmarshalErr := json.Unmarshal([]byte(value), &settings.Config); unmarshalErr != nil {
			log.Logger.Warn("存储配置 JSON 解析失败，使用默认配置", zap.Error(unmarshalErr))
		}
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
	current, _ := s.Get(false)
	mergeSensitiveStorageConfig(&settings.Config, current.Config)
	driver, err := buildStorageDriver(ctx, settings.ActiveDriver, settings.Config)
	if err != nil {
		return newStorageTestBusinessError("init", err)
	}
	key := "storage-test/" + time.Now().Format("20060102150405") + ".txt"
	bucket := bucketForDriver(settings.ActiveDriver, settings.Config, global.Yes)
	if _, err := driver.Put(ctx, filestorage.PutInput{Bucket: bucket, ObjectKey: key, Reader: strings.NewReader("ok"), Size: 2, ContentType: "text/plain"}); err != nil {
		return newStorageTestBusinessError("upload", err)
	}
	exists, err := driver.Exists(ctx, bucket, key)
	if err != nil {
		return newStorageTestBusinessError("exists", err)
	}
	if !exists {
		return newStorageTestBusinessError("exists", fmt.Errorf("storage object not found after put"))
	}
	if _, err := driver.SignedURL(ctx, bucket, key, time.Minute); err != nil {
		return newStorageTestBusinessError("signed_url", err)
	}
	if err := driver.Delete(ctx, bucket, key); err != nil {
		return newStorageTestBusinessError("delete", err)
	}
	return nil
}

func newStorageTestBusinessError(step string, err error) error {
	if err == nil {
		return nil
	}

	stepLabel, permissionLabel, defaultReason := storageTestStepMeta(step)
	message := fmt.Sprintf("%s失败，请检查存储配置、权限和网络连通性", stepLabel)
	if isStoragePermissionDenied(err) {
		message = fmt.Sprintf("%s失败：当前存储凭证没有%s权限，请检查 Bucket ACL、RAM Policy、Bucket Policy 或对象权限", stepLabel, permissionLabel)
	} else if defaultReason != "" {
		message = fmt.Sprintf("%s失败：%s", stepLabel, defaultReason)
	}

	businessErr := e.NewBusinessError(e.StorageTestFailed, message)
	businessErr.AppendContextErr(err)
	return businessErr
}

func storageTestStepMeta(step string) (stepLabel string, permissionLabel string, defaultReason string) {
	switch step {
	case "init":
		return "初始化存储驱动", "访问存储配置", "请检查驱动类型、Endpoint、Bucket 和密钥配置"
	case "upload":
		return "上传测试文件", "上传对象", "请检查 Bucket 写入权限、Endpoint 和网络连通性"
	case "exists":
		return "校验测试文件", "读取对象", "请检查 Bucket 读取权限和对象可见性"
	case "signed_url":
		return "生成测试访问链接", "读取对象", "请检查签名配置、Bucket 读取权限和 Endpoint"
	case "delete":
		return "删除测试文件", "删除对象", "请检查 Bucket 删除权限和对象归属"
	default:
		return "测试存储配置", "访问对象", "请检查存储配置、权限和网络连通性"
	}
}

func isStoragePermissionDenied(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "accessdenied") ||
		strings.Contains(msg, "access denied") ||
		strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "forbidden") ||
		strings.Contains(msg, "no right") ||
		strings.Contains(msg, "no permission")
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
		return filestorage.NewAliyunOSSDriver(config.AliyunOSS), nil
	default:
		return nil, fmt.Errorf("unsupported storage driver: %s", driverName)
	}
}

func bucketForDriver(driverName string, config filestorage.Config, isPublic uint8) string {
	switch driverName {
	case model.StorageDriverAliyunOSS:
		return config.AliyunOSS.Bucket
	default:
		if isPublic == global.Yes {
			return "public"
		}
		return "private"
	}
}

func validateStorageDriver(driverName string) error {
	switch driverName {
	case model.StorageDriverLocal, model.StorageDriverAliyunOSS:
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
	settings.Config.AliyunOSS.AccessKeySecret = maskIfNotEmpty(settings.Config.AliyunOSS.AccessKeySecret)
}

func mergeSensitiveStorageConfig(next *filestorage.Config, current filestorage.Config) {
	if shouldKeepSecret(next.AliyunOSS.AccessKeySecret) {
		next.AliyunOSS.AccessKeySecret = current.AliyunOSS.AccessKeySecret
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

func (s *StorageConfigService) GetSensitiveValue(driverName, field string) (string, error) {
	settings, err := s.Get(false)
	if err != nil {
		return "", err
	}
	switch strings.TrimSpace(driverName) {
	case model.StorageDriverAliyunOSS:
		switch strings.TrimSpace(field) {
		case "access_key_secret":
			return settings.Config.AliyunOSS.AccessKeySecret, nil
		default:
			return "", fmt.Errorf("unsupported storage secret field: %s", field)
		}
	default:
		return "", fmt.Errorf("unsupported storage driver: %s", driverName)
	}
}
