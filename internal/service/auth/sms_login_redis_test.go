package auth

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/sms"
)

type fakeSMSLoginSender struct {
	calls atomic.Int32
}

func (f *fakeSMSLoginSender) Send(context.Context, string, string, map[string]string) (sms.SendResult, error) {
	f.calls.Add(1)
	return sms.SendResult{State: sms.SendStateAccepted}, nil
}

type failedSMSLoginSender struct{}

func (*failedSMSLoginSender) Send(context.Context, string, string, map[string]string) (sms.SendResult, error) {
	return sms.SendResult{State: sms.SendStateFailed}, errors.New("provider rejected request")
}

func smsTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", DB: 15, DialTimeout: 200 * time.Millisecond, ReadTimeout: time.Second, WriteTimeout: time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Skipf("Redis integration test requires localhost:6379: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestReservePhoneSendDoesNotSpendDailyQuotaDuringCooldown(t *testing.T) {
	client := smsTestRedis(t)
	ctx := context.Background()
	phoneKey := uuid.NewString()
	keys := []string{"auth:sms:cooldown:" + phoneKey, "auth:sms:limit:phone-day:" + phoneKey, "sms:budget:global-day"}
	oldGlobal, oldGlobalErr := client.Get(ctx, keys[2]).Result()
	oldGlobalTTL := client.TTL(ctx, keys[2]).Val()
	if err := client.Del(ctx, keys...).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = client.Del(ctx, keys...).Err()
		if oldGlobalErr == nil {
			if oldGlobalTTL > 0 {
				_ = client.Set(ctx, keys[2], oldGlobal, oldGlobalTTL).Err()
			} else {
				_ = client.Set(ctx, keys[2], oldGlobal, 0).Err()
			}
		}
	})

	service := &SMSLoginService{redis: client}
	if err := service.reservePhoneSend(ctx, phoneKey, 2, 100, 60); err != nil {
		t.Fatalf("first reservation failed: %v", err)
	}
	if err := service.reservePhoneSend(ctx, phoneKey, 2, 100, 60); err == nil {
		t.Fatal("cooldown request must be rejected")
	}
	if got := client.Get(ctx, keys[1]).Val(); got != "1" {
		t.Fatalf("cooldown rejection spent phone quota: %s", got)
	}
	if err := client.Del(ctx, keys[0]).Err(); err != nil {
		t.Fatal(err)
	}
	if err := service.reservePhoneSend(ctx, phoneKey, 2, 100, 60); err != nil {
		t.Fatalf("second actual reservation failed: %v", err)
	}
	if err := client.Del(ctx, keys[0]).Err(); err != nil {
		t.Fatal(err)
	}
	if err := service.reservePhoneSend(ctx, phoneKey, 2, 100, 60); err == nil {
		t.Fatal("phone daily limit must reject the third actual reservation")
	}
}

func TestConsumeSMSChallengeAllowsOneConcurrentSuccess(t *testing.T) {
	client := smsTestRedis(t)
	ctx := context.Background()
	challengeID := uuid.NewString()
	key := smsChallengePrefix + challengeID
	phoneKey := "phone-" + uuid.NewString()
	expectedHash := "hash-" + uuid.NewString()
	if err := client.HSet(ctx, key, map[string]any{"phone_key": phoneKey, "code_hash": expectedHash, "user_id": 42, "attempts": 0}).Err(); err != nil {
		t.Fatal(err)
	}
	if err := client.Expire(ctx, key, time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Del(ctx, key).Err() })

	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			uid, err := consumeSMSChallenge(ctx, client, challengeID, phoneKey, expectedHash, 5)
			if err == nil && uid == 42 {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := successes.Load(); got != 1 {
		t.Fatalf("expected one successful OTP consumption, got %d", got)
	}
}

