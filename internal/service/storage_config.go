package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/filestorage"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/security/credential"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	cryptox "github.com/wannanbigpig/gin-layout/pkg/utils/crypto"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	StorageActiveDriverConfigKey = "storage.active_driver"
	StorageConfigConfigKey       = "storage.config"
	storageConfigManageTab       = "storage"
	storageEncryptedPrefix       = "enc:v1:"
)

type StorageConfigService struct {
	registry *filestorage.Registry
}

func NewStorageConfigService() *StorageConfigService {
	return &StorageConfigService{registry: filestorage.DefaultRegistry()}
}

func newStorageConfigServiceWithRegistry(registry *filestorage.Registry) *StorageConfigService {
	if registry == nil {
		registry = filestorage.DefaultRegistry()
	}
	return &StorageConfigService{registry: registry}
}

func (s *StorageConfigService) driverRegistry() *filestorage.Registry {
	if s != nil && s.registry != nil {
		return s.registry
	}
	return filestorage.DefaultRegistry()
}

type StorageSettings struct {
	ActiveDriver string             `json:"active_driver"`
	Config       filestorage.Config `json:"config"`
}

func (s *StorageConfigService) Get(maskSensitive bool) (StorageSettings, error) {
	settings := StorageSettings{ActiveDriver: model.StorageDriverLocal, Config: filestorage.DefaultConfig()}

	storageConfigModel := model.NewSysStorageConfig()
	record, err := storageConfigModel.GetConfig()
	if err == nil && record != nil {
		if strings.TrimSpace(record.ActiveDriver) != "" {
			settings.ActiveDriver = strings.TrimSpace(record.ActiveDriver)
		}
		if strings.TrimSpace(record.Config) != "" {
			if unmarshalErr := json.Unmarshal([]byte(record.Config), &settings.Config); unmarshalErr != nil {
				return StorageSettings{}, fmt.Errorf("invalid storage configuration: %w", unmarshalErr)
			}
		}
		if record.SignedURLTTLSeconds > 0 {
			settings.Config.SignedURLTTLSeconds = record.SignedURLTTLSeconds
		}
		if record.MaxFileSizeMB > 0 {
			settings.Config.MaxFileSizeMB = record.MaxFileSizeMB
		}
		if strings.TrimSpace(record.AllowedMIMETypes) != "" {
			var mimes []string
			if err := json.Unmarshal([]byte(record.AllowedMIMETypes), &mimes); err == nil {
				settings.Config.AllowedMimeTypes = mimes
			}
		}
		if maskSensitive {
			maskStorageSecretWithRegistry(&settings.Config, s.driverRegistry())
		} else if decryptErr := decryptStorageSecretWithRegistry(&settings.Config, s.driverRegistry()); decryptErr != nil {
			return StorageSettings{}, decryptErr
		}
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		if legacy, fbErr := s.getLegacySysConfig(maskSensitive); fbErr == nil {
			return legacy, nil
		}
		return StorageSettings{}, err
	} else {
		if legacy, fbErr := s.getLegacySysConfig(maskSensitive); fbErr == nil {
			return legacy, nil
		}
	}

	if settings.Config.SignedURLTTLSeconds <= 0 {
		settings.Config.SignedURLTTLSeconds = 300
	}
	if settings.Config.MaxFileSizeMB <= 0 {
		settings.Config.MaxFileSizeMB = 10
	}
	applyLocalStorageDefaults(&settings)
	return settings, nil
}

