package todo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	baseservice "github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/sms"
)

const (
	todoSMSBusinessType       = "user_todo"
	todoSMSMaxAttempts   uint = 3
	todoSMSBatchSize          = 100
	staleSMSSendingAfter      = 10 * time.Minute
	todoSMSRetryBase          = 5 * time.Minute
)

type reminderSMSPreference struct {
	InAppEnabled bool
	SMSEnabled   bool
	LeadMinutes  int
}

type reminderSMSSender interface {
	Send(context.Context, string, string, map[string]string) (sms.SendResult, error)
}

type reminderSMSRuntime struct {
	Paused           bool
	Sender           reminderSMSSender
	Provider         string
	GlobalDailyLimit int
}

type reminderSMSRuntimeFactory func() (reminderSMSRuntime, error)
type reminderSMSBudgetReserve func(context.Context, int) error

type reminderSMSProcessResult struct {
	Accepted int
	Failed   int
	Unknown  int
}

func loadReminderChannels(db *gorm.DB, userID uint) (reminderSMSPreference, string, error) {
	preference := reminderSMSPreference{InAppEnabled: true, LeadMinutes: DefaultReminderLeadMinutes}
	var row model.NotificationPreference
	err := db.Where("user_id = ? AND scene = ?", userID, model.NotificationSceneTodoDue).First(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return preference, "", err
	}
	if err == nil {
		preference.InAppEnabled = row.InAppEnabled == 1
		preference.SMSEnabled = row.SMSEnabled == 1
		preference.LeadMinutes = effectiveReminderLeadMinutes(row.LeadMinutes)
	}

	var user model.AdminUser
	if err := db.Select("id", "full_phone_number").
		Where("id = ? AND status = ? AND deleted_at = 0", userID, model.AdminUserStatusEnabled).
		First(&user).Error; err != nil {
		return preference, "", err
	}
	return preference, normalizeReminderPhone(user.FullPhoneNumber), nil
}

func persistTodoSMSDelivery(tx *gorm.DB, item *model.UserTodo, now time.Time) (bool, error) {
	if item == nil || item.DueAt == nil {
		return false, nil
	}
	reminderType := todoReminderType(item.DueAt.Time, now)
	variables, err := json.Marshal(map[string]string{
		"title":  truncateSMSVariable(item.Title, 30),
		"due_at": item.DueAt.Time.Format("2006-01-02 15:04"),
	})
	if err != nil {
		return false, err
	}
	delivery := model.NewSMSDelivery()
	delivery.IdempotencyKey = fmt.Sprintf("tds:v1:%d:%d:%s", item.ID, item.DueAt.Time.Unix(), reminderType)
	delivery.Scene = sms.SceneTodoDue
	delivery.UserID = item.AssigneeID
	delivery.BusinessType = todoSMSBusinessType
	delivery.BusinessID = item.ID
	delivery.BusinessVersion = todoSMSBusinessVersion(item.DueAt.Time, reminderType)
	delivery.Variables = string(variables)
	delivery.Status = model.SMSDeliveryStatusPending
	delivery.MaxAttempts = todoSMSMaxAttempts
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(delivery)
	return result.RowsAffected == 1, result.Error
}

