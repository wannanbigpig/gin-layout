package todo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/service/notification"
	"github.com/wannanbigpig/gin-layout/internal/sms"
)

func TestReminderRunCreatesDueSoonAndOverdueMessagesIdempotently(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	seedReminderUser(t, db, 8, model.AdminUserStatusDisabled)

	seedReminderTodo(t, db, 7, "due soon", model.TodoStatusPending, now.Add(30*time.Minute))
	seedReminderTodo(t, db, 7, "overdue", model.TodoStatusProcessing, now.Add(-30*time.Minute))
	seedReminderTodo(t, db, 7, "too old", model.TodoStatusPending, now.Add(-2*time.Hour))
	seedReminderTodo(t, db, 7, "completed", model.TodoStatusCompleted, now.Add(20*time.Minute))
	seedReminderTodo(t, db, 8, "disabled user", model.TodoStatusPending, now.Add(20*time.Minute))

	var dispatches atomic.Int64
	svc := newReminderTestService(db, now, func(notification.Event) {
		dispatches.Add(1)
	})
	options := ReminderOptions{LeadTime: time.Hour, CatchupWindow: time.Hour}

	first, err := svc.Run(context.Background(), options)
	if err != nil {
		t.Fatalf("first reminder run: %v", err)
	}
	second, err := svc.Run(context.Background(), options)
	if err != nil {
		t.Fatalf("second reminder run: %v", err)
	}
	if first.Scanned != 2 || first.Handled != 2 || second.Scanned != 2 || second.Handled != 2 {
		t.Fatalf("unexpected scan results: first=%#v second=%#v", first, second)
	}

	var messages []model.SysMessage
	if err := db.Order("id ASC").Find(&messages).Error; err != nil {
		t.Fatalf("load messages: %v", err)
	}
	if len(messages) != 2 || dispatches.Load() != 2 {
		t.Fatalf("duplicate run must not duplicate rows/events: rows=%d dispatches=%d", len(messages), dispatches.Load())
	}
	if messages[0].Category != model.MessageCategoryTodo || messages[0].Topic != model.MessageTopicTodoDueSoon {
		t.Fatalf("unexpected due-soon message: %#v", messages[0])
	}
	if messages[1].Topic != model.MessageTopicTodoOverdue {
		t.Fatalf("unexpected overdue message: %#v", messages[1])
	}
}

func TestReminderRunRescheduledTodoCreatesNewReminderCycle(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	item := seedReminderTodo(t, db, 7, "rescheduled", model.TodoStatusPending, now.Add(30*time.Minute))
	svc := newReminderTestService(db, now, nil)
	options := ReminderOptions{LeadTime: 2 * time.Hour, CatchupWindow: time.Hour}
	if _, err := svc.Run(context.Background(), options); err != nil {
		t.Fatalf("first run: %v", err)
	}
	newDueAt := now.Add(90 * time.Minute)
	if err := db.Model(&model.UserTodo{}).Where("id = ?", item.ID).Update("due_at", newDueAt).Error; err != nil {
		t.Fatalf("reschedule todo: %v", err)
	}
	if _, err := svc.Run(context.Background(), options); err != nil {
		t.Fatalf("second run: %v", err)
	}
	var count int64
	if err := db.Model(&model.SysMessage{}).Count(&count).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if count != 2 {
		t.Fatalf("rescheduled todo should create a new reminder cycle, got %d", count)
	}
}

func TestReminderNotifyRechecksCurrentTodoState(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	item := seedReminderTodo(t, db, 7, "completed after scan", model.TodoStatusPending, now.Add(30*time.Minute))
	if err := db.Model(&model.UserTodo{}).Where("id = ?", item.ID).Update("status", model.TodoStatusCompleted).Error; err != nil {
		t.Fatalf("complete todo: %v", err)
	}

	svc := newReminderTestService(db, now, nil)
	handled, _, err := svc.notify(context.Background(), db, item, now, false)
	if err != nil {
		t.Fatalf("notify stale todo: %v", err)
	}
	if handled {
		t.Fatal("stale completed todo must be ignored")
	}
	var count int64
	if err := db.Model(&model.SysMessage{}).Count(&count).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if count != 0 {
		t.Fatalf("stale completed todo created %d messages", count)
	}
}

