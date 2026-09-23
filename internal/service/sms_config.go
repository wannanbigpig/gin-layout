package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
	"github.com/wannanbigpig/gin-layout/internal/security/credential"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/sms"
	cryptox "github.com/wannanbigpig/gin-layout/pkg/utils/crypto"
	"gorm.io/gorm"
)

const (
	SMSConfigKey       = "sms.config"
	smsConfigManageTab = "sms"
	smsEncryptedPrefix = "enc:v1:"
)

type SMSConfigService struct {
	registry *sms.Registry
}

func NewSMSConfigService() *SMSConfigService {
	return &SMSConfigService{registry: sms.DefaultRegistry()}
}

func (s *SMSConfigService) providerRegistry() *sms.Registry {
	if s != nil && s.registry != nil {
		return s.registry
	}
	return sms.DefaultRegistry()
}

func (s *SMSConfigService) Get(maskSensitive bool) (sms.Config, error) {
	settings := sms.DefaultConfig()

	smsConfigModel := model.NewSysSmsConfig()
	record, err := smsConfigModel.GetConfig()
	if err == nil && record != nil {
		settings.Enabled = record.Enabled
		if strings.TrimSpace(record.Providers) != "" {
			if unmarshalErr := json.Unmarshal([]byte(record.Providers), &settings.Providers); unmarshalErr != nil {
				return sms.Config{}, fmt.Errorf("invalid sms providers configuration: %w", unmarshalErr)
			}
		}
		if strings.TrimSpace(record.Templates) != "" {
			if unmarshalErr := json.Unmarshal([]byte(record.Templates), &settings.Templates); unmarshalErr != nil {
				return sms.Config{}, fmt.Errorf("invalid sms templates configuration: %w", unmarshalErr)
			}
		}
		if strings.TrimSpace(record.Routes) != "" {
			if unmarshalErr := json.Unmarshal([]byte(record.Routes), &settings.Routes); unmarshalErr != nil {
				return sms.Config{}, fmt.Errorf("invalid sms routes configuration: %w", unmarshalErr)
			}
		}
		if strings.TrimSpace(record.OTPConfig) != "" {
			if unmarshalErr := json.Unmarshal([]byte(record.OTPConfig), &settings.OTP); unmarshalErr != nil {
				return sms.Config{}, fmt.Errorf("invalid sms otp configuration: %w", unmarshalErr)
			}
		}
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		if legacy, fbErr := s.getLegacySysConfig(); fbErr == nil {
			settings = legacy
		} else {
			return sms.Config{}, err
		}
	} else {
		if legacy, fbErr := s.getLegacySysConfig(); fbErr == nil {
			settings = legacy
		}
	}

	settings.ApplyDefaults()
	if maskSensitive {
		if err := transformSMSProviderSecrets(&settings, s.providerRegistry(), func(_, _, secret string) (string, error) {
			if strings.TrimSpace(secret) == "" {
				return "", nil
			}
			return sms.MaskPlaceholder, nil
		}); err != nil {
			return sms.Config{}, err
		}
	} else if err := transformSMSProviderSecrets(&settings, s.providerRegistry(), decryptStoredSMSSecret); err != nil {
		return sms.Config{}, err
	}
	return settings, nil
}

func (s *SMSConfigService) getLegacySysConfig() (sms.Config, error) {
	settings := sms.DefaultConfig()
	value, err := storageSysConfigValue(SMSConfigKey)
	if err != nil {
		return settings, err
	}
	if strings.TrimSpace(value) == "" {
		return settings, nil
	}
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return sms.Config{}, fmt.Errorf("invalid legacy sms configuration: %w", err)
	}
	return settings, nil
}

