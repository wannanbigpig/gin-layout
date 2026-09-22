package todo

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

// ReminderPreference 是待办到期提醒的用户级渠道偏好。
type ReminderPreference struct {
	InAppEnabled bool `json:"in_app_enabled"`
	SMSEnabled   bool `json:"sms_enabled"`
	SMSAvailable bool `json:"sms_available"`
}

// GetReminderPreference 返回有效偏好；未创建记录时默认开启站内、关闭短信。
func (s *Service) GetReminderPreference(ctx context.Context, userID uint) (*ReminderPreference, error) {
	if userID == 0 {
		return nil, e.NewBusinessError(e.NotLogin)
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	preference := model.NewNotificationPreference()
	preference.SetDB(db)
	preferenceErr := db.Where("user_id = ? AND scene = ?", userID, model.NotificationSceneTodoDue).First(preference).Error
	if preferenceErr != nil && !errors.Is(preferenceErr, gorm.ErrRecordNotFound) {
		return nil, preferenceErr
	}
	phoneAvailable, err := reminderSMSPhoneAvailable(db, userID)
	if err != nil {
		return nil, err
	}
	if errors.Is(preferenceErr, gorm.ErrRecordNotFound) || preference.ID == 0 {
		return &ReminderPreference{InAppEnabled: true, SMSAvailable: phoneAvailable}, nil
	}
	return &ReminderPreference{
		InAppEnabled: preference.InAppEnabled == 1,
		SMSEnabled:   preference.SMSEnabled == 1,
		SMSAvailable: phoneAvailable,
	}, nil
}

// UpdateReminderPreference 按当前用户 + todo_due 场景幂等更新渠道偏好。
func (s *Service) UpdateReminderPreference(ctx context.Context, userID uint, inAppEnabled, smsEnabled bool) (*ReminderPreference, error) {
	if userID == 0 {
		return nil, e.NewBusinessError(e.NotLogin)
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	phoneAvailable, err := reminderSMSPhoneAvailable(db, userID)
	if err != nil {
		return nil, err
	}
	if smsEnabled && !phoneAvailable {
		return nil, e.NewBusinessError(e.InvalidParameter, "开启短信提醒前请先绑定手机号")
	}
	preference := &model.NotificationPreference{
		UserID:       userID,
		Scene:        model.NotificationSceneTodoDue,
		InAppEnabled: boolToUint8(inAppEnabled),
		SMSEnabled:   boolToUint8(smsEnabled),
	}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "scene"}},
		DoUpdates: clause.AssignmentColumns([]string{"in_app_enabled", "sms_enabled", "updated_at"}),
	}).Create(preference).Error; err != nil {
		return nil, err
	}
	return &ReminderPreference{InAppEnabled: inAppEnabled, SMSEnabled: smsEnabled, SMSAvailable: phoneAvailable}, nil
}

func reminderSMSPhoneAvailable(db *gorm.DB, userID uint) (bool, error) {
	var fullPhone string
	err := db.Model(&model.AdminUser{}).
		Select("full_phone_number").
		Where("id = ? AND status = ? AND deleted_at = 0", userID, model.AdminUserStatusEnabled).
		Scan(&fullPhone).Error
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(fullPhone) != "", nil
}

func boolToUint8(value bool) uint8 {
	if value {
		return 1
	}
	return 0
}
