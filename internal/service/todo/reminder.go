package todo

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/service/notification"
)

const reminderPageSize = 200

// ReminderOptions 控制一次待办提醒扫描的时间窗口。
type ReminderOptions struct {
	LeadTime      time.Duration
	CatchupWindow time.Duration
	SMSEnabled    bool
}

// ReminderResult 描述一次扫描结果，便于 cron 运行记录和测试核对。
type ReminderResult struct {
	Scanned     int
	Handled     int
	SMSQueued   int
	SMSAccepted int
	SMSFailed   int
	SMSUnknown  int
}

type ReminderDeps struct {
	DB                  *gorm.DB
	Now                 func() time.Time
	NotificationFactory func(*gorm.DB) *notification.Service
	SMSRuntimeFactory   reminderSMSRuntimeFactory
	SMSBudgetReserve    reminderSMSBudgetReserve
}

// ReminderService 将待办的时间状态转换为站内通知。通知唯一键是最终幂等边界，
// 因此重复扫描或多个 cron 实例异常并发时不会重复投递实时事件。
type ReminderService struct {
	db                  *gorm.DB
	now                 func() time.Time
	notificationFactory func(*gorm.DB) *notification.Service
	smsRuntimeFactory   reminderSMSRuntimeFactory
	smsBudgetReserve    reminderSMSBudgetReserve
}

func NewReminderService() *ReminderService {
	return NewReminderServiceWithDeps(ReminderDeps{})
}

