package todo

import (
	"context"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/sms"
	"gorm.io/gorm/logger"
	"testing"
	"time"
)

func TestInvalidRecipientDoesNotStarveSMSOutbox(t *testing.T) {
	db := newReminderTestDB(t)
	db.Config.Logger = logger.Default.LogMode(logger.Silent)
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.Local)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	seedReminderUser(t, db, 8, model.AdminUserStatusEnabled)
	seedReminderPreference(t, db, 7, false, true)
	seedReminderPreference(t, db, 8, false, true)
	for i := 0; i < 100; i++ {
		item := seedReminderTodo(t, db, 7, "disabled recipient pending", model.TodoStatusPending, now.Add(-time.Hour))
		if _, err := persistTodoSMSDelivery(db, item, now); err != nil {
			t.Fatal(err)
		}
	}
	valid := seedReminderTodo(t, db, 8, "valid recipient", model.TodoStatusPending, now.Add(-time.Hour))
	if _, err := persistTodoSMSDelivery(db, valid, now); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE admin_user SET status = ? WHERE id = 7", model.AdminUserStatusDisabled).Error; err != nil {
		t.Fatal(err)
	}
	sender := &fakeReminderSMSSender{result: sms.SendResult{State: sms.SendStateAccepted}}
	svc := newReminderSMSTestService(db, func() time.Time { return now }, sender)
	for i := 0; i < 2; i++ {
		_, err := svc.processSMSOutbox(context.Background(), db, now)
		if err != nil {
			t.Fatal(err)
		}
	}
	var pending int64
	db.Model(&model.SMSDelivery{}).Where("status = ?", model.SMSDeliveryStatusPending).Count(&pending)
	t.Logf("remaining pending=%d", pending)
	if pending != 0 || sender.calls != 1 {
		t.Fatalf("pending=%d sends=%d; valid recipient must be sent", pending, sender.calls)
	}
	var canceled int64
	if err := db.Model(&model.SMSDelivery{}).Where("status = ?", model.SMSDeliveryStatusCanceled).Count(&canceled).Error; err != nil {
		t.Fatal(err)
	}
	if canceled != 100 {
		t.Fatalf("invalid recipients canceled=%d, want 100", canceled)
	}
}
