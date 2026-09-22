package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/security/credential"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/sms"
)

const (
	smsChallengePrefix         = "auth:sms:challenge:"
	smsSendCodeMinimumDuration = 500 * time.Millisecond
)

type SMSCodeResult struct {
	ChallengeID string `json:"challenge_id"`
	RetryAfter  int    `json:"retry_after"`
	ExpiresIn   int    `json:"expires_in"`
}

type smsLoginSender interface {
	Send(context.Context, string, string, map[string]string) (sms.SendResult, error)
}

type SMSLoginService struct {
	redis             *redis.Client
	configLoader      func() (sms.Config, error)
	userFinder        func(string) (*model.AdminUser, error)
	senderFactory     func(sms.Config) (smsLoginSender, error)
	codeGenerator     func() (string, error)
	skipResponseDelay bool
}

func NewSMSLoginService() *SMSLoginService {
	return &SMSLoginService{}
}

func (s *SMSLoginService) Options() map[string]bool {
	return map[string]bool{"sms_login_enabled": service.NewSMSConfigService().LoginEnabled() && data.RedisInitializedReady()}
}

func (s *SMSLoginService) SendCode(ctx context.Context, countryCode, phone, clientIP string) (*SMSCodeResult, error) {
	startedAt := time.Now()
	defer s.equalizeSendCodeResponseTime(ctx, startedAt)

	configLoader := s.configLoader
	if configLoader == nil {
		configLoader = func() (sms.Config, error) { return service.NewSMSConfigService().Get(false) }
	}
	settings, err := configLoader()
	if err != nil || !settings.Enabled || settings.Validate() != nil {
		return nil, e.NewBusinessError(e.SMSLoginDisabled)
	}
	if s.redis == nil {
		s.redis = data.RedisInitializedClient()
	}
	if s.redis == nil {
		return nil, e.NewDependencyNotReadyError()
	}
	secret, err := smsLoginSecret()
	if err != nil {
		return nil, e.NewBusinessError(e.SMSLoginDisabled)
	}
	fullPhone := strings.TrimPrefix(serviceNormalizeSMSPhone(countryCode, phone), "+")
	phoneKey := smsLoginHMAC(secret, fullPhone)
	if err := s.consumeRateLimit(ctx, "ip:"+smsLoginHMAC(secret, clientIP), settings.OTP.IPLimitPer10Minutes, 10*time.Minute); err != nil {
		return nil, err
	}

	challengeID := uuid.NewString()
	codeGenerator := s.codeGenerator
	if codeGenerator == nil {
		codeGenerator = secureSMSCode
	}
	code, err := codeGenerator()
	if err != nil {
		return nil, err
	}
	userID := uint(0)
	userFinder := s.userFinder
	if userFinder == nil {
		userFinder = findSMSLoginUser
	}
	user, lookupErr := userFinder(fullPhone)
	if lookupErr == nil {
		userID = user.ID
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) && log.Logger != nil {
		log.Logger.Warn("查询短信登录用户失败", zap.Error(lookupErr))
	}

	if userID != 0 {
		if err := s.reservePhoneSend(ctx, phoneKey, settings.OTP.PhoneDailyLimit, settings.OTP.GlobalDailyLimit, settings.OTP.ResendIntervalSeconds); err != nil {
			return nil, err
		}
	} else if err := s.reserveUnknownPhoneAttempt(ctx, phoneKey, settings.OTP.PhoneDailyLimit, settings.OTP.GlobalDailyLimit, settings.OTP.ResendIntervalSeconds); err != nil {
		return nil, err
	}

	challengeKey := smsChallengePrefix + challengeID
	values := map[string]any{
		"code_hash": smsLoginHMAC(secret, challengeID+":"+code),
		"phone_key": phoneKey,
		"user_id":   userID,
		"attempts":  0,
	}
	pipe := s.redis.TxPipeline()
	pipe.HSet(ctx, challengeKey, values)
	pipe.Expire(ctx, challengeKey, time.Duration(settings.OTP.TTLSeconds)*time.Second)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, e.NewDependencyNotReadyError()
	}

	// 未匹配到唯一有效用户时保持统一成功响应，避免暴露手机号是否存在。
	if userID != 0 {
		senderFactory := s.senderFactory
		if senderFactory == nil {
			senderFactory = func(settings sms.Config) (smsLoginSender, error) { return sms.NewDispatcher(settings, nil) }
		}
		sender, dispatchErr := senderFactory(settings)
		result := sms.SendResult{State: sms.SendStateFailed}
		if dispatchErr == nil {
			result, dispatchErr = sender.Send(ctx, sms.SceneLoginOTP, "+"+fullPhone, map[string]string{"code": code})
		}
		if dispatchErr != nil && log.Logger != nil {
			log.Logger.Warn("短信验证码发送失败", zap.Error(dispatchErr), zap.String("scene", sms.SceneLoginOTP))
		}
		if dispatchErr != nil && result.State != sms.SendStateUnknown {
			_ = s.redis.Del(ctx, challengeKey).Err()
		}
	}
	return &SMSCodeResult{ChallengeID: challengeID, RetryAfter: settings.OTP.ResendIntervalSeconds, ExpiresIn: settings.OTP.TTLSeconds}, nil
}