func NewReminderServiceWithDeps(deps ReminderDeps) *ReminderService {
	notificationFactory := deps.NotificationFactory
	if notificationFactory == nil {
		notificationFactory = func(db *gorm.DB) *notification.Service {
			return notification.NewServiceWithDeps(notification.ServiceDeps{DB: db})
		}
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	smsRuntimeFactory := deps.SMSRuntimeFactory
	if smsRuntimeFactory == nil {
		smsRuntimeFactory = defaultReminderSMSRuntime
	}
	smsBudgetReserve := deps.SMSBudgetReserve
	if smsBudgetReserve == nil {
		smsBudgetReserve = reserveReminderSMSBudget
	}
	return &ReminderService{
		db: deps.DB, now: now, notificationFactory: notificationFactory,
		smsRuntimeFactory: smsRuntimeFactory, smsBudgetReserve: smsBudgetReserve,
	}
}

func (s *ReminderService) Run(ctx context.Context, options ReminderOptions) (ReminderResult, error) {
	result := ReminderResult{}
	if options.LeadTime < 0 || options.CatchupWindow < 0 {
		return result, fmt.Errorf("todo reminder window must not be negative")
	}
	db := s.db
	if db == nil {
		var err error
		db, err = model.GetDB()
		if err != nil {
			return result, err
		}
	}
	if ctx != nil {
		db = db.WithContext(ctx)
	}

	now := s.now()
	windowStart := now.Add(-options.CatchupWindow)
	windowEnd := now.Add(options.LeadTime)
	lastID := uint(0)
	for {
		items := make([]*model.UserTodo, 0, reminderPageSize)
		err := db.Model(&model.UserTodo{}).
			Where("user_todos.id > ? AND user_todos.deleted_at = 0", lastID).
			Where("user_todos.status IN ?", []string{model.TodoStatusPending, model.TodoStatusProcessing}).
			Where("user_todos.due_at IS NOT NULL AND user_todos.due_at >= ? AND user_todos.due_at <= ?", windowStart, windowEnd).
			Where("EXISTS (SELECT 1 FROM admin_user WHERE admin_user.id = user_todos.assignee_id AND admin_user.status = ? AND admin_user.deleted_at = 0)", model.AdminUserStatusEnabled).
			Order("user_todos.id ASC").
			Limit(reminderPageSize).
			Find(&items).Error
		if err != nil {
			return result, err
		}
		if len(items) == 0 {
			break
		}
		candidates, err := loadReminderCandidates(db, items, now, options.SMSEnabled)
		if err != nil {
			return result, err
		}
		for _, item := range items {
			result.Scanned++
			lastID = item.ID
			candidate, ok := candidates[item.ID]
			if !ok {
				continue
			}
			if (!candidate.inAppEnabled || candidate.delivered.inApp) && (!candidate.smsEnabled || candidate.delivered.sms) {
				// Handled 表示本轮有效渠道已处理，包含此前已经投递的周期。
				if candidate.inAppEnabled || candidate.smsEnabled {
					result.Handled++
				}
				continue
			}
			handled, smsQueued, err := s.notifyWithDeliveryState(ctx, db, item, now, options.SMSEnabled, candidate.delivered)
			if err != nil {
				return result, err
			}
			if handled {
				result.Handled++
			}
			if smsQueued {
				result.SMSQueued++
			}
		}
		if len(items) < reminderPageSize {
			break
		}
	}
	if options.SMSEnabled {
		smsResult, err := s.processSMSOutbox(ctx, db, now)
		result.SMSAccepted += smsResult.Accepted
		result.SMSFailed += smsResult.Failed
		result.SMSUnknown += smsResult.Unknown
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

type reminderDeliveryState struct {
	inApp bool
	sms   bool
}

type reminderCandidate struct {
	userID       uint
	inAppEnabled bool
	smsEnabled   bool
	delivered    reminderDeliveryState
}

// 批量预查偏好和周期投递记录；只有缺少已开启渠道的待办才进入逐条事务。
// 消息软删除和短信失败/未知等状态均保留幂等身份，不能再次生成同周期投递。
func loadReminderCandidates(db *gorm.DB, items []*model.UserTodo, now time.Time, smsEnabled bool) (map[uint]*reminderCandidate, error) {
	preferencesByUser := make(map[uint]reminderSMSPreference)
	userIDs := make([]uint, 0, len(items))
	for _, item := range items {
		if _, exists := preferencesByUser[item.AssigneeID]; exists {
			continue
		}
		preferencesByUser[item.AssigneeID] = reminderSMSPreference{InAppEnabled: true, LeadMinutes: DefaultReminderLeadMinutes}
		userIDs = append(userIDs, item.AssigneeID)
	}
	var preferences []model.NotificationPreference
	if err := db.Model(&model.NotificationPreference{}).
		Select("user_id", "lead_minutes", "in_app_enabled", "sms_enabled").
		Where("scene = ? AND user_id IN ?", model.NotificationSceneTodoDue, userIDs).
		Find(&preferences).Error; err != nil {
		return nil, err
	}
	for _, preference := range preferences {
		preferencesByUser[preference.UserID] = reminderSMSPreference{
			InAppEnabled: preference.InAppEnabled == 1,
			SMSEnabled:   preference.SMSEnabled == 1,
			LeadMinutes:  effectiveReminderLeadMinutes(preference.LeadMinutes),
		}
	}
	phoneAvailable := make(map[uint]bool)
	if smsEnabled {
		smsUserIDs := make([]uint, 0, len(userIDs))
		for _, userID := range userIDs {
			if preferencesByUser[userID].SMSEnabled {
				smsUserIDs = append(smsUserIDs, userID)
			}
		}
		if len(smsUserIDs) > 0 {
			var users []model.AdminUser
			if err := db.Select("id", "full_phone_number").Where("id IN ? AND status = ? AND deleted_at = 0", smsUserIDs, model.AdminUserStatusEnabled).Find(&users).Error; err != nil {
				return nil, err
			}
			for _, user := range users {
				phoneAvailable[user.ID] = normalizeReminderPhone(user.FullPhoneNumber) != ""
			}
		}
	}

	candidates := make(map[uint]*reminderCandidate, len(items))
	messageKeys, smsKeys := make([]string, 0, len(items)), make([]string, 0, len(items))
	messageTargets, smsTargets := make(map[string]*reminderCandidate), make(map[string]*reminderCandidate)
	for _, item := range items {
		preference := preferencesByUser[item.AssigneeID]
		if item.DueAt == nil || item.DueAt.Time.After(now.Add(time.Duration(preference.LeadMinutes)*time.Minute)) {
			continue
		}
		candidate := &reminderCandidate{
			userID:       item.AssigneeID,
			inAppEnabled: preference.InAppEnabled,
			smsEnabled:   smsEnabled && preference.SMSEnabled && phoneAvailable[item.AssigneeID],
		}
		candidates[item.ID] = candidate
		cycle := fmt.Sprintf("v1:%d:%d:%s", item.ID, item.DueAt.Time.Unix(), todoReminderType(item.DueAt.Time, now))
		if candidate.inAppEnabled {
			key := "tdr:" + cycle
			messageKeys = append(messageKeys, key)
			messageTargets[key] = candidate
		}
		if candidate.smsEnabled {
			key := "tds:" + cycle
			smsKeys = append(smsKeys, key)
			smsTargets[key] = candidate
		}
	}
	if len(messageKeys) > 0 {
		var messages []model.SysMessage
		if err := db.Select("user_id", "batch_id").Where("user_id IN ? AND batch_id IN ?", userIDs, messageKeys).Find(&messages).Error; err != nil {
			return nil, err
		}
		for _, message := range messages {
			if target := messageTargets[message.BatchID]; target != nil && target.userID == message.UserID {
				target.delivered.inApp = true
			}
		}
	}
	if len(smsKeys) > 0 {
		var deliveries []model.SMSDelivery
		if err := db.Select("user_id", "idempotency_key").Where("idempotency_key IN ?", smsKeys).Find(&deliveries).Error; err != nil {
			return nil, err
		}
		for _, delivery := range deliveries {
			if target := smsTargets[delivery.IdempotencyKey]; target != nil && target.userID == delivery.UserID {
				target.delivered.sms = true
			}
		}
	}
	return candidates, nil
}

func (s *ReminderService) notify(ctx context.Context, db *gorm.DB, item *model.UserTodo, now time.Time, smsEnabled bool) (bool, bool, error) {
	return s.notifyWithDeliveryState(ctx, db, item, now, smsEnabled, reminderDeliveryState{})
}

func (s *ReminderService) notifyWithDeliveryState(ctx context.Context, db *gorm.DB, item *model.UserTodo, now time.Time, smsEnabled bool, delivered reminderDeliveryState) (bool, bool, error) {
	if item == nil || item.DueAt == nil {
		return false, false, nil
	}
	var message *model.SysMessage
	created := false
	handled := false
	smsQueued := false
	err := db.Transaction(func(tx *gorm.DB) error {
		current := model.NewUserTodo()
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_todos.id = ? AND user_todos.deleted_at = 0", item.ID).
			Where("user_todos.status IN ?", []string{model.TodoStatusPending, model.TodoStatusProcessing}).
			Where("user_todos.due_at = ?", item.DueAt.Time).
			Where("EXISTS (SELECT 1 FROM admin_user WHERE admin_user.id = user_todos.assignee_id AND admin_user.status = ? AND admin_user.deleted_at = 0)", model.AdminUserStatusEnabled).
			First(current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}

		preference, fullPhone, err := loadReminderChannels(tx, current.AssigneeID)
		if err != nil {
			return err
		}
		if current.DueAt.Time.After(now.Add(time.Duration(preference.LeadMinutes) * time.Minute)) {
			return nil
		}
		if preference.InAppEnabled {
			if !delivered.inApp {
				message, created, err = s.notificationFactory(tx).Persist(ctx, buildReminderMessage(current, now))
				if err != nil {
					return err
				}
			}
			handled = true
		}
		if smsEnabled && preference.SMSEnabled && fullPhone != "" {
			if !delivered.sms {
				smsQueued, err = persistTodoSMSDelivery(tx, current, now)
				if err != nil {
					return err
				}
			}
			handled = true
		}
		return nil
	})
	if err != nil || !handled || !created {
		return handled, smsQueued, err
	}
	if err := s.notificationFactory(db).DispatchCreated(ctx, message); err != nil {
		return true, smsQueued, err
	}
	return true, smsQueued, nil
}

func buildReminderMessage(item *model.UserTodo, now time.Time) notification.CreateMessageInput {
	dueAt := item.DueAt.Time
	topic := model.MessageTopicTodoDueSoon
	title := "待办即将到期"
	content := fmt.Sprintf("待办「%s」将在 %s 后到期，截止时间：%s。", item.Title, formatLeadTime(dueAt.Sub(now)), dueAt.Format("2006-01-02 15:04"))
	reminderType := "due_soon"
	if !dueAt.After(now) {
		topic = model.MessageTopicTodoOverdue
		title = "待办已逾期"
		content = fmt.Sprintf("待办「%s」已逾期，截止时间：%s。", item.Title, dueAt.Format("2006-01-02 15:04"))
		reminderType = "overdue"
	}
	batchID := fmt.Sprintf("tdr:v1:%d:%d:%s", item.ID, dueAt.Unix(), reminderType)
	actionURL := strings.TrimSpace(item.ActionURL)
	normalizedActionURL, err := notification.NormalizeActionURL(actionURL)
	if err != nil || normalizedActionURL == "" {
		actionURL = "/todo/list"
	} else {
		actionURL = normalizedActionURL
	}
	return notification.CreateMessageInput{
		UserID:      item.AssigneeID,
		Category:    model.MessageCategoryTodo,
		Topic:       topic,
		Title:       title,
		Content:     content,
		ActionURL:   actionURL,
		ActionLabel: "查看待办",
		BatchID:     batchID,
		Payload: map[string]any{
			"todo_id":       item.ID,
			"source_type":   item.SourceType,
			"due_at":        dueAt.Format("2006-01-02 15:04:05"),
			"reminder_type": reminderType,
		},
	}
}

func formatLeadTime(duration time.Duration) string {
	minutes := int(math.Ceil(duration.Minutes()))
	if minutes < 1 {
		minutes = 1
	}
	if minutes < 60 {
		return fmt.Sprintf("%d 分钟", minutes)
	}
	if minutes%60 == 0 && minutes < 24*60 {
		return fmt.Sprintf("%d 小时", minutes/60)
	}
	if minutes%(24*60) == 0 {
		return fmt.Sprintf("%d 天", minutes/(24*60))
	}
	return fmt.Sprintf("%d 小时 %d 分钟", minutes/60, minutes%60)
}