func TestReminderFallsBackFromLegacyUnsafeActionURL(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	item := seedReminderTodo(t, db, 7, "legacy action", model.TodoStatusPending, now.Add(30*time.Minute))
	if err := db.Model(&model.UserTodo{}).Where("id = ?", item.ID).Update("action_url", "/todo/list\nunsafe").Error; err != nil {
		t.Fatalf("set legacy action url: %v", err)
	}
	item.ActionURL = "/todo/list\nunsafe"

	svc := newReminderTestService(db, now, nil)
	handled, _, err := svc.notify(context.Background(), db, item, now, false)
	if err != nil || !handled {
		t.Fatalf("notify legacy todo: handled=%v err=%v", handled, err)
	}
	var message model.SysMessage
	if err := db.First(&message).Error; err != nil {
		t.Fatalf("load message: %v", err)
	}
	if message.ActionURL != "/todo/list" {
		t.Fatalf("expected safe fallback action url, got %q", message.ActionURL)
	}
}

func TestReminderSMSRequiresExplicitUserPreference(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	seedReminderTodo(t, db, 7, "default preference", model.TodoStatusPending, now.Add(30*time.Minute))
	sender := &fakeReminderSMSSender{result: sms.SendResult{State: sms.SendStateAccepted}}
	svc := newReminderSMSTestService(db, func() time.Time { return now }, sender)

	if _, err := svc.Run(context.Background(), ReminderOptions{LeadTime: time.Hour, CatchupWindow: time.Hour, SMSEnabled: true}); err != nil {
		t.Fatalf("run reminder: %v", err)
	}
	if sender.calls != 0 {
		t.Fatalf("SMS must be opt-in, calls=%d", sender.calls)
	}
	var count int64
	if err := db.Model(&model.SMSDelivery{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("default preference must not create SMS outbox: count=%d err=%v", count, err)
	}
}

func TestReminderSMSRequiresSystemChannelSwitch(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	seedReminderTodo(t, db, 7, "system switch off", model.TodoStatusPending, now.Add(30*time.Minute))
	seedReminderPreference(t, db, 7, true, true)
	sender := &fakeReminderSMSSender{result: sms.SendResult{State: sms.SendStateAccepted}}
	svc := newReminderSMSTestService(db, func() time.Time { return now }, sender)

	result, err := svc.Run(context.Background(), ReminderOptions{LeadTime: time.Hour, CatchupWindow: time.Hour})
	if err != nil {
		t.Fatalf("run reminder: %v", err)
	}
	if result.Handled != 1 || result.SMSQueued != 0 || sender.calls != 0 {
		t.Fatalf("system SMS switch must suppress outbox and send: result=%#v calls=%d", result, sender.calls)
	}
	var deliveryCount int64
	if err := db.Model(&model.SMSDelivery{}).Count(&deliveryCount).Error; err != nil || deliveryCount != 0 {
		t.Fatalf("system SMS switch off must not create outbox: count=%d err=%v", deliveryCount, err)
	}
	var messageCount int64
	if err := db.Model(&model.SysMessage{}).Count(&messageCount).Error; err != nil || messageCount != 1 {
		t.Fatalf("system SMS switch must not disable in-app reminder: count=%d err=%v", messageCount, err)
	}
}

func TestReminderSMSAcceptedIsPersistedAndIdempotent(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	seedReminderTodo(t, db, 7, "send once", model.TodoStatusPending, now.Add(30*time.Minute))
	seedReminderPreference(t, db, 7, true, true)
	sender := &fakeReminderSMSSender{result: sms.SendResult{
		State: sms.SendStateAccepted, ProviderRequestID: "req-1", ProviderMessageID: "msg-1", ProviderCode: "OK",
	}}
	svc := newReminderSMSTestService(db, func() time.Time { return now }, sender)
	options := ReminderOptions{LeadTime: time.Hour, CatchupWindow: time.Hour, SMSEnabled: true}

	first, err := svc.Run(context.Background(), options)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	second, err := svc.Run(context.Background(), options)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if first.SMSQueued != 1 || first.SMSAccepted != 1 || second.SMSQueued != 0 || sender.calls != 1 {
		t.Fatalf("unexpected SMS results: first=%#v second=%#v calls=%d", first, second, sender.calls)
	}
	var delivery model.SMSDelivery
	if err := db.First(&delivery).Error; err != nil {
		t.Fatalf("load delivery: %v", err)
	}
	if delivery.Status != model.SMSDeliveryStatusAccepted || delivery.AttemptCount != 1 || delivery.ProviderMessageID != "msg-1" {
		t.Fatalf("unexpected accepted delivery: %#v", delivery)
	}
	if delivery.RecipientMasked == "" || strings.Contains(delivery.RecipientMasked, "13800000007") {
		t.Fatalf("recipient must be masked: %q", delivery.RecipientMasked)
	}
}

func TestReminderSMSUnknownIsNeverAutomaticallyRetried(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	seedReminderTodo(t, db, 7, "unknown", model.TodoStatusPending, now.Add(30*time.Minute))
	seedReminderPreference(t, db, 7, true, true)
	sender := &fakeReminderSMSSender{result: sms.SendResult{State: sms.SendStateUnknown}, err: errors.New("timeout")}
	svc := newReminderSMSTestService(db, func() time.Time { return now }, sender)
	options := ReminderOptions{LeadTime: time.Hour, CatchupWindow: time.Hour, SMSEnabled: true}

	first, err := svc.Run(context.Background(), options)
	if err == nil || first.SMSUnknown != 1 {
		t.Fatalf("unknown outcome must fail the run and be counted: result=%#v err=%v", first, err)
	}
	if _, err := svc.Run(context.Background(), options); err != nil {
		t.Fatalf("terminal unknown must not fail later scans: %v", err)
	}
	if sender.calls != 1 {
		t.Fatalf("unknown delivery must not retry, calls=%d", sender.calls)
	}
	var delivery model.SMSDelivery
	if err := db.First(&delivery).Error; err != nil || delivery.Status != model.SMSDeliveryStatusUnknown {
		t.Fatalf("unexpected unknown delivery: %#v err=%v", delivery, err)
	}
}

func TestReminderSMSStaleSendingBecomesUnknownWithoutRetry(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	item := seedReminderTodo(t, db, 7, "stale sending", model.TodoStatusPending, now.Add(30*time.Minute))
	seedReminderPreference(t, db, 7, true, true)
	if _, err := persistTodoSMSDelivery(db, item, now); err != nil {
		t.Fatalf("persist delivery: %v", err)
	}
	if err := db.Model(&model.SMSDelivery{}).Where("business_id = ?", item.ID).Updates(map[string]any{
		"status":        model.SMSDeliveryStatusSending,
		"attempt_count": 1,
	}).Error; err != nil {
		t.Fatalf("mark delivery sending: %v", err)
	}
	if err := db.Model(&model.SMSDelivery{}).Where("business_id = ?", item.ID).
		UpdateColumn("updated_at", now.Add(-staleSMSSendingAfter-time.Minute)).Error; err != nil {
		t.Fatalf("make delivery stale: %v", err)
	}

	sender := &fakeReminderSMSSender{result: sms.SendResult{State: sms.SendStateAccepted}}
	svc := newReminderSMSTestService(db, func() time.Time { return now }, sender)
	result, err := svc.processSMSOutbox(context.Background(), db, now)
	if err == nil || result.Unknown != 1 {
		t.Fatalf("stale sending must surface unknown: result=%#v err=%v", result, err)
	}
	if sender.calls != 0 {
		t.Fatalf("stale sending must not be retried, calls=%d", sender.calls)
	}
	var delivery model.SMSDelivery
	if err := db.First(&delivery).Error; err != nil || delivery.Status != model.SMSDeliveryStatusUnknown {
		t.Fatalf("unexpected stale delivery: %#v err=%v", delivery, err)
	}
}

func TestReminderSMSKnownFailureRetriesAtMostThreeTimes(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	seedReminderTodo(t, db, 7, "retry", model.TodoStatusPending, now.Add(2*time.Hour))
	seedReminderPreference(t, db, 7, false, true)
	sender := &fakeReminderSMSSender{result: sms.SendResult{State: sms.SendStateFailed, ProviderCode: "BUSY"}, err: errors.New("busy")}
	svc := newReminderSMSTestService(db, func() time.Time { return now }, sender)
	options := ReminderOptions{LeadTime: 3 * time.Hour, CatchupWindow: time.Hour, SMSEnabled: true}

	for attempt, advance := range []time.Duration{0, 5 * time.Minute, 10 * time.Minute} {
		now = now.Add(advance)
		result, err := svc.Run(context.Background(), options)
		if err == nil || result.SMSFailed != 1 {
			t.Fatalf("attempt %d should record known failure: result=%#v err=%v", attempt+1, result, err)
		}
	}
	now = now.Add(time.Hour)
	if _, err := svc.Run(context.Background(), options); err != nil {
		t.Fatalf("terminal failed delivery must not retry: %v", err)
	}
	if sender.calls != 3 {
		t.Fatalf("known failure attempts=%d, want 3", sender.calls)
	}
	var delivery model.SMSDelivery
	if err := db.First(&delivery).Error; err != nil || delivery.Status != model.SMSDeliveryStatusFailed || delivery.AttemptCount != 3 {
		t.Fatalf("unexpected terminal failed delivery: %#v err=%v", delivery, err)
	}
}

func TestReminderPreferenceDefaultsAndRequiresPhoneForSMS(t *testing.T) {
	db := newReminderTestDB(t)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	if err := db.Exec("INSERT INTO admin_user (id, status, full_phone_number, deleted_at) VALUES (9, 1, '', 0)").Error; err != nil {
		t.Fatalf("seed no-phone user: %v", err)
	}
	svc := NewServiceWithDB(db)
	preference, err := svc.GetReminderPreference(context.Background(), 7)
	if err != nil || !preference.InAppEnabled || preference.SMSEnabled || !preference.SMSAvailable || preference.LeadMinutes != DefaultReminderLeadMinutes {
		t.Fatalf("unexpected default preference: %#v err=%v", preference, err)
	}
	minutes := 30
	updated, err := svc.UpdateReminderPreference(context.Background(), 7, false, true, &minutes)
	if err != nil || updated.InAppEnabled || !updated.SMSEnabled || updated.LeadMinutes != 30 {
		t.Fatalf("unexpected updated preference: %#v err=%v", updated, err)
	}
	if _, err := svc.UpdateReminderPreference(context.Background(), 9, true, true, nil); err == nil {
		t.Fatal("SMS preference must reject users without a phone")
	}
	if _, err := svc.UpdateReminderPreference(context.Background(), 7, true, false, nil); err != nil {
		t.Fatalf("legacy update must keep lead time: %v", err)
	}
	got, err := svc.GetReminderPreference(context.Background(), 7)
	if err != nil || got.LeadMinutes != 30 {
		t.Fatalf("legacy update changed lead time: %#v err=%v", got, err)
	}
	zero := 0
	updated, err = svc.UpdateReminderPreference(context.Background(), 7, true, false, &zero)
	if err != nil || updated.LeadMinutes != 0 {
		t.Fatalf("zero lead time must be accepted: %#v err=%v", updated, err)
	}
	got, err = svc.GetReminderPreference(context.Background(), 7)
	if err != nil || got.LeadMinutes != 0 {
		t.Fatalf("zero lead time was not persisted: %#v err=%v", got, err)
	}
	invalid := -1
	if _, err := svc.UpdateReminderPreference(context.Background(), 7, true, false, &invalid); err == nil {
		t.Fatal("negative lead time must be rejected")
	}
	oneMinute := 1
	updated, err = svc.UpdateReminderPreference(context.Background(), 7, true, false, &oneMinute)
	if err != nil || updated.LeadMinutes != oneMinute {
		t.Fatalf("one-minute lead time must be accepted: %#v err=%v", updated, err)
	}
	maximum := MaxReminderLeadMinutes
	updated, err = svc.UpdateReminderPreference(context.Background(), 7, true, false, &maximum)
	if err != nil || updated.LeadMinutes != maximum {
		t.Fatalf("maximum lead time must be accepted: %#v err=%v", updated, err)
	}
	got, err = svc.GetReminderPreference(context.Background(), 7)
	if err != nil || got.LeadMinutes != maximum {
		t.Fatalf("maximum lead time was not persisted: %#v err=%v", got, err)
	}
	tooLong := maximum + 1
	if _, err := svc.UpdateReminderPreference(context.Background(), 7, true, false, &tooLong); err == nil {
		t.Fatal("lead time beyond the maximum must be rejected")
	}
}

func TestReminderRunIncludesSevenDayLeadBoundary(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.Local)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	seedReminderTodo(t, db, 7, "seven-day reminder", model.TodoStatusPending, now.Add(7*24*time.Hour))
	seedReminderTodo(t, db, 7, "outside window", model.TodoStatusPending, now.Add(7*24*time.Hour+time.Minute))
	sevenDays := 7 * 24 * 60
	if _, err := NewServiceWithDB(db).UpdateReminderPreference(context.Background(), 7, true, false, &sevenDays); err != nil {
		t.Fatalf("set seven-day lead time: %v", err)
	}
	result, err := newReminderTestService(db, now, nil).Run(context.Background(), ReminderOptions{LeadTime: time.Duration(MaxReminderLeadMinutes) * time.Minute})
	if err != nil || result.Handled != 1 {
		t.Fatalf("seven-day scan: %#v err=%v", result, err)
	}
	var messages []model.SysMessage
	if err := db.Find(&messages).Error; err != nil || len(messages) != 1 || messages[0].Title != "待办即将到期" {
		t.Fatalf("unexpected seven-day notification: %#v err=%v", messages, err)
	}
}