func TestSendCodeUnknownPhoneDoesNotConsumeRealSMSBudget(t *testing.T) {
	client := smsTestRedis(t)
	ctx := context.Background()
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) { cfg.Security.CredentialEncryptionKey = "sms-login-budget-test-master-key-32" })
	t.Cleanup(restoreConfig)
	settings := validSMSLoginTestConfig(t)
	phone := "1390000" + uuid.NewString()[:4]
	fullPhone := "86" + phone
	secret, err := smsLoginSecret()
	if err != nil {
		t.Fatal(err)
	}
	phoneKey := smsLoginHMAC(secret, fullPhone)
	clientIP := "192.0.2." + uuid.NewString()[:2]
	keys := []string{
		"auth:sms:cooldown:" + phoneKey,
		"auth:sms:probe:phone-day:" + phoneKey,
		"auth:sms:limit:phone-day:" + phoneKey,
		"auth:sms:limit:ip:" + smsLoginHMAC(secret, clientIP),
	}
	cleanupGlobalSMSBudget(t, client, append(keys, "sms:budget:global-day")...)
	sender := &fakeSMSLoginSender{}
	svc := &SMSLoginService{
		redis:             client,
		skipResponseDelay: true,
		configLoader:      func() (sms.Config, error) { return settings, nil },
		userFinder:        func(string) (*model.AdminUser, error) { return nil, gorm.ErrRecordNotFound },
		senderFactory: func(sms.Config) (smsLoginSender, error) {
			return sender, nil
		},
		codeGenerator: func() (string, error) { return "123456", nil },
	}
	result, err := svc.SendCode(ctx, "86", phone, clientIP)
	if err != nil {
		t.Fatalf("unknown phone must keep generic success response: %v", err)
	}
	t.Cleanup(func() { _ = client.Del(ctx, smsChallengePrefix+result.ChallengeID).Err() })
	if sender.calls.Load() != 0 {
		t.Fatalf("unknown phone must not call sender, calls=%d", sender.calls.Load())
	}
	if got := client.Exists(ctx, "auth:sms:limit:phone-day:"+phoneKey, "sms:budget:global-day").Val(); got != 0 {
		t.Fatalf("unknown phone consumed real SMS budget keys, existing=%d", got)
	}
	if got := client.Get(ctx, "auth:sms:probe:phone-day:"+phoneKey).Val(); got != "1" {
		t.Fatalf("unknown phone must retain privacy throttle semantics, got %q", got)
	}
	if _, err := svc.SendCode(ctx, "86", phone, clientIP); err == nil {
		t.Fatal("unknown phone must retain the same cooldown response as a known phone")
	} else {
		var businessErr *e.BusinessError
		if !errors.As(err, &businessErr) || businessErr.GetCode() != e.TooManyRequests {
			t.Fatalf("unexpected unknown-phone cooldown error: %T %v", err, err)
		}
	}
	if got := client.Exists(ctx, "auth:sms:limit:phone-day:"+phoneKey, "sms:budget:global-day").Val(); got != 0 {
		t.Fatalf("unknown-phone cooldown consumed real SMS budget keys, existing=%d", got)
	}
}

func TestSendCodeKnownPhoneReservesBudgetAndCallsSender(t *testing.T) {
	client := smsTestRedis(t)
	ctx := context.Background()
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) { cfg.Security.CredentialEncryptionKey = "sms-login-send-test-master-key-32" })
	t.Cleanup(restoreConfig)
	settings := validSMSLoginTestConfig(t)
	phone := "1380000" + uuid.NewString()[:4]
	fullPhone := "86" + phone
	secret, err := smsLoginSecret()
	if err != nil {
		t.Fatal(err)
	}
	phoneKey := smsLoginHMAC(secret, fullPhone)
	clientIP := "198.51.100." + uuid.NewString()[:2]
	keys := []string{
		"auth:sms:cooldown:" + phoneKey,
		"auth:sms:limit:phone-day:" + phoneKey,
		"auth:sms:limit:ip:" + smsLoginHMAC(secret, clientIP),
	}
	cleanupGlobalSMSBudget(t, client, append(keys, "sms:budget:global-day")...)
	sender := &fakeSMSLoginSender{}
	svc := &SMSLoginService{
		redis:             client,
		skipResponseDelay: true,
		configLoader:      func() (sms.Config, error) { return settings, nil },
		userFinder: func(string) (*model.AdminUser, error) {
			return &model.AdminUser{ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 7}}}, nil
		},
		senderFactory: func(sms.Config) (smsLoginSender, error) {
			return sender, nil
		},
		codeGenerator: func() (string, error) { return "123456", nil },
	}
	result, err := svc.SendCode(ctx, "86", phone, clientIP)
	if err != nil {
		t.Fatalf("known phone send: %v", err)
	}
	t.Cleanup(func() { _ = client.Del(ctx, smsChallengePrefix+result.ChallengeID).Err() })
	if sender.calls.Load() != 1 {
		t.Fatalf("known phone sender calls=%d, want 1", sender.calls.Load())
	}
	if got := client.Get(ctx, "auth:sms:limit:phone-day:"+phoneKey).Val(); got != "1" {
		t.Fatalf("known phone budget=%q, want 1", got)
	}
	if got := client.Get(ctx, "sms:budget:global-day").Val(); got != "1" {
		t.Fatalf("global budget=%q, want 1", got)
	}
}