func (s *SMSLoginService) equalizeSendCodeResponseTime(ctx context.Context, startedAt time.Time) {
	if s.skipResponseDelay {
		return
	}
	wait := smsSendCodeMinimumDuration - time.Since(startedAt)
	if wait <= 0 {
		return
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// reserveUnknownPhoneAttempt 使用独立的隐私节流计数保持不存在手机号与真实账号相同的
// 冷却/单手机号日限额语义，但绝不占用真实短信的 per-phone/global 发送预算。
func (s *SMSLoginService) reserveUnknownPhoneAttempt(ctx context.Context, phoneKey string, phoneDailyLimit, globalDailyLimit, cooldownSeconds int) error {
	const script = `
local cooldown=KEYS[1]
local probe_daily=KEYS[2]
if redis.call('EXISTS',cooldown)==1 then return 0 end
local used=tonumber(redis.call('GET',probe_daily) or '0')
local global_used=tonumber(redis.call('GET',KEYS[3]) or '0')
if used>=tonumber(ARGV[1]) or global_used>=tonumber(ARGV[2]) then return -1 end
used=redis.call('INCR',probe_daily)
if used==1 then redis.call('EXPIRE',probe_daily,86400) end
redis.call('SET',cooldown,'1','EX',ARGV[3])
return 1`
	result, err := s.redis.Eval(ctx, script, []string{
		"auth:sms:cooldown:" + phoneKey,
		"auth:sms:probe:phone-day:" + phoneKey,
		"sms:budget:global-day",
	}, phoneDailyLimit, globalDailyLimit, cooldownSeconds).Int()
	if err != nil {
		return e.NewDependencyNotReadyError()
	}
	if result != 1 {
		return e.NewBusinessError(e.TooManyRequests, "验证码发送过于频繁，请稍后重试")
	}
	return nil
}

func (s *SMSLoginService) Login(ctx context.Context, countryCode, phone, challengeID, code string, logInfo LoginLogInfo) (*TokenResponse, error) {
	settings, err := service.NewSMSConfigService().Get(false)
	if err != nil || !settings.Enabled || settings.Validate() != nil {
		return nil, e.NewBusinessError(e.SMSLoginDisabled)
	}
	if s.redis == nil {
		s.redis = data.RedisInitializedClient()
	}
	if s.redis == nil {
		return nil, e.NewDependencyNotReadyError()
	}
	secret, err := smsLoginSecret()
	if err != nil {
		return nil, e.NewBusinessError(e.SMSLoginDisabled)
	}
	fullPhone := strings.TrimPrefix(serviceNormalizeSMSPhone(countryCode, phone), "+")
	phoneKey := smsLoginHMAC(secret, fullPhone)
	userID, err := consumeSMSChallenge(ctx, s.redis, challengeID, phoneKey, smsLoginHMAC(secret, challengeID+":"+code), settings.OTP.MaxAttempts)
	if err != nil || userID == 0 {
		return nil, e.NewBusinessError(e.SMSCredentialInvalid)
	}
	user, err := loadSMSLoginUser(userID, fullPhone)
	if err != nil {
		return nil, e.NewBusinessError(e.SMSCredentialInvalid)
	}
	return NewLoginService().LoginWithVerifiedUser(user, logInfo, model.LoginTypeSMS)
}

func (s *SMSLoginService) reservePhoneSend(ctx context.Context, phoneKey string, phoneDailyLimit, globalDailyLimit, cooldownSeconds int) error {
	const script = `
local cooldown=KEYS[1]
local daily=KEYS[2]
local global=KEYS[3]
if redis.call('EXISTS',cooldown)==1 then return 0 end
local used=tonumber(redis.call('GET',daily) or '0')
local global_used=tonumber(redis.call('GET',global) or '0')
if used>=tonumber(ARGV[1]) or global_used>=tonumber(ARGV[2]) then return -1 end
used=redis.call('INCR',daily)
if used==1 then redis.call('EXPIRE',daily,86400) end
global_used=redis.call('INCR',global)
if global_used==1 then redis.call('EXPIRE',global,86400) end
redis.call('SET',cooldown,'1','EX',ARGV[3])
return 1`
	result, err := s.redis.Eval(ctx, script, []string{"auth:sms:cooldown:" + phoneKey, "auth:sms:limit:phone-day:" + phoneKey, "sms:budget:global-day"}, phoneDailyLimit, globalDailyLimit, cooldownSeconds).Int()
	if err != nil {
		return e.NewDependencyNotReadyError()
	}
	if result != 1 {
		return e.NewBusinessError(e.TooManyRequests, "验证码发送过于频繁，请稍后重试")
	}
	return nil
}

func (s *SMSLoginService) consumeRateLimit(ctx context.Context, suffix string, limit int, ttl time.Duration) error {
	if limit <= 0 {
		return nil
	}
	const script = `local n=redis.call('INCR',KEYS[1]); if n==1 then redis.call('EXPIRE',KEYS[1],ARGV[2]); end; if n>tonumber(ARGV[1]) then return 0 else return 1 end`
	allowed, err := s.redis.Eval(ctx, script, []string{"auth:sms:limit:" + suffix}, limit, int(ttl.Seconds())).Int()
	if err != nil {
		return e.NewDependencyNotReadyError()
	}
	if allowed != 1 {
		return e.NewBusinessError(e.TooManyRequests, "验证码发送次数已达上限，请稍后重试")
	}
	return nil
}

func consumeSMSChallenge(ctx context.Context, client *redis.Client, challengeID, phoneKey, expectedHash string, maxAttempts int) (uint, error) {
	const script = `
local key=KEYS[1]
if redis.call('EXISTS',key)==0 then return {-1,0} end
if redis.call('HGET',key,'phone_key')~=ARGV[1] then return {-2,0} end
local attempts=tonumber(redis.call('HGET',key,'attempts') or '0')
if attempts>=tonumber(ARGV[3]) then redis.call('DEL',key); return {-3,0} end
if redis.call('HGET',key,'code_hash')~=ARGV[2] then
  attempts=redis.call('HINCRBY',key,'attempts',1)
  if attempts>=tonumber(ARGV[3]) then redis.call('DEL',key) end
  return {0,0}
end
local uid=tonumber(redis.call('HGET',key,'user_id') or '0')
redis.call('DEL',key)
return {1,uid}`
	values, err := client.Eval(ctx, script, []string{smsChallengePrefix + challengeID}, phoneKey, expectedHash, maxAttempts).Int64Slice()
	if err != nil || len(values) != 2 || values[0] != 1 {
		return 0, err
	}
	return uint(values[1]), nil
}

func secureSMSCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func smsLoginHMAC(secret, value string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func smsLoginSecret() (string, error) {
	return credential.DeriveKey(credential.PurposeSMSOTP)
}

func findSMSLoginUser(fullPhone string) (*model.AdminUser, error) {
	db, err := model.GetDB()
	if err != nil {
		return nil, err
	}
	var users []model.AdminUser
	if err := db.Where("full_phone_number = ? AND status = ? AND deleted_at = 0", fullPhone, model.AdminUserStatusEnabled).Limit(2).Find(&users).Error; err != nil {
		return nil, err
	}
	if len(users) != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	return &users[0], nil
}

func loadSMSLoginUser(userID uint, fullPhone string) (*model.AdminUser, error) {
	db, err := model.GetDB()
	if err != nil {
		return nil, err
	}
	user := model.NewAdminUsers()
	if err := db.Where("id = ? AND full_phone_number = ? AND status = ? AND deleted_at = 0", userID, fullPhone, model.AdminUserStatusEnabled).Take(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func serviceNormalizeSMSPhone(countryCode, phone string) string {
	return "+" + strings.TrimPrefix(strings.TrimSpace(countryCode), "+") + strings.TrimSpace(phone)
}