func (s *SMSConfigService) Save(next sms.Config) (string, error) {
	next.ApplyDefaults()
	current, err := s.Get(false)
	if err != nil {
		return "", err
	}
	registry := s.providerRegistry()
	if err := mergeSMSProviderSecrets(&next, current, registry); err != nil {
		return "", newSMSConfigInputError("短信供应商配置格式不正确", err)
	}
	if err := next.ValidateWithRegistry(registry); err != nil {
		return "", newSMSConfigInputError(err.Error(), err)
	}
	if next.Enabled {
		if _, err := sms.NewDispatcher(next, registry); err != nil {
			return "", newSMSConfigInputError("短信供应商、模板或场景路由配置无效", err)
		}
	}
	currentSecrets, err := smsProviderSecretSnapshot(current, registry)
	if err != nil {
		return "", err
	}
	nextSecrets, err := smsProviderSecretSnapshot(next, registry)
	if err != nil {
		return "", err
	}
	secretChanged := !equalStringMaps(currentSecrets, nextSecrets)
	persisted := next
	if err := encryptSMSSecrets(&persisted, registry); err != nil {
		return "", err
	}
	providersPayload, err := json.Marshal(persisted.Providers)
	if err != nil {
		return "", err
	}
	templatesPayload, err := json.Marshal(persisted.Templates)
	if err != nil {
		return "", err
	}
	routesPayload, err := json.Marshal(persisted.Routes)
	if err != nil {
		return "", err
	}
	otpPayload, err := json.Marshal(persisted.OTP)
	if err != nil {
		return "", err
	}
	db, err := model.GetDB()
	if err != nil {
		return "", err
	}
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		smsRecord := model.NewSysSmsConfig()
		smsRecord.Enabled = persisted.Enabled
		smsRecord.Providers = string(providersPayload)
		smsRecord.Templates = string(templatesPayload)
		smsRecord.Routes = string(routesPayload)
		smsRecord.OTPConfig = string(otpPayload)
		return smsRecord.SaveConfig(tx)
	})
	if err != nil {
		return "", err
	}
	before, after := current, next
	if err := clearSMSProviderSecrets(&before, registry); err != nil {
		return "", err
	}
	if err := clearSMSProviderSecrets(&after, registry); err != nil {
		return "", err
	}
	items := auditdiff.BuildFieldDiff(
		map[string]any{"config": before}, map[string]any{"config": after},
		[]auditdiff.FieldRule{{Field: "config", Label: "短信配置"}},
	)
	if secretChanged {
		items = append(items, auditdiff.ChangeDiffItem{Field: "provider_credentials", Label: "短信供应商密钥", Before: "[REDACTED]", After: "[CHANGED]"})
	}
	return auditdiff.Marshal(items), nil
}

func (s *SMSConfigService) LoginEnabled() bool {
	settings, err := s.Get(false)
	if err != nil || !settings.Enabled {
		return false
	}
	_, err = sms.NewDispatcher(settings, s.providerRegistry())
	return err == nil
}

// Test 使用请求中的草稿配置发送测试短信，草稿绝不入库；只有掩码占位符会复用已保存密钥，空值表示明确清空。
func (s *SMSConfigService) Test(ctx context.Context, actorID uint, countryCode, phone string, draft sms.Config) error {
	if actorID == 0 {
		return e.NewBusinessError(e.NotLogin)
	}
	draft.ApplyDefaults()
	registry := s.providerRegistry()
	hasMask, err := hasMaskedSMSProviderSecret(draft, registry)
	if err != nil {
		return newSMSConfigInputError("当前短信测试配置格式不正确", err)
	}
	if hasMask {
		stored, err := s.Get(false)
		if err != nil {
			return err
		}
		if err := mergeSMSProviderSecrets(&draft, stored, registry); err != nil {
			return newSMSConfigInputError("当前短信测试配置格式不正确", err)
		}
	}
	dispatcher, err := buildSMSTestDispatcher(draft, registry)
	if err != nil {
		return err
	}
	client := data.RedisInitializedClient()
	if client == nil || !data.RedisInitializedReady() {
		return e.NewDependencyNotReadyError()
	}
	secret, err := credential.DeriveKey(credential.PurposeSMSQuota)
	if err != nil {
		return err
	}
	phoneKey := smsConfigHMAC(secret, normalizeSMSPhone(countryCode, phone))
	if err := reserveSMSTestQuota(ctx, client, actorID, phoneKey, draft.OTP.GlobalDailyLimit); err != nil {
		return err
	}
	_, err = dispatcher.Send(ctx, sms.SceneLoginOTP, normalizeSMSPhone(countryCode, phone), map[string]string{"code": "123456"})
	if err != nil {
		businessErr := e.NewBusinessError(e.SMSProviderFailed)
		businessErr.AppendContextErr(err)
		return businessErr
	}
	return nil
}