func (s *ReminderService) processSMSOutbox(ctx context.Context, db *gorm.DB, now time.Time) (reminderSMSProcessResult, error) {
	result := reminderSMSProcessResult{}
	var runErrors []error
	stale := db.Model(&model.SMSDelivery{}).
		Where("scene = ? AND status = ? AND updated_at < ?", sms.SceneTodoDue, model.SMSDeliveryStatusSending, now.Add(-staleSMSSendingAfter)).
		Updates(map[string]any{
			"status":     model.SMSDeliveryStatusUnknown,
			"last_error": "sender stopped while provider outcome was unresolved",
		})
	if stale.Error != nil {
		return result, stale.Error
	}
	if stale.RowsAffected > 0 {
		result.Unknown += int(stale.RowsAffected)
		runErrors = append(runErrors, fmt.Errorf("%d stale SMS deliveries moved to unknown", stale.RowsAffected))
	}

	var runtime reminderSMSRuntime
	var runtimeErr error
	lastID, claimedCount := uint(0), 0
	for claimedCount < todoSMSBatchSize {
		deliveries := make([]*model.SMSDelivery, 0, todoSMSBatchSize)
		if err := db.Where("id > ?", lastID).Where("scene = ? AND (status = ? OR (status = ? AND next_attempt_at <= ?))",
			sms.SceneTodoDue, model.SMSDeliveryStatusPending, model.SMSDeliveryStatusRetryWait, now).
			Order("id ASC").Limit(todoSMSBatchSize).Find(&deliveries).Error; err != nil {
			return result, err
		}
		if len(deliveries) == 0 {
			break
		}
		if lastID == 0 {
			runtime, runtimeErr = s.smsRuntimeFactory()
			if runtimeErr == nil && runtime.Paused {
				return result, errors.Join(runErrors...)
			}
		}
		for _, candidate := range deliveries {
			if claimedCount >= todoSMSBatchSize {
				break
			}
			lastID = candidate.ID
			delivery, phone, claimed, err := claimTodoSMSDelivery(db, candidate.ID, now)
			if err != nil {
				runErrors = append(runErrors, err)
				continue
			}
			if !claimed {
				continue
			}
			claimedCount++
			if runtimeErr != nil || runtime.Sender == nil {
				if err := finishSMSKnownFailure(db, delivery, now, "sms configuration unavailable", ""); err != nil {
					runErrors = append(runErrors, err)
				} else {
					result.Failed++
					runErrors = append(runErrors, fmt.Errorf("SMS delivery %d configuration unavailable", delivery.ID))
				}
				continue
			}
			variables := make(map[string]string)
			if err := json.Unmarshal([]byte(delivery.Variables), &variables); err != nil {
				if finishErr := finishSMSPermanentFailure(db, delivery, now, "invalid template variables", runtime.Provider); finishErr != nil {
					runErrors = append(runErrors, finishErr)
				} else {
					result.Failed++
					runErrors = append(runErrors, fmt.Errorf("SMS delivery %d has invalid template variables", delivery.ID))
				}
				continue
			}
			if err := s.smsBudgetReserve(ctx, runtime.GlobalDailyLimit); err != nil {
				if finishErr := finishSMSKnownFailure(db, delivery, now, "SMS daily budget unavailable or exhausted", runtime.Provider); finishErr != nil {
					runErrors = append(runErrors, finishErr)
				} else {
					result.Failed++
					runErrors = append(runErrors, fmt.Errorf("SMS delivery %d budget unavailable", delivery.ID))
				}
				continue
			}
			sendResult, sendErr := runtime.Sender.Send(ctx, sms.SceneTodoDue, phone, variables)
			if sendErr != nil {
				log.Logger.Warn("待办提醒短信发送失败", zap.Uint("delivery_id", delivery.ID), zap.String("provider", runtime.Provider), zap.Error(sendErr))
			}
			switch sendResult.State {
			case sms.SendStateAccepted:
				if err := finishSMSAccepted(db, delivery, now, runtime.Provider, sendResult); err != nil {
					runErrors = append(runErrors, err)
					continue
				}
				result.Accepted++
			case sms.SendStateFailed, "":
				reason := "provider rejected SMS request"
				if sendResult.State == "" {
					reason = "SMS request rejected before provider send"
				}
				if err := finishSMSKnownFailure(db, delivery, now, reason, runtime.Provider, sendResult); err != nil {
					runErrors = append(runErrors, err)
					continue
				}
				result.Failed++
				runErrors = append(runErrors, fmt.Errorf("SMS delivery %d: %s", delivery.ID, reason))
			default:
				if err := finishSMSUnknown(db, delivery, now, runtime.Provider, sendResult); err != nil {
					runErrors = append(runErrors, err)
					continue
				}
				result.Unknown++
				runErrors = append(runErrors, fmt.Errorf("SMS delivery %d provider outcome unknown", delivery.ID))
			}
		}
	}
	return result, errors.Join(runErrors...)
}

