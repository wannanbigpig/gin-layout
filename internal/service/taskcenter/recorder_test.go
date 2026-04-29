package taskcenter

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestRecorderStartAndFinishSuccess(t *testing.T) {
	db := newTaskCenterTestDB(t)
	recorder := NewRecorderWithDB(db)

	run, err := recorder.Start(context.Background(), RunStart{
		TaskCode: "demo:send",
		Kind:     model.TaskKindAsync,
		Source:   model.TaskSourceQueue,
		SourceID: "task-1",
		Queue:    "default",
		Attempt:  1,
		MaxRetry: 3,
		Payload:  []byte(`{"name":"codex"}`),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if run.ID == 0 {
		t.Fatal("expected run id to be assigned")
	}

	if err := recorder.Finish(context.Background(), run, RunFinish{}); err != nil {
		t.Fatalf("Finish returned error: %v", err)
	}

	var stored model.TaskRun
	if err := db.First(&stored, run.ID).Error; err != nil {
		t.Fatalf("query task run failed: %v", err)
	}
	if stored.Status != model.TaskRunStatusSuccess {
		t.Fatalf("unexpected status: %s", stored.Status)
	}
	if stored.Payload != `{"name":"codex"}` {
		t.Fatalf("unexpected payload: %s", stored.Payload)
	}

	var count int64
	if err := db.Model(&model.TaskRunEvent{}).Where("run_id = ?", run.ID).Count(&count).Error; err != nil {
		t.Fatalf("count events failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 events, got %d", count)
	}
}

func TestRecorderFinishFailureUpdatesCronState(t *testing.T) {
	db := newTaskCenterTestDB(t)
	recorder := NewRecorderWithDB(db)

	run, err := recorder.Start(context.Background(), RunStart{
		TaskCode: "cron:demo",
		Kind:     model.TaskKindCron,
		Source:   model.TaskSourceCron,
		SourceID: "demo",
		CronSpec: "0/5 * * * * *",
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	taskErr := errors.New("boom")
	if err := recorder.Finish(context.Background(), run, RunFinish{Error: taskErr}); err != nil {
		t.Fatalf("Finish returned error: %v", err)
	}

	var stored model.TaskRun
	if err := db.First(&stored, run.ID).Error; err != nil {
		t.Fatalf("query task run failed: %v", err)
	}
	if stored.Status != model.TaskRunStatusFailed {
		t.Fatalf("unexpected status: %s", stored.Status)
	}
	if stored.ErrorMessage != "boom" {
		t.Fatalf("unexpected error message: %s", stored.ErrorMessage)
	}

	var state model.CronTaskState
	if err := db.Where("task_code = ?", "cron:demo").First(&state).Error; err != nil {
		t.Fatalf("query cron state failed: %v", err)
	}
	if state.LastRunID != run.ID || state.LastStatus != model.TaskRunStatusFailed {
		t.Fatalf("unexpected cron state: %#v", state)
	}
	if state.LastError != "boom" {
		t.Fatalf("unexpected cron state error: %s", state.LastError)
	}

	var event model.TaskRunEvent
	if err := db.Where("run_id = ? AND event_type = ?", run.ID, model.TaskEventFail).First(&event).Error; err != nil {
		t.Fatalf("query fail event failed: %v", err)
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(event.Meta), &meta); err != nil {
		t.Fatalf("unmarshal fail meta failed: %v", err)
	}
	if meta["status"] != model.TaskRunStatusFailed {
		t.Fatalf("unexpected fail meta: %#v", meta)
	}
}

func TestRecorderFinishCancelWritesOperatorMeta(t *testing.T) {
	db := newTaskCenterTestDB(t)
	recorder := NewRecorderWithDB(db)

	run, err := recorder.Enqueue(context.Background(), RunStart{
		TaskCode:       "demo:send",
		Kind:           model.TaskKindAsync,
		Source:         model.TaskSourceManual,
		SourceID:       "task-1",
		TriggerUserID:  7,
		TriggerAccount: "starter",
		TriggerConfirm: "CONFIRM",
		TriggerReason:  "manual run",
	})
	if err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}

	if err := recorder.Finish(context.Background(), run, RunFinish{
		Status:            model.TaskRunStatusCanceled,
		CanceledBy:        9,
		CanceledByAccount: "operator",
		CancelReason:      "manual cancel",
	}); err != nil {
		t.Fatalf("Finish returned error: %v", err)
	}

	var event model.TaskRunEvent
	if err := db.Where("run_id = ? AND event_type = ?", run.ID, model.TaskEventCancel).First(&event).Error; err != nil {
		t.Fatalf("query cancel event failed: %v", err)
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(event.Meta), &meta); err != nil {
		t.Fatalf("unmarshal cancel meta failed: %v", err)
	}
	if meta["canceled_by_account"] != "operator" || meta["cancel_reason"] != "manual cancel" {
		t.Fatalf("unexpected cancel meta: %#v", meta)
	}

	var enqueueEvent model.TaskRunEvent
	if err := db.Where("run_id = ? AND event_type = ?", run.ID, model.TaskEventEnqueue).First(&enqueueEvent).Error; err != nil {
		t.Fatalf("query enqueue event failed: %v", err)
	}
	var enqueueMeta map[string]any
	if err := json.Unmarshal([]byte(enqueueEvent.Meta), &enqueueMeta); err != nil {
		t.Fatalf("unmarshal enqueue meta failed: %v", err)
	}
	if enqueueMeta["trigger_account"] != "starter" || enqueueMeta["trigger_confirm"] != "CONFIRM" || enqueueMeta["trigger_reason"] != "manual run" {
		t.Fatalf("unexpected enqueue meta: %#v", enqueueMeta)
	}
}

func newTaskCenterTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	statements := []string{
		`CREATE TABLE task_runs (
			id integer primary key autoincrement,
			created_at datetime,
			updated_at datetime,
			task_code text,
			kind text,
			source text,
			source_id text,
			queue text,
			trigger_user_id integer,
			trigger_account text,
			status text,
			attempt integer,
			max_retry integer,
			payload text,
			error_message text,
			started_at datetime,
			finished_at datetime,
			duration_ms real
		)`,
		`CREATE TABLE task_run_events (
			id integer primary key autoincrement,
			created_at datetime,
			updated_at datetime,
			run_id integer,
			event_type text,
			message text,
			meta text
		)`,
		`CREATE TABLE cron_task_states (
			id integer primary key autoincrement,
			created_at datetime,
			updated_at datetime,
			task_code text unique,
			cron_spec text,
			last_run_id integer,
			last_status text,
			last_started_at datetime,
			last_finished_at datetime,
			next_run_at datetime,
			last_error text
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create test table failed: %v", err)
		}
	}
	return db
}