func buildSMSTestDispatcher(settings sms.Config, registries ...*sms.Registry) (*sms.Dispatcher, error) {
	if !settings.Enabled {
		return nil, newSMSConfigInputError("请先在当前测试配置中启用短信", errors.New("draft SMS configuration is disabled"))
	}
	// Config.Validate 只返回本项目维护的配置校验文案，不包含密钥、SQL 或供应商响应，
	// 可以安全返回给管理员，帮助其准确定位未启用的供应商或缺失的场景模板。
	settings.ApplyDefaults()
	registry := sms.DefaultRegistry()
	if len(registries) > 0 && registries[0] != nil {
		registry = registries[0]
	}
	if err := settings.ValidateWithRegistry(registry); err != nil {
		return nil, newSMSConfigInputError(err.Error(), err)
	}
	dispatcher, err := sms.NewDispatcher(settings, registry)
	if err != nil {
		return nil, newSMSConfigInputError("当前短信测试配置中的供应商、login_otp 模板或场景路由无效", err)
	}
	return dispatcher, nil
}

func newSMSConfigInputError(message string, cause error) *e.BusinessError {
	businessErr := e.NewBusinessError(e.InvalidParameter, message)
	if cause != nil {
		businessErr.AppendContextErr(cause)
	}
	return businessErr
}

func reserveSMSTestQuota(ctx context.Context, client *redis.Client, actorID uint, phoneKey string, globalLimit int) error {
	const script = `
if redis.call('EXISTS',KEYS[1])==1 then return 0 end
local actor=tonumber(redis.call('GET',KEYS[2]) or '0')
local target=tonumber(redis.call('GET',KEYS[3]) or '0')
local global=tonumber(redis.call('GET',KEYS[4]) or '0')
if actor>=10 or target>=5 or global>=tonumber(ARGV[1]) then return -1 end
actor=redis.call('INCR',KEYS[2]); if actor==1 then redis.call('EXPIRE',KEYS[2],86400) end
target=redis.call('INCR',KEYS[3]); if target==1 then redis.call('EXPIRE',KEYS[3],86400) end
global=redis.call('INCR',KEYS[4]); if global==1 then redis.call('EXPIRE',KEYS[4],86400) end
redis.call('SET',KEYS[1],'1','EX',60)
return 1`
	actorKey := fmt.Sprintf("sms:test:actor-day:%d", actorID)
	result, err := client.Eval(ctx, script, []string{"sms:test:cooldown:" + phoneKey, actorKey, "sms:test:target-day:" + phoneKey, "sms:budget:global-day"}, globalLimit).Int()
	if err != nil {
		return e.NewDependencyNotReadyError()
	}
	if result != 1 {
		return e.NewBusinessError(e.TooManyRequests, "测试短信发送过于频繁，请稍后重试")
	}
	return nil
}

func smsConfigHMAC(secret, value string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func upsertSMSSysConfig(tx *gorm.DB, value string) error {
	configModel := model.NewSysConfig()
	configModel.SetDB(tx)
	err := configModel.FindByKey(SMSConfigKey)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		configModel.ConfigKey = SMSConfigKey
	}
	configModel.ConfigValue = value
	configModel.ValueType = model.SysConfigValueTypeJSON
	configModel.GroupCode = "sms"
	configModel.IsSystem = 1
	configModel.IsSensitive = 1
	configModel.IsVisible = 0
	configModel.ManageTab = smsConfigManageTab
	configModel.Status = global.Yes
	configModel.Sort = 70
	configModel.Remark = "短信供应商、模板、场景路由与验证码配置"
	if err := tx.Save(configModel).Error; err != nil {
		return err
	}
	return model.NewSysConfigI18n().UpsertConfigNames(configModel.ID, map[string]string{
		i18n.LocaleZhCN: "短信配置",
		i18n.LocaleEnUS: "SMS Config",
	}, tx)
}

type smsProviderSecretTransform func(provider, field, value string) (string, error)

