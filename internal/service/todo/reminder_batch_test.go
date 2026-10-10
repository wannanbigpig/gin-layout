package todo

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm/logger"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/sms"
)

type reminderQueryCounter struct {
	logger.Interface
	queries []string
}

func (l *reminderQueryCounter) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	l.queries = append(l.queries, sql)
}

func (l *reminderQueryCounter) inserts(table string) int {
	count := 0
	for _, sql := range l.queries {
		if strings.HasPrefix(sql, "INSERT INTO `"+table+"`") {
			count++
		}
	}
	return count
}

func TestReminderRepeatedDeliveredBatchAvoidsPerTodoSQL(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.Local)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	for i := 0; i < 10; i++ {
		seedReminderTodo(t, db, 7, fmt.Sprintf("overdue-%d", i), model.TodoStatusPending, now.Add(-time.Hour))
	}
	svc := newReminderTestService(db, now, nil)
	options := ReminderOptions{LeadTime: 7 * 24 * time.Hour, CatchupWindow: 7 * 24 * time.Hour}
	if _, err := svc.Run(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	counter := &reminderQueryCounter{Interface: logger.Default}
	db.Config.Logger = counter
	result, err := svc.Run(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 10 || result.Handled != 10 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(counter.queries) > 5 || counter.inserts("sys_messages") != 0 {
		t.Fatalf("delivered batch still performs per-todo work: SQL=%d inserts=%d", len(counter.queries), counter.inserts("sys_messages"))
	}
	t.Logf("repeated batch: SQL=%d messages inserted=%d", len(counter.queries), counter.inserts("sys_messages"))
}

func TestReminderEnablingSMSAfterInAppOnlyQueuesMissingChannel(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.Local)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	seedReminderPreference(t, db, 7, true, false)
	seedReminderTodo(t, db, 7, "channels", model.TodoStatusPending, now.Add(time.Hour))
	sender := &fakeReminderSMSSender{result: sms.SendResult{State: sms.SendStateAccepted}}
	svc := newReminderSMSTestService(db, func() time.Time { return now }, sender)
	options := ReminderOptions{LeadTime: 2 * time.Hour, SMSEnabled: true}
	if _, err := svc.Run(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.NotificationPreference{}).Where("user_id = ?", 7).Update("sms_enabled", 1).Error; err != nil {
		t.Fatal(err)
	}
	counter := &reminderQueryCounter{Interface: logger.Default}
	db.Config.Logger = counter
	result, err := svc.Run(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if result.Handled != 1 || result.SMSQueued != 1 || sender.calls != 1 || counter.inserts("sys_messages") != 0 {
		t.Fatalf("missing channel not delivered independently: result=%+v sends=%d message inserts=%d", result, sender.calls, counter.inserts("sys_messages"))
	}
}

func TestReminderEnablingInAppPreservesExistingSMSOutcome(t *testing.T) {
	for _, status := range []string{model.SMSDeliveryStatusAccepted, model.SMSDeliveryStatusUnknown, model.SMSDeliveryStatusFailed, model.SMSDeliveryStatusCanceled} {
		t.Run(status, func(t *testing.T) {
			db := newReminderTestDB(t)
			now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.Local)
			seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
			seedReminderPreference(t, db, 7, false, true)
			item := seedReminderTodo(t, db, 7, "sms state", model.TodoStatusPending, now.Add(time.Hour))
			if _, err := persistTodoSMSDelivery(db, item, now); err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&model.SMSDelivery{}).Where("business_id = ?", item.ID).Update("status", status).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&model.NotificationPreference{}).Where("user_id = ?", 7).Update("in_app_enabled", 1).Error; err != nil {
				t.Fatal(err)
			}
			sender := &fakeReminderSMSSender{result: sms.SendResult{State: sms.SendStateAccepted}}
			svc := newReminderSMSTestService(db, func() time.Time { return now }, sender)
			counter := &reminderQueryCounter{Interface: logger.Default}
			db.Config.Logger = counter
			result, err := svc.Run(context.Background(), ReminderOptions{LeadTime: 2 * time.Hour, SMSEnabled: true})
			if err != nil {
				t.Fatal(err)
			}
			if result.Handled != 1 || result.SMSQueued != 0 || sender.calls != 0 || counter.inserts("sms_deliveries") != 0 || counter.inserts("sys_messages") != 1 {
				t.Fatalf("existing SMS outcome changed: result=%+v sends=%d sms inserts=%d messages=%d", result, sender.calls, counter.inserts("sms_deliveries"), counter.inserts("sys_messages"))
			}
		})
	}
}