func (s *StorageConfigService) getLegacySysConfig(maskSensitive bool) (StorageSettings, error) {
	settings := StorageSettings{ActiveDriver: model.StorageDriverLocal, Config: filestorage.DefaultConfig()}
	found := false
	if value, err := storageSysConfigValue(StorageActiveDriverConfigKey); err == nil && strings.TrimSpace(value) != "" {
		settings.ActiveDriver = strings.TrimSpace(value)
		found = true
	}
	if value, err := storageSysConfigValue(StorageConfigConfigKey); err == nil && strings.TrimSpace(value) != "" {
		if unmarshalErr := json.Unmarshal([]byte(value), &settings.Config); unmarshalErr != nil {
			return StorageSettings{}, fmt.Errorf("invalid storage configuration: %w", unmarshalErr)
		}
		found = true
		if maskSensitive {
			maskStorageSecretWithRegistry(&settings.Config, s.driverRegistry())
		} else if decryptErr := decryptStorageSecretWithRegistry(&settings.Config, s.driverRegistry()); decryptErr != nil {
			return StorageSettings{}, decryptErr
		}
	}
	if !found {
		return StorageSettings{}, gorm.ErrRecordNotFound
	}
	return settings, nil
}

func (s *StorageConfigService) Save(next StorageSettings) error {
	_, err := s.SaveWithAuditDiff(next)
	return err
}

func (s *StorageConfigService) SaveWithAuditDiff(next StorageSettings) (string, error) {
	if err := validateStorageDriverWithRegistry(s.driverRegistry(), next.ActiveDriver); err != nil {
		return "", err
	}
	current, err := s.Get(false)
	if err != nil {
		return "", err
	}
	mergeSensitiveStorageConfigWithRegistry(&next.Config, current.Config, s.driverRegistry())
	applyLocalStorageDefaults(&next)
	if err := s.driverRegistry().Validate(next.ActiveDriver, next.Config); err != nil {
		return "", newStorageConfigInputError(err.Error(), err)
	}
	persisted := next.Config
	if err := encryptStorageSecretWithRegistry(&persisted, s.driverRegistry()); err != nil {
		return "", err
	}
	payload, err := json.Marshal(persisted)
	if err != nil {
		return "", err
	}
	allowedMimes, err := json.Marshal(persisted.AllowedMimeTypes)
	if err != nil {
		allowedMimes = []byte("[]")
	}
	db, err := model.GetDB()
	if err != nil {
		return "", err
	}
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		storageRecord := model.NewSysStorageConfig()
		storageRecord.ActiveDriver = next.ActiveDriver
		storageRecord.Config = string(payload)
		storageRecord.SignedURLTTLSeconds = persisted.SignedURLTTLSeconds
		storageRecord.MaxFileSizeMB = persisted.MaxFileSizeMB
		storageRecord.AllowedMIMETypes = string(allowedMimes)
		return storageRecord.SaveConfig(tx)
	})
	if err != nil {
		return "", err
	}
	return storageSettingsAuditDiffWithRegistry(current, next, s.driverRegistry()), nil
}

func storageSettingsAuditDiff(before, after StorageSettings) string {
	return storageSettingsAuditDiffWithRegistry(before, after, filestorage.DefaultRegistry())
}