func TestSendCodeGlobalBudgetLimitDoesNotRevealWhetherPhoneExists(t *testing.T) {
	client := smsTestRedis(t)
	ctx := context.Background()
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Security.CredentialEncryptionKey = "sms-global-budget-test-master-key-32"
	})
	t.Cleanup(restoreConfig)
	settings := validSMSLoginTestConfig(t)
	globalKey := "sms:budget:global-day"
	cleanupGlobalSMSBudget(t, client, globalKey)
	if err := client.Set(ctx, globalKey, settings.OTP.GlobalDailyLimit, time.Hour).Err(); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name   string
		phone  string
		finder func(string) (*model.AdminUser, error)
	}{
		{name: "known", phone: "13800001111", finder: func(string) (*model.AdminUser, error) {
			return &model.AdminUser{ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 8}}}, nil
		}},
		{name: "unknown", phone: "13900002222", finder: func(string) (*model.AdminUser, error) {
			return nil, gorm.ErrRecordNotFound
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &SMSLoginService{
				redis:             client,
				skipResponseDelay: true,
				configLoader:      func() (sms.Config, error) { return settings, nil },
				userFinder:        tc.finder,
				codeGenerator:     func() (string, error) { return "123456", nil },
			}
			_, err := svc.SendCode(ctx, "86", tc.phone, "203.0.113."+uuid.NewString()[:4])
			var businessErr *e.BusinessError
			if !errors.As(err, &businessErr) || businessErr.GetCode() != e.TooManyRequests {
				t.Fatalf("expected identical TooManyRequests at global limit, got %T %v", err, err)
			}
		})
	}
}

func TestSendCodeProviderFailureKeepsCooldown(t *testing.T) {
	client := smsTestRedis(t)
	ctx := context.Background()
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Security.CredentialEncryptionKey = "sms-provider-failure-test-master-key-32"
	})
	t.Cleanup(restoreConfig)
	settings := validSMSLoginTestConfig(t)
	phone := "13700003333"
	clientIP := "203.0.113." + uuid.NewString()[:4]
	secret, err := smsLoginSecret()
	if err != nil {
		t.Fatal(err)
	}
	phoneKey := smsLoginHMAC(secret, "86"+phone)
	keys := []string{
		"auth:sms:cooldown:" + phoneKey,
		"auth:sms:limit:phone-day:" + phoneKey,
		"sms:budget:global-day",
	}
	cleanupGlobalSMSBudget(t, client, keys...)
	svc := &SMSLoginService{
		redis:             client,
		skipResponseDelay: true,
		configLoader:      func() (sms.Config, error) { return settings, nil },
		userFinder: func(string) (*model.AdminUser, error) {
			return &model.AdminUser{ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 9}}}, nil
		},
		senderFactory: func(sms.Config) (smsLoginSender, error) { return &failedSMSLoginSender{}, nil },
		codeGenerator: func() (string, error) { return "123456", nil },
	}
	result, err := svc.SendCode(ctx, "86", phone, clientIP)
	if err != nil {
		t.Fatalf("provider failure should retain generic response: %v", err)
	}
	if got := client.Exists(ctx, "auth:sms:cooldown:"+phoneKey).Val(); got != 1 {
		t.Fatalf("provider failure removed cooldown, exists=%d", got)
	}
	if got := client.Exists(ctx, smsChallengePrefix+result.ChallengeID).Val(); got != 0 {
		t.Fatalf("failed delivery left a usable challenge, exists=%d", got)
	}
	if _, err := svc.SendCode(ctx, "86", phone, clientIP); err == nil {
		t.Fatal("provider failure retry must keep the same cooldown response")
	}
}

func validSMSLoginTestConfig(t *testing.T) sms.Config {
	t.Helper()
	settings := sms.DefaultConfig()
	settings.Enabled = true
	settings.Providers[sms.ProviderAliyun] = json.RawMessage(`{"enabled":true,"access_key_id":"id","access_key_secret":"secret","endpoint":"https://dysmsapi.aliyuncs.com"}`)
	template := settings.Templates[sms.ProviderAliyun]["login_otp"]
	template.SignName = "Gin Layout"
	template.TemplateCode = "SMS_LOGIN"
	settings.Templates[sms.ProviderAliyun]["login_otp"] = template
	if err := settings.Validate(); err != nil {
		t.Fatalf("valid SMS test config: %v", err)
	}
	return settings
}

func cleanupGlobalSMSBudget(t *testing.T, client *redis.Client, keys ...string) {
	t.Helper()
	ctx := context.Background()
	globalKey := "sms:budget:global-day"
	oldGlobal, oldGlobalErr := client.Get(ctx, globalKey).Result()
	oldGlobalTTL := client.TTL(ctx, globalKey).Val()
	if err := client.Del(ctx, keys...).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = client.Del(ctx, keys...).Err()
		if oldGlobalErr == nil {
			if oldGlobalTTL > 0 {
				_ = client.Set(ctx, globalKey, oldGlobal, oldGlobalTTL).Err()
			} else {
				_ = client.Set(ctx, globalKey, oldGlobal, 0).Err()
			}
		}
	})
}
