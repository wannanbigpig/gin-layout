package todo

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/sms"
)

func TestReminderSMSGlobalDisablePreservesPendingAndRetry(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.Local)
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	seedReminderPreference(t, db, 7, false, true)
	for _, status := range []string{model.SMSDeliveryStatusPending, model.SMSDeliveryStatusRetryWait} {
		item := seedReminderTodo(t, db, 7, status, model.TodoStatusPending, now.Add(time.Hour))
		if _, err := persistTodoSMSDelivery(db, item, now); err != nil {
			t.Fatal(err)
		}
		if status == model.SMSDeliveryStatusRetryWait {
			if err := db.Model(&model.SMSDelivery{}).Where("business_id = ?", item.ID).Updates(map[string]any{"status": status, "attempt_count": 1, "next_attempt_at": now}).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	sender := &fakeReminderSMSSender{result: sms.SendResult{State: sms.SendStateAccepted}}
	svc := newReminderSMSTestService(db, func() time.Time { return now }, sender)
	enabledFactory := svc.smsRuntimeFactory
	svc.smsRuntimeFactory = func() (reminderSMSRuntime, error) { return buildReminderSMSRuntime(sms.Config{Enabled: false}) }
	budgetCalls := 0
	svc.smsBudgetReserve = func(context.Context, int) error { budgetCalls++; return nil }
	result, err := svc.processSMSOutbox(context.Background(), db, now)
	if err != nil || result != (reminderSMSProcessResult{}) || budgetCalls != 0 || sender.calls != 0 {
		t.Fatalf("result=%+v err=%v budget=%d sends=%d", result, err, budgetCalls, sender.calls)
	}
	var deliveries []model.SMSDelivery
	if err := db.Order("id").Find(&deliveries).Error; err != nil {
		t.Fatal(err)
	}
	if len(deliveries) != 2 || deliveries[0].Status != model.SMSDeliveryStatusPending || deliveries[0].AttemptCount != 0 || deliveries[1].Status != model.SMSDeliveryStatusRetryWait || deliveries[1].AttemptCount != 1 {
		t.Fatalf("paused records changed: %+v", deliveries)
	}
	svc.smsRuntimeFactory = enabledFactory
	result, err = svc.processSMSOutbox(context.Background(), db, now)
	if err != nil || result.Accepted != 2 || sender.calls != 2 || budgetCalls != 2 {
		t.Fatalf("reenabled result=%+v err=%v budget=%d sends=%d", result, err, budgetCalls, sender.calls)
	}
}

func TestReminderSMSLocalRejectionDoesNotBecomeUnknown(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Now()
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	seedReminderPreference(t, db, 7, false, true)
	item := seedReminderTodo(t, db, 7, "local rejection", model.TodoStatusPending, now.Add(time.Hour))
	if _, err := persistTodoSMSDelivery(db, item, now); err != nil {
		t.Fatal(err)
	}
	dispatcher, err := sms.NewDispatcher(sms.Config{Routes: map[string]sms.Route{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	svc := newReminderSMSTestService(db, func() time.Time { return now }, dispatcher)
	result, err := svc.processSMSOutbox(context.Background(), db, now)
	if err == nil || result.Unknown != 0 || result.Failed != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	var delivery model.SMSDelivery
	if err := db.First(&delivery).Error; err != nil {
		t.Fatal(err)
	}
	if delivery.Status != model.SMSDeliveryStatusRetryWait {
		t.Fatalf("local rejection status=%s", delivery.Status)
	}
}

func TestReminderSMSShortenedLeadDefersWithoutLosingCycle(t *testing.T) {
	for _, retry := range []bool{false, true} {
		for _, lead := range []int{0, 30} {
			t.Run(fmt.Sprintf("retry_%t_lead_%d", retry, lead), func(t *testing.T) {
				db := newReminderTestDB(t)
				now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.Local)
				seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
				seedReminderPreference(t, db, 7, false, true)
				item := seedReminderTodo(t, db, 7, "changed lead", model.TodoStatusPending, now.Add(24*time.Hour))
				if _, err := persistTodoSMSDelivery(db, item, now); err != nil {
					t.Fatal(err)
				}
				sender := &fakeReminderSMSSender{result: sms.SendResult{State: sms.SendStateAccepted}}
				svc := newReminderSMSTestService(db, func() time.Time { return now }, sender)
				wantAttempts := uint(0)
				if retry {
					sender.result = sms.SendResult{State: sms.SendStateFailed}
					sender.err = errors.New("provider rejection")
					if _, err := svc.processSMSOutbox(context.Background(), db, now); err == nil {
						t.Fatal("expected first rejection")
					}
					wantAttempts = 1
					sender.result = sms.SendResult{State: sms.SendStateAccepted}
					sender.err = nil
				}
				preference, err := (&Service{db: db, now: func() time.Time { return now }}).UpdateReminderPreference(context.Background(), 7, false, true, &lead)
				if err != nil || preference.LeadMinutes != lead {
					t.Fatalf("preference=%+v err=%v", preference, err)
				}
				now = now.Add(5 * time.Minute)
				beforeCalls := sender.calls
				budgetCalls := 0
				svc.smsBudgetReserve = func(context.Context, int) error { budgetCalls++; return nil }
				if result, err := svc.processSMSOutbox(context.Background(), db, now); err != nil || result != (reminderSMSProcessResult{}) {
					t.Fatalf("deferred result=%+v err=%v", result, err)
				}
				var old model.SMSDelivery
				if err := db.First(&old).Error; err != nil {
					t.Fatal(err)
				}
				if old.AttemptCount != wantAttempts || old.Status == model.SMSDeliveryStatusCanceled || old.Status == model.SMSDeliveryStatusUnknown || sender.calls != beforeCalls || budgetCalls != 0 {
					t.Fatalf("pause lost record/attempt: %+v sends=%d budget=%d", old, sender.calls, budgetCalls)
				}
				now = item.DueAt.Time.Add(-time.Duration(lead) * time.Minute)
				result, err := svc.Run(context.Background(), ReminderOptions{LeadTime: 24 * time.Hour, CatchupWindow: time.Hour, SMSEnabled: true})
				if err != nil || result.SMSAccepted != 1 || sender.calls != beforeCalls+1 || budgetCalls != 1 {
					t.Fatalf("window result=%+v err=%v calls=%d budget=%d", result, err, sender.calls, budgetCalls)
				}
				if err := db.First(&old, old.ID).Error; err != nil {
					t.Fatal(err)
				}
				if lead == 0 {
					if old.Status != model.SMSDeliveryStatusCanceled || old.AttemptCount != wantAttempts {
						t.Fatalf("old due-soon cycle should yield to overdue: %+v", old)
					}
					var overdue model.SMSDelivery
					if err := db.Where("business_id = ? AND status = ?", item.ID, model.SMSDeliveryStatusAccepted).First(&overdue).Error; err != nil {
						t.Fatal(err)
					}
					if overdue.BusinessVersion != todoSMSBusinessVersion(item.DueAt.Time, "overdue") {
						t.Fatalf("unexpected cycle %+v", overdue)
					}
				} else if old.Status != model.SMSDeliveryStatusAccepted || old.AttemptCount != wantAttempts+1 {
					t.Fatalf("same cycle did not resume: %+v", old)
				}
			})
		}
	}
}

func TestReminderSMSDeferredPageDoesNotStarveReadyDelivery(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Now()
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	seedReminderPreference(t, db, 7, false, true)
	for i := 0; i < todoSMSBatchSize; i++ {
		item := seedReminderTodo(t, db, 7, fmt.Sprintf("deferred %d", i), model.TodoStatusPending, now.Add(time.Hour))
		if _, err := persistTodoSMSDelivery(db, item, now); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < todoSMSBatchSize+1; i++ {
		ready := seedReminderTodo(t, db, 7, fmt.Sprintf("ready overdue %d", i), model.TodoStatusPending, now.Add(-time.Minute))
		if _, err := persistTodoSMSDelivery(db, ready, now); err != nil {
			t.Fatal(err)
		}
	}
	zero := 0
	if _, err := (&Service{db: db}).UpdateReminderPreference(context.Background(), 7, false, true, &zero); err != nil {
		t.Fatal(err)
	}
	sender := &fakeReminderSMSSender{result: sms.SendResult{State: sms.SendStateAccepted}}
	svc := newReminderSMSTestService(db, func() time.Time { return now }, sender)
	result, err := svc.processSMSOutbox(context.Background(), db, now)
	if err != nil || result.Accepted != todoSMSBatchSize || sender.calls != todoSMSBatchSize {
		t.Fatalf("first batch result=%+v calls=%d err=%v", result, sender.calls, err)
	}
	result, err = svc.processSMSOutbox(context.Background(), db, now)
	if err != nil || result.Accepted != 1 || sender.calls != todoSMSBatchSize+1 {
		t.Fatalf("second batch result=%+v calls=%d err=%v", result, sender.calls, err)
	}
	var attempts int64
	if err := db.Model(&model.SMSDelivery{}).Select("COALESCE(SUM(attempt_count),0)").Scan(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if attempts != todoSMSBatchSize+1 {
		t.Fatalf("attempts=%d, want only ready items", attempts)
	}
}

func TestReminderSMSDeferredCycleRechecksIncreasedLead(t *testing.T) {
	db := newReminderTestDB(t)
	now := time.Now()
	seedReminderUser(t, db, 7, model.AdminUserStatusEnabled)
	seedReminderPreference(t, db, 7, false, true)
	item := seedReminderTodo(t, db, 7, "lead grows again", model.TodoStatusPending, now.Add(time.Hour))
	if _, err := persistTodoSMSDelivery(db, item, now); err != nil {
		t.Fatal(err)
	}
	preferenceService := &Service{db: db}
	zero := 0
	if _, err := preferenceService.UpdateReminderPreference(context.Background(), 7, false, true, &zero); err != nil {
		t.Fatal(err)
	}
	sender := &fakeReminderSMSSender{result: sms.SendResult{State: sms.SendStateAccepted}}
	svc := newReminderSMSTestService(db, func() time.Time { return now }, sender)
	if result, err := svc.processSMSOutbox(context.Background(), db, now); err != nil || result.Accepted != 0 {
		t.Fatalf("paused result=%+v err=%v", result, err)
	}
	longer := 120
	if _, err := preferenceService.UpdateReminderPreference(context.Background(), 7, false, true, &longer); err != nil {
		t.Fatal(err)
	}
	if result, err := svc.processSMSOutbox(context.Background(), db, now); err != nil || result.Accepted != 1 || sender.calls != 1 {
		t.Fatalf("resumed result=%+v err=%v calls=%d", result, err, sender.calls)
	}
}