func claimTodoSMSDelivery(db *gorm.DB, id uint, now time.Time) (*model.SMSDelivery, string, bool, error) {
	delivery := model.NewSMSDelivery()
	phone := ""
	claimed := false
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND (status = ? OR (status = ? AND next_attempt_at <= ?))", id,
				model.SMSDeliveryStatusPending, model.SMSDeliveryStatusRetryWait, now).
			First(delivery).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		item := model.NewUserTodo()
		if delivery.Scene != sms.SceneTodoDue || delivery.BusinessType != todoSMSBusinessType {
			return cancelSMSDelivery(tx, delivery.ID, "unsupported SMS delivery business type")
		}
		if err := tx.Where("id = ? AND assignee_id = ? AND deleted_at = 0 AND status IN ?", delivery.BusinessID, delivery.UserID,
			[]string{model.TodoStatusPending, model.TodoStatusProcessing}).First(item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return cancelSMSDelivery(tx, delivery.ID, "todo is no longer active")
			}
			return err
		}
		if item.DueAt == nil || delivery.BusinessVersion != todoSMSBusinessVersion(item.DueAt.Time, todoReminderType(item.DueAt.Time, now)) {
			return cancelSMSDelivery(tx, delivery.ID, "todo reminder cycle changed")
		}
		preference, fullPhone, err := loadReminderChannels(tx, delivery.UserID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return cancelSMSDelivery(tx, delivery.ID, "recipient is no longer active")
		}
		if err != nil {
			return err
		}
		if !preference.SMSEnabled {
			return cancelSMSDelivery(tx, delivery.ID, "SMS preference disabled")
		}
		if fullPhone == "" {
			return cancelSMSDelivery(tx, delivery.ID, "recipient phone unavailable")
		}
		// A changed lead time pauses this cycle without consuming an attempt.
		// Keep the record eligible for rechecking later preference changes.
		if item.DueAt.Time.After(now.Add(time.Duration(preference.LeadMinutes) * time.Minute)) {
			return nil
		}
		attempt := delivery.AttemptCount + 1
		if err := tx.Model(&model.SMSDelivery{}).Where("id = ?", delivery.ID).Updates(map[string]any{
			"status":           model.SMSDeliveryStatusSending,
			"attempt_count":    attempt,
			"last_attempt_at":  now,
			"next_attempt_at":  nil,
			"recipient_masked": maskReminderPhone(fullPhone),
			"last_error":       "",
		}).Error; err != nil {
			return err
		}
		delivery.Status = model.SMSDeliveryStatusSending
		delivery.AttemptCount = attempt
		phone = fullPhone
		claimed = true
		return nil
	})
	return delivery, phone, claimed, err
}

func cancelSMSDelivery(db *gorm.DB, id uint, reason string) error {
	return db.Model(&model.SMSDelivery{}).Where("id = ?", id).Updates(map[string]any{
		"status":     model.SMSDeliveryStatusCanceled,
		"last_error": reason,
	}).Error
}

func finishSMSAccepted(db *gorm.DB, delivery *model.SMSDelivery, now time.Time, provider string, result sms.SendResult) error {
	return finishSMSDelivery(db, delivery, map[string]any{
		"status":              model.SMSDeliveryStatusAccepted,
		"accepted_at":         now,
		"provider":            provider,
		"provider_request_id": result.ProviderRequestID,
		"provider_message_id": result.ProviderMessageID,
		"provider_code":       result.ProviderCode,
		"last_error":          "",
	})
}

func finishSMSKnownFailure(db *gorm.DB, delivery *model.SMSDelivery, now time.Time, reason, provider string, results ...sms.SendResult) error {
	result := sms.SendResult{}
	if len(results) > 0 {
		result = results[0]
	}
	status := model.SMSDeliveryStatusRetryWait
	nextAttemptAt := any(now.Add(todoSMSRetryDelay(delivery.AttemptCount)))
	if delivery.AttemptCount >= delivery.MaxAttempts {
		status = model.SMSDeliveryStatusFailed
		nextAttemptAt = nil
	}
	return finishSMSDelivery(db, delivery, map[string]any{
		"status":              status,
		"next_attempt_at":     nextAttemptAt,
		"provider":            provider,
		"provider_request_id": result.ProviderRequestID,
		"provider_message_id": result.ProviderMessageID,
		"provider_code":       result.ProviderCode,
		"last_error":          reason,
	})
}

