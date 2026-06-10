package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	auditsvc "github.com/wannanbigpig/gin-layout/internal/service/audit"
)

func findHandler(registry queue.Registry, taskType string) queue.Handler {
	for _, entry := range registry.Entries() {
		if entry.TaskType == taskType {
			return entry.Handler
		}
	}
	return nil
}

func TestNewAuditLogPayload(t *testing.T) {
	payload, err := NewAuditLogPayload(AuditLogKindRequest, &auditsvc.AuditLogSnapshot{RequestID: "req-1"})
	if err != nil {
		t.Fatalf("NewAuditLogPayload returned error: %v", err)
	}
	if payload.Kind != AuditLogKindRequest {
		t.Fatalf("expected kind %q, got %q", AuditLogKindRequest, payload.Kind)
	}
	if payload.Snapshot == nil || payload.Snapshot.RequestID != "req-1" {
		t.Fatalf("unexpected snapshot: %#v", payload.Snapshot)
	}
}

func TestAuditLogHandlerReturnsSkipRetryForInvalidPayload(t *testing.T) {
	registry := queue.NewRegistry()
	RegisterAll(registry)

	handler := findHandler(registry, AuditLogTaskType)
	if handler == nil {
		t.Fatalf("handler for %s not found", AuditLogTaskType)
	}

	err := handler(context.Background(), []byte(`{"kind":"invalid"}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, queue.ErrSkipRetry) {
		t.Fatalf("expected skip retry error, got %v", err)
	}
}

func TestAuditLogHandlerPersistsSnapshot(t *testing.T) {
	called := false
	deps := AuditLogHandlerDeps{
		Persist: func(snapshot *auditsvc.AuditLogSnapshot) error {
			called = true
			if snapshot == nil || snapshot.RequestID != "req-2" {
				t.Fatalf("unexpected snapshot: %#v", snapshot)
			}
			return nil
		},
	}

	registry := queue.NewRegistry()
	RegisterAllWithDeps(registry, deps)

	handler := findHandler(registry, AuditLogTaskType)
	if handler == nil {
		t.Fatalf("handler for %s not found", AuditLogTaskType)
	}

	payload, err := NewAuditLogPayload(AuditLogKindPanic, &auditsvc.AuditLogSnapshot{RequestID: "req-2"})
	if err != nil {
		t.Fatalf("NewAuditLogPayload returned error: %v", err)
	}

	job := queue.NewJSONJob(AuditLogTaskType, AuditQueueName, payload)
	raw, err := job.Payload()
	if err != nil {
		t.Fatalf("Payload returned error: %v", err)
	}

	if err := handler(context.Background(), raw); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !called {
		t.Fatal("expected persist function to be called")
	}
}

func TestRegisterAllIncludesRequestLogExportHandler(t *testing.T) {
	registry := queue.NewRegistry()
	RegisterAll(registry)

	if findHandler(registry, TypeExportRequestLog) == nil {
		t.Fatalf("handler for %s not found", TypeExportRequestLog)
	}
}

func TestRegisterAllIncludesFileListExportHandler(t *testing.T) {
	registry := queue.NewRegistry()
	RegisterAll(registry)

	if findHandler(registry, TypeExportFileList) == nil {
		t.Fatalf("handler for %s not found", TypeExportFileList)
	}
}

func TestExportRequestLogQueueMatchesExportQueue(t *testing.T) {
	if ExportRequestLogQueue != ExportQueueName {
		t.Fatalf("unexpected export request log queue: got=%s want=%s", ExportRequestLogQueue, ExportQueueName)
	}
}

func TestExportFileListQueueMatchesExportQueue(t *testing.T) {
	if ExportFileListQueue != ExportQueueName {
		t.Fatalf("unexpected export file list queue: got=%s want=%s", ExportFileListQueue, ExportQueueName)
	}
}

func TestRecordExportRequestLogResultWritesEventMetaAndLeavesRunErrorMessage(t *testing.T) {
	db := newJobsTestDB(t)
	run := &model.TaskRun{
		TaskCode: TypeExportRequestLog,
		Kind:     model.TaskKindAsync,
		Source:   model.TaskSourceQueue,
		SourceID: "job-1",
		Queue:    ExportRequestLogQueue,
		Status:   model.TaskRunStatusRunning,
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create task run failed: %v", err)
	}

	uploadFile := &model.UploadFiles{
		UUID: "file-uuid",
	}
	uploadFile.ID = 12

	if err := recordExportRequestLogResultWithDB(context.Background(), db, run.ID, uploadFile, "request_logs.csv"); err != nil {
		t.Fatalf("record result returned error: %v", err)
	}

	var event model.TaskRunEvent
	if err := db.Where("run_id = ? AND event_type = ?", run.ID, exportRequestLogResultType).First(&event).Error; err != nil {
		t.Fatalf("query result event failed: %v", err)
	}
	if event.Message != "export file generated" {
		t.Fatalf("unexpected event message: %s", event.Message)
	}

	var meta map[string]any
	if err := json.Unmarshal([]byte(event.Meta), &meta); err != nil {
		t.Fatalf("unmarshal result meta failed: %v", err)
	}
	if meta["file_uuid"] != "file-uuid" || meta["file_name"] != "request_logs.csv" || meta["file_id"].(float64) != 12 {
		t.Fatalf("unexpected result meta: %#v", meta)
	}
	if _, ok := meta["file_size"]; !ok {
		t.Fatalf("expected file_size in result meta: %#v", meta)
	}

	var stored model.TaskRun
	if err := db.First(&stored, run.ID).Error; err != nil {
		t.Fatalf("query task run failed: %v", err)
	}
	if stored.ErrorMessage != "" {
		t.Fatalf("successful export result must not use error_message, got %q", stored.ErrorMessage)
	}
}

func newJobsTestDB(t *testing.T) *gorm.DB {
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
		`CREATE TABLE sys_export_records (
			id integer primary key autoincrement,
			created_at datetime,
			updated_at datetime,
			scene text,
			task_run_id integer,
			trigger_user_id integer,
			status text,
			queue text,
			progress integer,
			total_count integer,
			processed_count integer,
			file_id integer,
			file_uuid text,
			file_name text,
			file_size integer,
			format text,
			error_message text,
			started_at datetime,
			finished_at datetime
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create jobs test table failed: %v", err)
		}
	}
	return db
}
