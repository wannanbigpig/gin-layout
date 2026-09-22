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
		for _, item := range items {
			result.Scanned++
			handled, smsQueued, err := s.notify(ctx, db, item, now, options.SMSEnabled)
			if err != nil {
				return result, err
			}
			if handled {
				result.Handled++
			}
			if smsQueued {
				result.SMSQueued++
			}
			lastID = item.ID
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

func (s *ReminderService) notify(ctx context.Context, db *gorm.DB, item *model.UserTodo, now time.Time, smsEnabled bool) (bool, bool, error) {
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
		input := buildReminderMessage(current, now)
		if preference.InAppEnabled {
			message, created, err = s.notificationFactory(tx).Persist(ctx, input)
			if err != nil {
				return err
			}
			handled = true
		}
		if smsEnabled && preference.SMSEnabled && fullPhone != "" {
			smsQueued, err = persistTodoSMSDelivery(tx, current, now)
			if err != nil {
				return err
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