func TestReminderRunUsesEachAssigneeLeadTime(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	seedReminderUser(t, db, 8, model.AdminUserStatusEnabled)
	seedReminderTodo(t, db, 7, "short lead", model.TodoStatusPending, now.Add(30*time.Minute))
	seedReminderTodo(t, db, 8, "default lead", model.TodoStatusPending, now.Add(30*time.Minute))
	minutes := 5
	if _, err := NewServiceWithDB(db).UpdateReminderPreference(context.Background(), 7, true, false, &minutes); err != nil {
		t.Fatalf("set personal lead time: %v", err)
	}
	svc := newReminderTestService(db, now, nil)
	first, err := svc.Run(context.Background(), ReminderOptions{LeadTime: 24 * time.Hour})
	if err != nil || first.Handled != 1 {
		t.Fatalf("first scan: %#v err=%v", first, err)
	}
	var messages []model.SysMessage
	if err := db.Find(&messages).Error; err != nil || len(messages) != 1 || messages[0].UserID != 8 {
		t.Fatalf("wrong user notified: %#v err=%v", messages, err)
	}
	svc.now = func() time.Time { return now.Add(25 * time.Minute) }
	second, err := svc.Run(context.Background(), ReminderOptions{LeadTime: 24 * time.Hour})
	if err != nil || second.Handled != 2 {
		t.Fatalf("second scan: %#v err=%v", second, err)
	}
	if err := db.Find(&messages).Error; err != nil || len(messages) != 2 {
		t.Fatalf("short lead reminder missing: %#v err=%v", messages, err)
	}
}

