package taskcenter

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

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
	var stat model.TaskRunStat
	if err := db.Where("task_code = ?", "demo:send").First(&stat).Error; err != nil {
		t.Fatalf("query task run stat failed: %v", err)
	}
	if stat.TotalCount != 1 || stat.SuccessCount != 1 || stat.SampledSuccessCount != 1 {
		t.Fatalf("unexpected task run stat: %#v", stat)
	}
}

func TestRecorderSuccessWithoutDetailStillUpdatesStats(t *testing.T) {
	db := newTaskCenterTestDB(t)
	insertTaskDefinition(t, db, "audit:request_log.write", model.TaskRecordSuccessNone)
	recorder := NewRecorderWithDB(db)

	run, err := recorder.Start(context.Background(), RunStart{
		TaskCode: "audit:request_log.write",
		Kind:     model.TaskKindAsync,
		Source:   model.TaskSourceQueue,
		SourceID: "task-1",
		Queue:    "audit",
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if err := recorder.Finish(context.Background(), run, RunFinish{}); err != nil {
		t.Fatalf("Finish returned error: %v", err)
	}

	var count int64
	if err := db.Model(&model.TaskRun{}).Where("id = ?", run.ID).Count(&count).Error; err != nil {
		t.Fatalf("count task runs failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected success detail to be deleted, got %d", count)
	}
	if err := db.Model(&model.TaskRunEvent{}).Where("run_id = ?", run.ID).Count(&count).Error; err != nil {
		t.Fatalf("count task events failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected success events to be deleted, got %d", count)
	}
	var stat model.TaskRunStat
	if err := db.Where("task_code = ?", "audit:request_log.write").First(&stat).Error; err != nil {
		t.Fatalf("query task run stat failed: %v", err)
	}
	if stat.TotalCount != 1 || stat.SuccessCount != 1 || stat.SampledSuccessCount != 0 {
		t.Fatalf("unexpected task run stat: %#v", stat)
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

func TestRecorderFailureAlwaysKeepsDetail(t *testing.T) {
	db := newTaskCenterTestDB(t)
	insertTaskDefinition(t, db, "audit:request_log.write", model.TaskRecordSuccessNone)
	recorder := NewRecorderWithDB(db)

	run, err := recorder.Start(context.Background(), RunStart{
		TaskCode: "audit:request_log.write",
		Kind:     model.TaskKindAsync,
		Source:   model.TaskSourceQueue,
		SourceID: "task-1",
		Queue:    "audit",
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if err := recorder.Finish(context.Background(), run, RunFinish{Error: errors.New("boom")}); err != nil {
		t.Fatalf("Finish returned error: %v", err)
	}

	var stored model.TaskRun
	if err := db.First(&stored, run.ID).Error; err != nil {
		t.Fatalf("query task run failed: %v", err)
	}
	if stored.Status != model.TaskRunStatusFailed || stored.DetailRecordMode != model.TaskDetailRecordForced {
		t.Fatalf("unexpected failed run: %#v", stored)
	}
	var stat model.TaskRunStat
	if err := db.Where("task_code = ?", "audit:request_log.write").First(&stat).Error; err != nil {
		t.Fatalf("query task run stat failed: %v", err)
	}
	if stat.TotalCount != 1 || stat.FailedCount != 1 {
		t.Fatalf("unexpected task run stat: %#v", stat)
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

func TestRecorderEnqueuePersistsRetryChain(t *testing.T) {
	db := newTaskCenterTestDB(t)
	recorder := NewRecorderWithDB(db)

	run, err := recorder.Enqueue(context.Background(), RunStart{
		TaskCode:       "demo:send",
		Kind:           model.TaskKindAsync,
		Source:         model.TaskSourceManual,
		SourceID:       "retry:101:1",
		MaxRetry:       3,
		RetryOfRunID:   101,
		RetryRootRunID: 88,
		RetrySeq:       2,
	})
	if err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}

	var stored model.TaskRun
	if err := db.First(&stored, run.ID).Error; err != nil {
		t.Fatalf("query task run failed: %v", err)
	}
	if stored.RetryOfRunID != 101 || stored.RetryRootRunID != 88 || stored.RetrySeq != 2 {
		t.Fatalf("unexpected retry chain fields: %#v", stored)
	}

	var event model.TaskRunEvent
	if err := db.Where("run_id = ? AND event_type = ?", run.ID, model.TaskEventEnqueue).First(&event).Error; err != nil {
		t.Fatalf("query enqueue event failed: %v", err)
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(event.Meta), &meta); err != nil {
		t.Fatalf("unmarshal enqueue meta failed: %v", err)
	}
	if meta["retry_of_run_id"] != float64(101) || meta["retry_root_run_id"] != float64(88) || meta["retry_seq"] != float64(2) {
		t.Fatalf("unexpected retry meta: %#v", meta)
	}
}

func TestRecorderStartPreservesRetryChainWhenReusingPendingRun(t *testing.T) {
	db := newTaskCenterTestDB(t)
	recorder := NewRecorderWithDB(db)

	queued, err := recorder.Enqueue(context.Background(), RunStart{
		TaskCode:       "demo:send",
		Kind:           model.TaskKindAsync,
		Source:         model.TaskSourceManual,
		SourceID:       "retry:88:2",
		Queue:          "default",
		MaxRetry:       3,
		RetryOfRunID:   101,
		RetryRootRunID: 88,
		RetrySeq:       2,
	})
	if err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}

	started, err := recorder.Start(context.Background(), RunStart{
		TaskCode: "demo:send",
		Kind:     model.TaskKindAsync,
		Source:   model.TaskSourceQueue,
		SourceID: "retry:88:2",
		Queue:    "default",
		Attempt:  1,
		MaxRetry: 3,
		Payload:  []byte(`{"retry":true}`),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if started.ID != queued.ID {
		t.Fatalf("expected start to reuse queued run %d, got %d", queued.ID, started.ID)
	}

	var stored model.TaskRun
	if err := db.First(&stored, queued.ID).Error; err != nil {
		t.Fatalf("query task run failed: %v", err)
	}
	if stored.Status != model.TaskRunStatusRunning {
		t.Fatalf("unexpected status: %s", stored.Status)
	}
	if stored.Source != model.TaskSourceManual {
		t.Fatalf("expected source to stay manual, got %s", stored.Source)
	}
	if stored.RetryOfRunID != 101 || stored.RetryRootRunID != 88 || stored.RetrySeq != 2 {
		t.Fatalf("retry chain fields were not preserved: %#v", stored)
	}

	var event model.TaskRunEvent
	if err := db.Where("run_id = ? AND event_type = ?", queued.ID, model.TaskEventStart).First(&event).Error; err != nil {
		t.Fatalf("query start event failed: %v", err)
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(event.Meta), &meta); err != nil {
		t.Fatalf("unmarshal start meta failed: %v", err)
	}
	if meta["retry_of_run_id"] != float64(101) || meta["retry_root_run_id"] != float64(88) || meta["retry_seq"] != float64(2) {
		t.Fatalf("unexpected start retry meta: %#v", meta)
	}
}

func TestTaskRunEventsReturnsCreatedAtAscending(t *testing.T) {
	db := newTaskCenterTestDB(t)
	events := []model.TaskRunEvent{
		{RunID: 7, EventType: model.TaskEventStart, Message: "start"},
		{RunID: 7, EventType: model.TaskEventSuccess, Message: "success"},
	}
	for i := range events {
		if err := db.Create(&events[i]).Error; err != nil {
			t.Fatalf("create event failed: %v", err)
		}
	}

	result, err := NewTaskCenterServiceWithDeps(TaskCenterServiceDeps{DB: db}).TaskRunEvents(7)
	if err != nil {
		t.Fatalf("TaskRunEvents returned error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 events, got %d", len(result))
	}
	if result[0].EventType != model.TaskEventStart || result[1].EventType != model.TaskEventSuccess {
		t.Fatalf("unexpected event order: %#v", result)
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
			retry_of_run_id integer,
			retry_root_run_id integer,
			retry_seq integer,
			payload text,
			error_message text,
			started_at datetime,
			finished_at datetime,
			duration_ms real,
			detail_record_mode text
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
			last_success_at datetime,
			last_failed_at datetime,
			last_stats_window_start datetime,
			last_error text
		)`,
		`CREATE TABLE task_definitions (
			id integer primary key autoincrement,
			created_at datetime,
			updated_at datetime,
			deleted_at integer default 0,
			code text,
			name text,
			kind text,
			queue text,
			cron_spec text,
			handler text,
			status integer,
			allow_manual integer,
			allow_retry integer,
			is_high_risk integer,
			record_success_mode text,
			record_success_rate integer,
			record_success_interval_seconds integer,
			record_detail_on_manual integer,
			record_detail_on_failure integer,
			remark text
		)`,
		`CREATE TABLE task_run_stats (
			id integer primary key autoincrement,
			created_at datetime,
			updated_at datetime,
			task_code text,
			kind text,
			source text,
			window_start datetime,
			window_size text,
			total_count integer default 0,
			success_count integer default 0,
			failed_count integer default 0,
			canceled_count integer default 0,
			timeout_count integer default 0,
			interrupted_count integer default 0,
			sampled_success_count integer default 0,
			duration_total_ms real default 0,
			duration_max_ms real default 0,
			unique(task_code, kind, source, window_start, window_size)
		)`,
		`CREATE TABLE admin_user (
			id integer primary key autoincrement,
			created_at datetime,
			updated_at datetime,
			deleted_at integer default 0,
			username text,
			nickname text
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create test table failed: %v", err)
		}
	}
	return db
}

func insertTaskDefinition(t *testing.T, db *gorm.DB, code string, successMode string) {
	t.Helper()
	now := time.Now()
	if err := db.Exec(`INSERT INTO task_definitions (created_at, updated_at, deleted_at, code, name, kind, queue, status, allow_manual, allow_retry, is_high_risk, record_success_mode, record_success_rate, record_success_interval_seconds, record_detail_on_manual, record_detail_on_failure) VALUES (?, ?, 0, ?, ?, ?, ?, 1, 0, 1, 0, ?, 0, 0, 1, 1)`,
		now, now, code, code, model.TaskKindAsync, "default", successMode).Error; err != nil {
		t.Fatalf("insert task definition failed: %v", err)
	}
}