func finishSMSPermanentFailure(db *gorm.DB, delivery *model.SMSDelivery, _ time.Time, reason, provider string) error {
	return finishSMSDelivery(db, delivery, map[string]any{
		"status":          model.SMSDeliveryStatusFailed,
		"next_attempt_at": nil,
		"provider":        provider,
		"last_error":      reason,
	})
}

func finishSMSUnknown(db *gorm.DB, delivery *model.SMSDelivery, _ time.Time, provider string, result sms.SendResult) error {
	return finishSMSDelivery(db, delivery, map[string]any{
		"status":              model.SMSDeliveryStatusUnknown,
		"next_attempt_at":     nil,
		"provider":            provider,
		"provider_request_id": result.ProviderRequestID,
		"provider_message_id": result.ProviderMessageID,
		"provider_code":       result.ProviderCode,
		"last_error":          "provider outcome unknown; automatic retry disabled",
	})
}

func finishSMSDelivery(db *gorm.DB, delivery *model.SMSDelivery, updates map[string]any) error {
	if delivery == nil {
		return nil
	}
	result := db.Model(&model.SMSDelivery{}).
		Where("id = ? AND status = ? AND attempt_count = ?", delivery.ID, model.SMSDeliveryStatusSending, delivery.AttemptCount).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("SMS delivery %d state changed while finishing", delivery.ID)
	}
	return nil
}

func defaultReminderSMSRuntime() (reminderSMSRuntime, error) {
	settings, err := baseservice.NewSMSConfigService().Get(false)
	if err != nil {
		return reminderSMSRuntime{}, err
	}
	return buildReminderSMSRuntime(settings)
}

func buildReminderSMSRuntime(settings sms.Config) (reminderSMSRuntime, error) {
	if !settings.Enabled {
		return reminderSMSRuntime{Paused: true}, nil
	}
	route, ok := settings.Routes[sms.SceneTodoDue]
	if !ok {
		return reminderSMSRuntime{}, errors.New("todo_due SMS route is not configured")
	}
	dispatcher, err := sms.NewDispatcher(settings, nil)
	if err != nil {
		return reminderSMSRuntime{}, err
	}
	return reminderSMSRuntime{Sender: dispatcher, Provider: route.Provider, GlobalDailyLimit: settings.OTP.GlobalDailyLimit}, nil
}

func reserveReminderSMSBudget(ctx context.Context, limit int) error {
	client := data.RedisInitializedClient()
	if client == nil || !data.RedisInitializedReady() || limit < 1 {
		return errors.New("SMS budget storage is unavailable")
	}
	const script = `
local current=tonumber(redis.call('GET',KEYS[1]) or '0')
if current>=tonumber(ARGV[1]) then return 0 end
current=redis.call('INCR',KEYS[1])
if current==1 then redis.call('EXPIRE',KEYS[1],86400) end
return 1`
	allowed, err := client.Eval(ctx, script, []string{"sms:budget:global-day"}, limit).Int()
	if err != nil || allowed != 1 {
		return errors.New("SMS daily budget unavailable or exhausted")
	}
	return nil
}

func todoReminderType(dueAt, now time.Time) string {
	if dueAt.After(now) {
		return "due_soon"
	}
	return "overdue"
}

func todoSMSBusinessVersion(dueAt time.Time, reminderType string) string {
	return fmt.Sprintf("%d:%s", dueAt.Unix(), reminderType)
}

func todoSMSRetryDelay(attempt uint) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := todoSMSRetryBase << (attempt - 1)
	if delay > time.Hour {
		return time.Hour
	}
	return delay
}

func truncateSMSVariable(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes < 1 || utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxRunes])
}

func normalizeReminderPhone(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "+") {
		return value
	}
	return "+" + value
}

func maskReminderPhone(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= 7 {
		return "****"
	}
	return string(runes[:3]) + "****" + string(runes[len(runes)-4:])
}