type fakeReminderSMSSender struct {
	result sms.SendResult
	err    error
	calls  int
}

func (f *fakeReminderSMSSender) Send(context.Context, string, string, map[string]string) (sms.SendResult, error) {
	f.calls++
	return f.result, f.err
}

func newReminderSMSTestService(db *gorm.DB, now func() time.Time, sender reminderSMSSender) *ReminderService {
	return NewReminderServiceWithDeps(ReminderDeps{
		DB:  db,
		Now: now,
		NotificationFactory: func(tx *gorm.DB) *notification.Service {
			return notification.NewServiceWithDeps(notification.ServiceDeps{
				DB: tx, BroadcastFn: func(notification.Event) {}, PublishFn: func(context.Context, notification.Event) error { return nil },
			})
		},
		SMSRuntimeFactory: func() (reminderSMSRuntime, error) {
			return reminderSMSRuntime{Sender: sender, Provider: sms.ProviderAliyun, GlobalDailyLimit: 100}, nil
		},
		SMSBudgetReserve: func(context.Context, int) error { return nil },
	})
}

func seedReminderPreference(t *testing.T, db *gorm.DB, userID uint, inAppEnabled, smsEnabled bool) {
	t.Helper()
	if err := db.Exec(`INSERT INTO notification_preferences (user_id, scene, in_app_enabled, sms_enabled)
		VALUES (?, ?, ?, ?)`, userID, model.NotificationSceneTodoDue, boolToUint8(inAppEnabled), boolToUint8(smsEnabled)).Error; err != nil {
		t.Fatalf("seed reminder preference: %v", err)
	}
}

