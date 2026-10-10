package admin

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
	"github.com/wannanbigpig/gin-layout/internal/sms"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

type profileOTPStub struct {
	consumeCalls int
	consumeErr   error
	purpose      string
	phone        string
	challenge    string
	code         string
}

func (*profileOTPStub) Send(context.Context, uint, string, string, string, string, sms.Config) (*auth.SMSCodeResult, error) {
	return &auth.SMSCodeResult{}, nil
}

func (s *profileOTPStub) Consume(_ context.Context, uid uint, purpose, countryCode, phone, challengeID, code string, _ sms.Config) error {
	s.consumeCalls++
	if uid != 7 || purpose != s.purpose || countryCode != "86" || phone != s.phone || challengeID != s.challenge || code != s.code {
		return errors.New("unexpected challenge binding")
	}
	return s.consumeErr
}

func TestChangePhoneRequiresPasswordAndConditionalNewPhoneOTP(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE admin_user (id integer primary key, password text, phone_number text, country_code text, full_phone_number text, status integer, updated_at datetime, deleted_at integer not null default 0)`).Error; err != nil {
		t.Fatal(err)
	}
	hash, err := utils2.PasswordHash("current-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO admin_user (id,password,phone_number,country_code,full_phone_number,status,deleted_at) VALUES (7,?,?,?, ?,1,0)`, hash, "13800138000", "86", "8613800138000").Error; err != nil {
		t.Fatal(err)
	}
	previousOTP := newProfileOTPService
	stub := &profileOTPStub{purpose: auth.ProfileOTPPhone, phone: "13900139000", challenge: "challenge", code: "123456"}
	newProfileOTPService = func() profileOTPClient { return stub }
	previousPrepare, previousComplete := prepareUserTokenRevocationFunc, completeUserTokenRevocationFunc
	prepareCalls, completeCalls := 0, 0
	prepareUserTokenRevocationFunc = func(_ *gorm.DB, _ uint, _ uint8, _ string) (*auth.PreparedUserTokenRevocation, error) {
		prepareCalls++
		return &auth.PreparedUserTokenRevocation{}, nil
	}
	completeUserTokenRevocationFunc = func(_ *auth.PreparedUserTokenRevocation) error {
		completeCalls++
		return nil
	}
	t.Cleanup(func() {
		newProfileOTPService = previousOTP
		prepareUserTokenRevocationFunc, completeUserTokenRevocationFunc = previousPrepare, previousComplete
	})
	svc := NewAdminUserService()
	params := &form.ChangeProfilePhone{CountryCode: "86", PhoneNumber: "13900139000", CurrentPassword: "wrong-password", SMSChallengeID: "challenge", SMSCode: "123456"}
	if err := svc.changePhoneWithDB(db, 7, params, sms.Config{}); err == nil {
		t.Fatal("wrong current password accepted")
	}
	params.CurrentPassword = "current-password"
	stub.consumeErr = e.NewBusinessError(e.InvalidParameter, "验证码错误")
	if err := svc.changePhoneWithDB(db, 7, params, sms.Config{Enabled: true}); err == nil {
		t.Fatal("wrong new-phone code accepted")
	}
	var currentPhone string
	if err := db.Table("admin_user").Where("id = 7").Pluck("full_phone_number", &currentPhone).Error; err != nil || currentPhone != "8613800138000" {
		t.Fatalf("rejected change modified phone: %q %v", currentPhone, err)
	}
	stub.consumeErr = nil
	if err := svc.changePhoneWithDB(db, 7, params, sms.Config{Enabled: true}); err != nil {
		t.Fatalf("valid phone change failed: %v", err)
	}
	if stub.consumeCalls != 2 || prepareCalls != 1 || completeCalls != 1 {
		t.Fatalf("unexpected OTP or revocation calls: otp=%d prepare=%d complete=%d", stub.consumeCalls, prepareCalls, completeCalls)
	}
	if err := db.Table("admin_user").Where("id = 7").Pluck("full_phone_number", &currentPhone).Error; err != nil || currentPhone != "8613900139000" {
		t.Fatalf("new phone was not persisted: %q %v", currentPhone, err)
	}
	params.PhoneNumber = "13700137000"
	params.SMSChallengeID = ""
	params.SMSCode = ""
	if err := svc.changePhoneWithDB(db, 7, params, sms.Config{}); err != nil {
		t.Fatalf("disabled SMS should not require OTP: %v", err)
	}
	if stub.consumeCalls != 2 {
		t.Fatal("disabled SMS unexpectedly consumed an OTP")
	}
}

func TestProfileSecurityOptionsFailsClosedOnConfigError(t *testing.T) {
	previous := profileSMSSettings
	profileSMSSettings = func() (sms.Config, error) { return sms.Config{}, errors.New("config unavailable") }
	t.Cleanup(func() { profileSMSSettings = previous })
	if _, err := NewAdminUserService().ProfileSecurityOptions(); err == nil {
		t.Fatal("config failure must not report verification disabled")
	}
	if err := NewAdminUserService().ChangePhone(7, &form.ChangeProfilePhone{CountryCode: "86", PhoneNumber: "13900139000"}); err == nil {
		t.Fatal("config failure must block phone change")
	}
}

func TestUpdateProfilePasswordRequiresBoundPhoneCodeWhenEnabled(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE admin_user (id integer primary key, password text, phone_number text, country_code text, updated_at datetime, deleted_at integer not null default 0)`).Error; err != nil {
		t.Fatal(err)
	}
	hash, err := utils2.PasswordHash("current-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO admin_user (id,password,phone_number,country_code,deleted_at) VALUES (7,?,?,?,0)`, hash, "13800138000", "86").Error; err != nil {
		t.Fatal(err)
	}
	previousSettings, previousOTP := profileSMSSettings, newProfileOTPService
	profileSMSSettings = func() (sms.Config, error) { return sms.Config{Enabled: true}, nil }
	stub := &profileOTPStub{purpose: auth.ProfileOTPPassword, phone: "13800138000", consumeErr: e.NewBusinessError(e.InvalidParameter, "短信验证码无效")}
	newProfileOTPService = func() profileOTPClient { return stub }
	t.Cleanup(func() {
		profileSMSSettings, newProfileOTPService = previousSettings, previousOTP
	})
	currentPassword, newPassword := "current-password", "new-password"
	params := &form.UpdateProfile{CurrentPassword: &currentPassword, Password: &newPassword}
	if err := NewAdminUserService().updateProfileWithDB(db, 7, params); err == nil {
		t.Fatal("password changed without bound-phone verification")
	}
	if stub.consumeCalls != 1 {
		t.Fatalf("expected bound-phone OTP check, got %d", stub.consumeCalls)
	}
	var stored string
	if err := db.Table("admin_user").Where("id = 7").Pluck("password", &stored).Error; err != nil || stored != hash {
		t.Fatalf("rejected password change updated hash: %v", err)
	}
}
