package auth

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/sms"
)

func TestProfileOTPBoundToUserPurposePhoneAndOneUse(t *testing.T) {
	client := smsTestRedis(t)
	ctx := context.Background()
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Security.CredentialEncryptionKey = "sms-login-send-test-master-key-32"
	})
	t.Cleanup(restoreConfig)
	settings := validSMSLoginTestConfig(t)
	phone := "138" + uuid.NewString()[:8]
	secret, err := smsLoginSecret()
	if err != nil {
		t.Fatal(err)
	}
	phoneKey := smsLoginHMAC(secret, "86"+phone)
	clientIP := "203.0.113." + uuid.NewString()[:2]
	cleanupGlobalSMSBudget(t, client,
		"auth:sms:cooldown:"+phoneKey,
		"auth:sms:limit:phone-day:"+phoneKey,
		"auth:sms:limit:ip:"+smsLoginHMAC(secret, clientIP),
		"sms:budget:global-day")
	sender := &fakeSMSLoginSender{}
	svc := &ProfileOTPService{
		redis:         client,
		senderFactory: func(sms.Config) (smsLoginSender, error) { return sender, nil },
		codeGenerator: func() (string, error) { return "123456", nil },
	}
	challenge, err := svc.Send(ctx, 42, ProfileOTPPhone, "86", phone, clientIP, settings)
	if err != nil {
		t.Fatalf("send profile code: %v", err)
	}
	t.Cleanup(func() { _ = client.Del(ctx, profileOTPPrefix+challenge.ChallengeID).Err() })
	if sender.calls.Load() != 1 {
		t.Fatal("expected one provider send")
	}
	for _, attempt := range []struct {
		uid     uint
		purpose string
		phone   string
		code    string
	}{
		{43, ProfileOTPPhone, phone, "123456"},
		{42, ProfileOTPPassword, phone, "123456"},
		{42, ProfileOTPPhone, "13900139000", "123456"},
		{42, ProfileOTPPhone, phone, "000000"},
	} {
		if err := svc.Consume(ctx, attempt.uid, attempt.purpose, "86", attempt.phone, challenge.ChallengeID, attempt.code, settings); err == nil {
			t.Fatalf("mismatched challenge accepted: %+v", attempt)
		}
	}
	if err := svc.Consume(ctx, 42, ProfileOTPPhone, "86", phone, challenge.ChallengeID, "123456", settings); err != nil {
		t.Fatalf("correct challenge rejected: %v", err)
	}
	if err := svc.Consume(ctx, 42, ProfileOTPPhone, "86", phone, challenge.ChallengeID, "123456", settings); err == nil {
		t.Fatal("replayed profile code accepted")
	}
}