func newReminderTestService(db *gorm.DB, now time.Time, onDispatch func(notification.Event)) *ReminderService {
	return NewReminderServiceWithDeps(ReminderDeps{
		DB:  db,
		Now: func() time.Time { return now },
		NotificationFactory: func(tx *gorm.DB) *notification.Service {
			return notification.NewServiceWithDeps(notification.ServiceDeps{
				DB: tx,
				BroadcastFn: func(event notification.Event) {
					if onDispatch != nil {
						onDispatch(event)
					}
				},
				PublishFn: func(context.Context, notification.Event) error { return nil },
			})
		},
	})
}

func newReminderTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	statements := []string{
		`CREATE TABLE admin_user (id integer primary key, status integer not null default 1, full_phone_number text not null default '', deleted_at integer not null default 0)`,
		`CREATE TABLE user_todos (
			id integer primary key autoincrement, assignee_id integer not null default 0, creator_id integer not null default 0,
			group_id integer not null default 0,
			source_type text not null default 'manual', source_key text not null default '', title text not null default '', description text,
			priority text not null default 'normal', status text not null default 'pending', action_url text not null default '', due_at datetime,
			completed_at datetime, canceled_at datetime, created_at datetime, updated_at datetime, deleted_at integer not null default 0
		)`,
		`CREATE TABLE sys_messages (
			id integer primary key autoincrement, user_id integer not null default 0, sender_id integer not null default 0,
			batch_id text not null default '', category text not null default '', topic text not null default '', title text not null default '',
			content text, payload text, action_url text not null default '', action_label text not null default '', recipient_type text not null default '',
			recipient_ids text, send_immediately integer not null default 1, send_at datetime, sent_at datetime, is_read integer not null default 0,
			read_at datetime, deleted_at datetime, created_at datetime, updated_at datetime, UNIQUE(user_id, batch_id)
		)`,
		`CREATE TABLE notification_preferences (
			id integer primary key autoincrement, user_id integer not null default 0, scene text not null default '',
			in_app_enabled integer not null default 1, sms_enabled integer not null default 0,
			lead_minutes integer not null default 1440,
			created_at datetime, updated_at datetime, UNIQUE(user_id, scene)
		)`,
		`CREATE TABLE sms_deliveries (
			id integer primary key autoincrement, idempotency_key text not null default '' unique, scene text not null default '',
			user_id integer not null default 0, business_type text not null default '', business_id integer not null default 0,
			business_version text not null default '', variables text, recipient_masked text not null default '',
			status text not null default 'pending', attempt_count integer not null default 0, max_attempts integer not null default 3,
			next_attempt_at datetime, last_attempt_at datetime, accepted_at datetime, provider text not null default '',
			provider_request_id text not null default '', provider_message_id text not null default '', provider_code text not null default '',
			last_error text not null default '', created_at datetime, updated_at datetime
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create reminder test schema: %v", err)
		}
	}
	return db
}

func seedReminderUser(t *testing.T, db *gorm.DB, id uint, status uint8) {
	t.Helper()
	phone := fmt.Sprintf("+861380%07d", id)
	if err := db.Exec("INSERT INTO admin_user (id, status, full_phone_number, deleted_at) VALUES (?, ?, ?, 0)", id, status, phone).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func seedReminderTodo(t *testing.T, db *gorm.DB, userID uint, title, status string, dueAt time.Time) *model.UserTodo {
	t.Helper()
	item := model.NewUserTodo()
	item.AssigneeID = userID
	item.CreatorID = userID
	item.SourceType = model.TodoSourceManual
	item.SourceKey = title
	item.Title = title
	item.Priority = model.TodoPriorityNormal
	item.Status = status
	item.DueAt = nil
	if err := db.Exec(`INSERT INTO user_todos
		(assignee_id, creator_id, source_type, source_key, title, priority, status, action_url, due_at, deleted_at)
		VALUES (?, ?, 'manual', ?, ?, 'normal', ?, '', ?, 0)`, userID, userID, title, title, status, dueAt).Error; err != nil {
		t.Fatalf("seed todo: %v", err)
	}
	if err := db.Order("id DESC").First(item).Error; err != nil {
		t.Fatalf("load seeded todo: %v", err)
	}
	return item
}