func storageSettingsAuditDiffWithRegistry(before, after StorageSettings, registry *filestorage.Registry) string {
	type changedSecret struct {
		field string
		label string
	}
	changedSecrets := make([]changedSecret, 0)
	if registry == nil {
		registry = filestorage.DefaultRegistry()
	}
	for _, driverName := range registry.Names() {
		fields, err := registry.SensitiveFields(driverName)
		if err != nil {
			continue
		}
		for _, field := range fields {
			if field.Get == nil || field.Set == nil {
				continue
			}
			if field.Get(before.Config) != field.Get(after.Config) {
				changedSecrets = append(changedSecrets, changedSecret{field: driverName + "." + field.Name, label: field.Label})
			}
			field.Set(&before.Config, "")
			field.Set(&after.Config, "")
		}
	}
	items := auditdiff.BuildFieldDiff(
		map[string]any{"active_driver": before.ActiveDriver, "config": before.Config},
		map[string]any{"active_driver": after.ActiveDriver, "config": after.Config},
		[]auditdiff.FieldRule{{Field: "active_driver", Label: "存储驱动"}, {Field: "config", Label: "存储参数"}},
	)
	for _, secret := range changedSecrets {
		items = append(items, auditdiff.ChangeDiffItem{Field: secret.field, Label: secret.label, Before: "[REDACTED]", After: "[CHANGED]"})
	}
	return auditdiff.Marshal(items)
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
	current, err := s.Get(false)
	if err != nil {
		return err
	}
	mergeSensitiveStorageConfigWithRegistry(&settings.Config, current.Config, s.driverRegistry())
	applyLocalStorageDefaults(&settings)
	driver, err := buildStorageDriverWithRegistry(ctx, s.driverRegistry(), settings.ActiveDriver, settings.Config)
	if err != nil {
		return newStorageTestBusinessError("init", err)
	}
	key := "storage-test/" + strings.ReplaceAll(uuid.NewString(), "-", "") + ".txt"
	bucket, err := s.driverRegistry().ResolveBucket(settings.ActiveDriver, settings.Config, true)
	if err != nil {
		return newStorageTestBusinessError("init", err)
	}
	if _, err := driver.Put(ctx, filestorage.PutInput{Bucket: bucket, ObjectKey: key, Reader: strings.NewReader("ok"), Size: 2, ContentType: "text/plain"}); err != nil {
		return newStorageTestBusinessError("upload", err)
	}
	cleanupNeeded := true
	defer func() {
		if cleanupNeeded {
			if cleanupErr := driver.Delete(context.Background(), bucket, key); cleanupErr != nil {
				log.Logger.Warn("清理存储测试文件失败", zap.String("driver", settings.ActiveDriver), zap.String("bucket", bucket), zap.String("object_key", key), zap.Error(cleanupErr))
			}
		}
	}()
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
	cleanupNeeded = false
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
	return buildStorageDriverWithRegistry(ctx, filestorage.DefaultRegistry(), driverName, config)
}

func buildStorageDriverWithRegistry(ctx context.Context, registry *filestorage.Registry, driverName string, config filestorage.Config) (filestorage.Driver, error) {
	if registry == nil {
		registry = filestorage.DefaultRegistry()
	}
	return registry.Build(ctx, driverName, config)
}

func bucketForDriver(driverName string, config filestorage.Config, isPublic uint8) string {
	bucket, _ := filestorage.DefaultRegistry().ResolveBucket(driverName, config, isPublic == global.Yes)
	return bucket
}

func validateStorageDriver(driverName string) error {
	return validateStorageDriverWithRegistry(filestorage.DefaultRegistry(), driverName)
}

func validateStorageDriverWithRegistry(registry *filestorage.Registry, driverName string) error {
	if registry == nil {
		registry = filestorage.DefaultRegistry()
	}
	if _, ok := registry.Definition(driverName); !ok {
		return newStorageConfigInputError("不支持的存储驱动: "+strings.TrimSpace(driverName), nil)
	}
	return nil
}