func TestReminderDeletedMessageStaysDeletedAndOverdueStartsNewCycle(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.Local)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	seedReminderTodo(t, db, 7, "cycle", model.TodoStatusPending, now.Add(time.Minute))
	svc := newReminderTestService(db, now, nil)
	options := ReminderOptions{LeadTime: time.Hour, CatchupWindow: time.Hour}
	if _, err := svc.Run(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.SysMessage{}).Where("user_id = ?", 7).Update("deleted_at", now).Error; err != nil {
		t.Fatal(err)
	}
	counter := &reminderQueryCounter{Interface: logger.Default}
	db.Config.Logger = counter
	if _, err := svc.Run(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	if counter.inserts("sys_messages") != 0 {
		t.Fatal("deleted message must still count as delivered")
	}
	svc.now = func() time.Time { return now.Add(2 * time.Minute) }
	if _, err := svc.Run(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	var messages []model.SysMessage
	if err := db.Order("id").Find(&messages).Error; err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[0].DeletedAt == nil || messages[1].Topic != model.MessageTopicTodoOverdue {
		t.Fatalf("unexpected reminder cycles: %+v", messages)
	}
}

func TestReminderDeliveredBatchStillProcessesNewTodoOnNextPage(t *testing.T) {
	db := newReminderTestDB(t)
	db.Config.Logger = logger.Default.LogMode(logger.Silent)
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.Local)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	svc := newReminderTestService(db, now, nil)
	for i := 0; i < reminderPageSize; i++ {
		seedReminderTodo(t, db, 7, fmt.Sprintf("already delivered-%d", i), model.TodoStatusPending, now.Add(-time.Hour))
	}
	options := ReminderOptions{CatchupWindow: 2 * time.Hour}
	if _, err := svc.Run(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	seedReminderTodo(t, db, 7, "new on second page", model.TodoStatusPending, now.Add(-time.Hour))
	counter := &reminderQueryCounter{Interface: logger.Default}
	db.Config.Logger = counter
	result, err := svc.Run(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != reminderPageSize+1 || result.Handled != reminderPageSize+1 || counter.inserts("sys_messages") != 1 {
		t.Fatalf("second page was skipped or previous page rewritten: result=%+v inserts=%d", result, counter.inserts("sys_messages"))
	}
}

func TestReminderBatchHonorsZeroAndThirtyDayLeadTimes(t *testing.T) {
	t.Run("zero only notifies once due", func(t *testing.T) {
		db := newReminderTestDB(t)
		now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.Local)
		seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
		seedReminderTodo(t, db, 7, "due later", model.TodoStatusPending, now.Add(time.Minute))
		zero := 0
		if _, err := NewServiceWithDB(db).UpdateReminderPreference(context.Background(), 7, true, false, &zero); err != nil {
			t.Fatal(err)
		}
		svc := newReminderTestService(db, now, nil)
		options := ReminderOptions{LeadTime: time.Duration(MaxReminderLeadMinutes) * time.Minute, CatchupWindow: time.Hour}
		result, err := svc.Run(context.Background(), options)
		if err != nil || result.Handled != 0 {
			t.Fatalf("zero lead must not notify early: result=%+v err=%v", result, err)
		}
		svc.now = func() time.Time { return now.Add(time.Minute) }
		result, err = svc.Run(context.Background(), options)
		if err != nil || result.Handled != 1 {
			t.Fatalf("zero lead must notify at due time: result=%+v err=%v", result, err)
		}
	})
	t.Run("thirty day boundary is reachable", func(t *testing.T) {
		db := newReminderTestDB(t)
		now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.Local)
		seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
		seedReminderTodo(t, db, 7, "thirty days", model.TodoStatusPending, now.Add(30*24*time.Hour))
		seedReminderTodo(t, db, 7, "past thirty days", model.TodoStatusPending, now.Add(30*24*time.Hour+time.Minute))
		minutes := 30 * 24 * 60
		if _, err := NewServiceWithDB(db).UpdateReminderPreference(context.Background(), 7, true, false, &minutes); err != nil {
			t.Fatal(err)
		}
		result, err := newReminderTestService(db, now, nil).Run(context.Background(), ReminderOptions{LeadTime: time.Duration(MaxReminderLeadMinutes) * time.Minute})
		if err != nil || result.Scanned != 1 || result.Handled != 1 {
			t.Fatalf("thirty day boundary: result=%+v err=%v", result, err)
		}
	})
}
