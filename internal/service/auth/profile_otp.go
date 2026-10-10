package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/wannanbigpig/gin-layout/data"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/sms"
)

const (
	ProfileOTPPhone    = "phone"
	ProfileOTPPassword = "password"
	profileOTPPrefix   = "auth:sms:profile:challenge:"
)

// ProfileOTPService keeps account-change codes separate from login codes.
type ProfileOTPService struct {
	redis         *redis.Client
	senderFactory func(sms.Config) (smsLoginSender, error)
	codeGenerator func() (string, error)
}

func NewProfileOTPService() *ProfileOTPService { return &ProfileOTPService{} }

// ProfileSMSSettings is the security policy source. A config read or validation
// failure must never turn off verification for an account change.
func ProfileSMSSettings() (sms.Config, error) {
	settings, err := service.NewSMSConfigService().GetStrict(false)
	if err != nil {
		return sms.Config{}, err
	}
	if settings.Enabled {
		if err := settings.Validate(); err != nil {
			return sms.Config{}, err
		}
	}
	return settings, nil
}

func (s *ProfileOTPService) client() *redis.Client {
	if s.redis != nil {
		return s.redis
	}
	return data.RedisInitializedClient()
}

func (s *ProfileOTPService) Send(ctx context.Context, uid uint, purpose, countryCode, phone, clientIP string, settings sms.Config) (*SMSCodeResult, error) {
	if uid == 0 || (purpose != ProfileOTPPhone && purpose != ProfileOTPPassword) || !settings.Enabled {
		return nil, e.NewBusinessError(e.InvalidParameter)
	}
	client := s.client()
	if client == nil {
		return nil, e.NewDependencyNotReadyError()
	}
	secret, err := smsLoginSecret()
	if err != nil {
		return nil, err
	}
	fullPhone := serviceNormalizeSMSPhone(countryCode, phone)
	phoneKey := smsLoginHMAC(secret, strings.TrimPrefix(fullPhone, "+"))
	limiter := &SMSLoginService{redis: client}
	if err := limiter.consumeRateLimit(ctx, "ip:"+smsLoginHMAC(secret, clientIP), settings.OTP.IPLimitPer10Minutes, 10*time.Minute); err != nil {
		return nil, err
	}
	if err := limiter.reservePhoneSend(ctx, phoneKey, settings.OTP.PhoneDailyLimit, settings.OTP.GlobalDailyLimit, settings.OTP.ResendIntervalSeconds); err != nil {
		return nil, err
	}
	codeGenerator := s.codeGenerator
	if codeGenerator == nil {
		codeGenerator = secureSMSCode
	}
	code, err := codeGenerator()
	if err != nil {
		return nil, err
	}
	challengeID := uuid.NewString()
	key := profileOTPPrefix + challengeID
	values := map[string]any{
		"user_id": uid, "purpose": purpose, "phone_key": phoneKey,
		"code_hash": smsLoginHMAC(secret, challengeID+":"+code), "attempts": 0,
	}
	pipe := client.TxPipeline()
	pipe.HSet(ctx, key, values)
	pipe.Expire(ctx, key, time.Duration(settings.OTP.TTLSeconds)*time.Second)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, e.NewDependencyNotReadyError()
	}
	senderFactory := s.senderFactory
	if senderFactory == nil {
		senderFactory = func(settings sms.Config) (smsLoginSender, error) { return sms.NewDispatcher(settings, nil) }
	}
	sender, err := senderFactory(settings)
	result := sms.SendResult{State: sms.SendStateFailed}
	if err == nil {
		result, err = sender.Send(ctx, sms.SceneLoginOTP, fullPhone, map[string]string{"code": code})
	}
	if err != nil && result.State != sms.SendStateUnknown {
		_ = client.Del(ctx, key).Err()
		return nil, e.NewBusinessError(e.SMSProviderFailed)
	}
	return &SMSCodeResult{ChallengeID: challengeID, RetryAfter: settings.OTP.ResendIntervalSeconds, ExpiresIn: settings.OTP.TTLSeconds}, nil
}

func (s *ProfileOTPService) Consume(ctx context.Context, uid uint, purpose, countryCode, phone, challengeID, code string, settings sms.Config) error {
	if uid == 0 || (purpose != ProfileOTPPhone && purpose != ProfileOTPPassword) || !settings.Enabled ||
		strings.TrimSpace(challengeID) == "" || strings.TrimSpace(code) == "" {
		return e.NewBusinessError(e.InvalidParameter, "请先获取并填写短信验证码")
	}
	if _, err := uuid.Parse(challengeID); err != nil {
		return e.NewBusinessError(e.InvalidParameter, "短信验证码无效或已过期")
	}
	client := s.client()
	if client == nil {
		return e.NewDependencyNotReadyError()
	}
	secret, err := smsLoginSecret()
	if err != nil {
		return err
	}
	const script = `
local key=KEYS[1]
if redis.call('EXISTS',key)==0 then return 0 end
if redis.call('HGET',key,'user_id')~=ARGV[1] or redis.call('HGET',key,'purpose')~=ARGV[2] or redis.call('HGET',key,'phone_key')~=ARGV[3] then return 0 end
local attempts=tonumber(redis.call('HGET',key,'attempts') or '0')
if attempts>=tonumber(ARGV[5]) then redis.call('DEL',key); return 0 end
if redis.call('HGET',key,'code_hash')~=ARGV[4] then
  attempts=redis.call('HINCRBY',key,'attempts',1)
  if attempts>=tonumber(ARGV[5]) then redis.call('DEL',key) end
  return 0
end
redis.call('DEL',key)
return 1`
	phoneKey := smsLoginHMAC(secret, strings.TrimPrefix(serviceNormalizeSMSPhone(countryCode, phone), "+"))
	allowed, err := client.Eval(ctx, script, []string{profileOTPPrefix + challengeID},
		fmt.Sprint(uid), purpose, phoneKey, smsLoginHMAC(secret, challengeID+":"+code), settings.OTP.MaxAttempts).Int()
	if err != nil {
		return e.NewDependencyNotReadyError()
	}
	if allowed != 1 {
		return e.NewBusinessError(e.InvalidParameter, "短信验证码无效或已过期")
	}
	return nil
}
