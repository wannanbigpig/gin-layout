package admin

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
	"github.com/wannanbigpig/gin-layout/internal/sms"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

var profileSMSSettings = auth.ProfileSMSSettings

type profileOTPClient interface {
	Send(context.Context, uint, string, string, string, string, sms.Config) (*auth.SMSCodeResult, error)
	Consume(context.Context, uint, string, string, string, string, string, sms.Config) error
}

var newProfileOTPService = func() profileOTPClient { return auth.NewProfileOTPService() }

func (s *AdminUserService) ProfileSecurityOptions() (map[string]bool, error) {
	settings, err := profileSMSSettings()
	if err != nil {
		return nil, err
	}
	return map[string]bool{"sms_verification_required": settings.Enabled}, nil
}

// SendProfileCode sends a code to the new phone for binding, or to the bound
// phone for a password change. It never treats a login challenge as proof.
func (s *AdminUserService) SendProfileCode(ctx context.Context, uid uint, params *form.ProfileCodeRequest, clientIP string) (*auth.SMSCodeResult, error) {
	if uid == 0 {
		return nil, e.NewBusinessError(e.NotLogin)
	}
	if params == nil {
		return nil, e.NewBusinessError(e.InvalidParameter)
	}
	settings, err := profileSMSSettings()
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		return nil, e.NewBusinessError(e.InvalidParameter, "当前未启用手机号登录，无需短信验证码")
	}
	user := model.NewAdminUsers()
	if err := user.GetById(uid); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.UserDoesNotExist)
		}
		return nil, err
	}
	var countryCode, phone string
	switch params.Purpose {
	case auth.ProfileOTPPhone:
		if params.CountryCode == "" || params.PhoneNumber == "" {
			return nil, e.NewBusinessError(e.InvalidParameter, "请填写新手机号")
		}
		if !utils2.ComparePasswords(user.Password, params.CurrentPassword) {
			return nil, e.NewBusinessError(e.CurrentPasswordWrong)
		}
		countryCode, phone = params.CountryCode, params.PhoneNumber
		if strings.TrimPrefix(countryCode, "+")+phone == user.FullPhoneNumber {
			return nil, e.NewBusinessError(e.InvalidParameter, "新手机号与当前绑定手机号相同")
		}
	case auth.ProfileOTPPassword:
		if user.PhoneNumber == "" {
			return nil, e.NewBusinessError(e.InvalidParameter, "请先绑定手机号再修改密码")
		}
		countryCode, phone = user.CountryCode, user.PhoneNumber
	default:
		return nil, e.NewBusinessError(e.InvalidParameter)
	}
	return newProfileOTPService().Send(ctx, uid, params.Purpose, countryCode, phone, clientIP, settings)
}

// ChangePhone is the only self-service path that can update the login phone.
func (s *AdminUserService) ChangePhone(uid uint, params *form.ChangeProfilePhone) error {
	if uid == 0 {
		return e.NewBusinessError(e.NotLogin)
	}
	if params == nil || params.PhoneNumber == "" || params.CountryCode == "" {
		return e.NewBusinessError(e.InvalidParameter)
	}
	settings, err := profileSMSSettings()
	if err != nil {
		return err
	}
	db, err := model.GetDB()
	if err != nil {
		return e.NewBusinessError(e.UpdateUserFailed)
	}
	return s.changePhoneWithDB(db, uid, params, settings)
}

func (s *AdminUserService) changePhoneWithDB(db *gorm.DB, uid uint, params *form.ChangeProfilePhone, settings sms.Config) error {
	newCountryCode := strings.TrimPrefix(strings.TrimSpace(params.CountryCode), "+")
	newFullPhone := newCountryCode + strings.TrimSpace(params.PhoneNumber)
	pendingRevocations := make([]userTokenRevocation, 0, 1)
	err := access.RunInTransaction(db, func(tx *gorm.DB) error {
		user := model.NewAdminUsers()
		user.SetDB(tx)
		if err := loadAdminUserForUpdate(tx, user, uid); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return e.NewBusinessError(e.UserDoesNotExist)
			}
			return err
		}
		if !utils2.ComparePasswords(user.Password, params.CurrentPassword) {
			return e.NewBusinessError(e.CurrentPasswordWrong)
		}
		if user.FullPhoneNumber == newFullPhone {
			return e.NewBusinessError(e.InvalidParameter, "新手机号与当前绑定手机号相同")
		}
		if err := s.validateUniqueFieldsWithLock(tx, map[string]interface{}{"full_phone_number": newFullPhone}, uid); err != nil {
			return err
		}
		if settings.Enabled {
			if err := newProfileOTPService().Consume(context.Background(), uid, auth.ProfileOTPPhone,
				newCountryCode, params.PhoneNumber, params.SMSChallengeID, params.SMSCode, settings); err != nil {
				return err
			}
		}
		if err := tx.Model(&model.AdminUser{}).Where("id = ? AND deleted_at = 0", uid).
			Updates(map[string]any{"phone_number": params.PhoneNumber, "country_code": newCountryCode, "full_phone_number": newFullPhone}).Error; err != nil {
			return err
		}
		if revocation := profileCredentialRevocation(uid, false, true); revocation != nil {
			if err := s.prepareUserTokenRevocation(tx, revocation); err != nil {
				return err
			}
			pendingRevocations = append(pendingRevocations, *revocation)
		}
		return nil
	})
	if err := s.handleMutationError(err, e.UpdateUserFailed); err != nil {
		return err
	}
	return s.handleMutationError(s.revokeUserTokensAfterCommit(pendingRevocations), e.UpdateUserFailed)
}