func upsertStorageSysConfig(tx *gorm.DB, key, value, valueType string, sensitive uint8, remark string, names map[string]string) error {
	configModel := model.NewSysConfig()
	configModel.SetDB(tx)
	err := configModel.FindByKey(key)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
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

func mergeSensitiveStorageConfig(next *filestorage.Config, current filestorage.Config) {
	mergeSensitiveStorageConfigWithRegistry(next, current, filestorage.DefaultRegistry())
}

func mergeSensitiveStorageConfigWithRegistry(next *filestorage.Config, current filestorage.Config, registry *filestorage.Registry) {
	if next == nil {
		return
	}
	if registry == nil {
		registry = filestorage.DefaultRegistry()
	}
	for _, driverName := range registry.Names() {
		fields, err := registry.SensitiveFields(driverName)
		if err != nil {
			continue
		}
		for _, field := range fields {
			if field.Get != nil && field.Set != nil && strings.TrimSpace(field.Get(*next)) == filestorage.MaskPlaceholder {
				field.Set(next, field.Get(current))
			}
		}
	}
}

func encryptStorageSecret(storageConfig *filestorage.Config) error {
	return encryptStorageSecretWithRegistry(storageConfig, filestorage.DefaultRegistry())
}

func encryptStorageSecretWithRegistry(storageConfig *filestorage.Config, registry *filestorage.Registry) error {
	return transformStorageSecretsWithRegistry(storageConfig, registry, func(driverName, secret string) (string, error) {
		if strings.TrimSpace(secret) == "" {
			return "", nil
		}
		key, err := storageCredentialSecretKey(driverName)
		if err != nil {
			return "", err
		}
		encrypted, err := cryptox.Encrypt(key, secret)
		if err != nil {
			return "", err
		}
		return storageEncryptedPrefix + encrypted, nil
	})
}

func decryptStorageSecret(storageConfig *filestorage.Config) error {
	return decryptStorageSecretWithRegistry(storageConfig, filestorage.DefaultRegistry())
}

func decryptStorageSecretWithRegistry(storageConfig *filestorage.Config, registry *filestorage.Registry) error {
	return transformStorageSecretsWithRegistry(storageConfig, registry, func(driverName, secret string) (string, error) {
		if strings.TrimSpace(secret) == "" {
			return "", nil
		}
		if !strings.HasPrefix(secret, storageEncryptedPrefix) {
			return "", errors.New("对象存储凭据不是受支持的加密格式")
		}
		key, err := storageCredentialSecretKey(driverName)
		if err != nil {
			return "", err
		}
		return cryptox.Decrypt(key, strings.TrimPrefix(secret, storageEncryptedPrefix))
	})
}

func maskStorageSecret(storageConfig *filestorage.Config) {
	maskStorageSecretWithRegistry(storageConfig, filestorage.DefaultRegistry())
}

func maskStorageSecretWithRegistry(storageConfig *filestorage.Config, registry *filestorage.Registry) {
	_ = transformStorageSecretsWithRegistry(storageConfig, registry, func(_ string, secret string) (string, error) {
		if strings.TrimSpace(secret) == "" {
			return "", nil
		}
		return filestorage.MaskPlaceholder, nil
	})
}

func transformStorageSecrets(storageConfig *filestorage.Config, transform func(driverName, secret string) (string, error)) error {
	return transformStorageSecretsWithRegistry(storageConfig, filestorage.DefaultRegistry(), transform)
}

func transformStorageSecretsWithRegistry(storageConfig *filestorage.Config, registry *filestorage.Registry, transform func(driverName, secret string) (string, error)) error {
	if storageConfig == nil || transform == nil {
		return nil
	}
	if registry == nil {
		registry = filestorage.DefaultRegistry()
	}
	for _, driverName := range registry.Names() {
		fields, err := registry.SensitiveFields(driverName)
		if err != nil {
			return err
		}
		for _, field := range fields {
			if field.Get == nil || field.Set == nil {
				continue
			}
			value, err := transform(driverName, field.Get(*storageConfig))
			if err != nil {
				return err
			}
			field.Set(storageConfig, value)
		}
	}
	return nil
}

func storageCredentialSecretKey(driverName string) (string, error) {
	purposeName := strings.ReplaceAll(strings.TrimSpace(driverName), "_", "-")
	return credential.DeriveKey(credential.Purpose("storage-" + purposeName + "-credentials:v1"))
}

func newStorageConfigInputError(message string, cause error) *e.BusinessError {
	businessErr := e.NewBusinessError(e.InvalidParameter, message)
	if cause != nil {
		businessErr.AppendContextErr(cause)
	}
	return businessErr
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstUint(values ...uint) uint {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func (s *StorageConfigService) GetSensitiveValue(driverName, field string) (string, error) {
	settings, err := s.Get(false)
	if err != nil {
		return "", err
	}
	fields, err := s.driverRegistry().SensitiveFields(driverName)
	if err != nil {
		return "", newStorageConfigInputError(err.Error(), err)
	}
	for _, definition := range fields {
		if definition.Name == strings.TrimSpace(field) && definition.Get != nil {
			return definition.Get(settings.Config), nil
		}
	}
	return "", newStorageConfigInputError("不支持的存储敏感字段: "+strings.TrimSpace(field), nil)
}