func transformSMSProviderSecrets(settings *sms.Config, registry *sms.Registry, transform smsProviderSecretTransform) error {
	if settings == nil || transform == nil {
		return nil
	}
	if registry == nil {
		return errors.New("短信供应商注册表未初始化")
	}
	for provider, raw := range settings.Providers {
		definition, ok := registry.Definition(provider)
		if !ok {
			return fmt.Errorf("短信供应商 %s 未注册", provider)
		}
		if len(definition.SensitiveFields) == 0 {
			continue
		}
		object := make(map[string]json.RawMessage)
		if err := json.Unmarshal(raw, &object); err != nil {
			return fmt.Errorf("短信供应商 %s 配置格式错误: %w", provider, err)
		}
		for _, field := range definition.SensitiveFields {
			encoded, exists := object[field]
			if !exists {
				continue
			}
			var value string
			if err := json.Unmarshal(encoded, &value); err != nil {
				return fmt.Errorf("短信供应商 %s 的敏感字段 %s 必须是字符串", provider, field)
			}
			next, err := transform(provider, field, value)
			if err != nil {
				return err
			}
			object[field], err = json.Marshal(next)
			if err != nil {
				return err
			}
		}
		encoded, err := json.Marshal(object)
		if err != nil {
			return err
		}
		settings.Providers[provider] = encoded
	}
	return nil
}

func smsProviderSecretValue(settings sms.Config, provider, field string) (string, error) {
	raw, ok := settings.Providers[provider]
	if !ok {
		return "", nil
	}
	object := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &object); err != nil {
		return "", err
	}
	encoded, ok := object[field]
	if !ok {
		return "", nil
	}
	var value string
	if err := json.Unmarshal(encoded, &value); err != nil {
		return "", err
	}
	return value, nil
}

func mergeSMSProviderSecrets(next *sms.Config, current sms.Config, registry *sms.Registry) error {
	return transformSMSProviderSecrets(next, registry, func(provider, field, value string) (string, error) {
		if value != sms.MaskPlaceholder {
			return value, nil
		}
		return smsProviderSecretValue(current, provider, field)
	})
}

func smsProviderSecretSnapshot(settings sms.Config, registry *sms.Registry) (map[string]string, error) {
	result := make(map[string]string)
	if registry == nil {
		return nil, errors.New("短信供应商注册表未初始化")
	}
	for provider := range settings.Providers {
		definition, ok := registry.Definition(provider)
		if !ok {
			return nil, fmt.Errorf("短信供应商 %s 未注册", provider)
		}
		for _, field := range definition.SensitiveFields {
			value, err := smsProviderSecretValue(settings, provider, field)
			if err != nil {
				return nil, err
			}
			result[provider+"."+field] = value
		}
	}
	return result, nil
}

func equalStringMaps(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func hasMaskedSMSProviderSecret(settings sms.Config, registry *sms.Registry) (bool, error) {
	values, err := smsProviderSecretSnapshot(settings, registry)
	if err != nil {
		return false, err
	}
	for _, value := range values {
		if value == sms.MaskPlaceholder {
			return true, nil
		}
	}
	return false, nil
}

func clearSMSProviderSecrets(settings *sms.Config, registry *sms.Registry) error {
	return transformSMSProviderSecrets(settings, registry, func(_, _, _ string) (string, error) { return "", nil })
}

func normalizeSMSPhone(countryCode, phone string) string {
	return "+" + strings.TrimPrefix(strings.TrimSpace(countryCode), "+") + strings.TrimSpace(phone)
}

func smsCredentialSecretKey() (string, error) {
	return credential.DeriveKey(credential.PurposeSMSProvider)
}

func encryptSMSSecrets(settings *sms.Config, registries ...*sms.Registry) error {
	registry := sms.DefaultRegistry()
	if len(registries) > 0 && registries[0] != nil {
		registry = registries[0]
	}
	return transformSMSProviderSecrets(settings, registry, func(_, _, secret string) (string, error) {
		if strings.TrimSpace(secret) == "" {
			return "", nil
		}
		key, err := smsCredentialSecretKey()
		if err != nil {
			return "", err
		}
		encrypted, err := cryptox.Encrypt(key, secret)
		if err != nil {
			return "", err
		}
		return smsEncryptedPrefix + encrypted, nil
	})
}

func decryptStoredSMSSecret(_, _, secret string) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", nil
	}
	if !strings.HasPrefix(secret, smsEncryptedPrefix) {
		return "", errors.New("短信凭据不是受支持的加密格式")
	}
	key, err := smsCredentialSecretKey()
	if err != nil {
		return "", err
	}
	return cryptox.Decrypt(key, strings.TrimPrefix(secret, smsEncryptedPrefix))
}
