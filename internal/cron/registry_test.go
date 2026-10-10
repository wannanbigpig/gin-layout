package taskcron

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/jobs"
	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestSyncBuiltinDefinitionsUpsert(t *testing.T) {
	db := newTaskDefinitionSyncTestDB(t)

	cfg := &config.Conf{}
	cfg.EnableResetSystemCron = false
	report, err := syncBuiltinDefinitionsWithReport(db, cfg)
	if err != nil {
		t.Fatalf("sync builtin definitions failed: %v", err)
	}
	if report.Total != 9 || report.Inserted != 9 || report.Updated != 0 || report.Unchanged != 0 {
		t.Fatalf("unexpected first sync report: %+v", report)
	}

	assertTaskDefinitionCount(t, db, 9)
	assertTaskStatus(t, db, TaskCodeCronResetSystemData, 0)
	assertTaskStatus(t, db, TaskCodeCronDemo, 0)
	assertTaskStatus(t, db, TaskCodeCronFileAccessFlush, 1)
	assertTaskStatus(t, db, TaskCodeCronCleanExpiredFiles, 1)
	assertTaskStatus(t, db, TaskCodeCronCleanAuditLogs, 1)
	assertTaskStatus(t, db, TaskCodeCronTodoReminder, 1)
	var todoReminder model.TaskDefinition
	if err := db.Where("code = ?", TaskCodeCronTodoReminder).First(&todoReminder).Error; err != nil || todoReminder.CronSpec != "0 * * * * *" {
		t.Fatalf("todo reminder must scan each minute: %#v err=%v", todoReminder, err)
	}
	assertTaskStatus(t, db, jobs.AuditLogTaskType, 1)
	assertTaskStatus(t, db, jobs.TypeExportRequestLog, 1)
	assertTaskStatus(t, db, jobs.TypeExportFileList, 1)
	assertTaskQueue(t, db, jobs.TypeExportRequestLog, jobs.ExportRequestLogQueue)
	assertTaskQueue(t, db, jobs.TypeExportFileList, jobs.ExportFileListQueue)
	assertTaskAllowManual(t, db, TaskCodeCronDemo, 1)
	assertTaskAllowManual(t, db, TaskCodeCronResetSystemData, 1)
	assertTaskAllowManual(t, db, TaskCodeCronFileAccessFlush, 1)
	assertTaskAllowManual(t, db, TaskCodeCronCleanExpiredFiles, 1)
	assertTaskAllowManual(t, db, TaskCodeCronCleanAuditLogs, 1)
	assertTaskAllowManual(t, db, TaskCodeCronTodoReminder, 1)
	assertTaskAllowManual(t, db, jobs.AuditLogTaskType, 0)
	assertTaskAllowManual(t, db, jobs.TypeExportRequestLog, 0)
	assertTaskAllowManual(t, db, jobs.TypeExportFileList, 0)
	assertTaskRecordSuccessMode(t, db, jobs.AuditLogTaskType, model.TaskRecordSuccessNone)
	assertTaskRecordSuccessMode(t, db, jobs.TypeExportRequestLog, model.TaskRecordSuccessAll)

	cfg.EnableResetSystemCron = true
	report, err = syncBuiltinDefinitionsWithReport(db, cfg)
	if err != nil {
		t.Fatalf("sync builtin definitions on second run failed: %v", err)
	}
	if report.Total != 9 || report.Inserted != 0 || report.Updated != 0 || report.Unchanged != 9 {
		t.Fatalf("unexpected second sync report: %+v", report)
	}

	assertTaskDefinitionCount(t, db, 9)
	assertTaskStatus(t, db, TaskCodeCronResetSystemData, 0)
}

func TestSyncBuiltinDefinitionsPreservesOperationalConfigAndUpdatesDefinitionFields(t *testing.T) {
	db := newTaskDefinitionSyncTestDB(t)
	now := "2026-06-09 10:00:00"
	if err := db.Exec(`INSERT INTO task_definitions (
		code, name, kind, queue, cron_spec, handler, status, allow_manual, allow_retry, is_high_risk,
		record_success_mode, record_success_rate, record_success_interval_seconds, record_detail_on_manual,
		record_detail_on_failure, remark, created_at, updated_at, deleted_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		TaskCodeCronCleanExpiredFiles, "旧名称", model.TaskKindCron, "old_queue", "0 * * * * *", "old.handler",
		model.TaskStatusDisabled, model.TaskManualNotAllowed, model.TaskRetryAllowed, model.TaskHighRisk,
		model.TaskRecordSuccessNone, 7, 123, 0, 0, "旧备注", now, now,
	).Error; err != nil {
		t.Fatalf("insert existing task definition failed: %v", err)
	}

	report, err := syncBuiltinDefinitionsWithReport(db, &config.Conf{})
	if err != nil {
		t.Fatalf("sync builtin definitions failed: %v", err)
	}
	if report.Total != 9 || report.Inserted != 8 || report.Updated != 1 || report.Unchanged != 0 {
		t.Fatalf("unexpected sync report: %+v", report)
	}

	var definition model.TaskDefinition
	if err := db.Where("code = ?", TaskCodeCronCleanExpiredFiles).First(&definition).Error; err != nil {
		t.Fatalf("query synced task definition failed: %v", err)
	}
	if definition.Name != "自动清理过期临时文件" ||
		definition.CronSpec != "0 * * * * *" ||
		definition.Handler != HandlerCronCleanExpiredFiles ||
		definition.Remark == "旧备注" {
		t.Fatalf("definition fields were not updated: %#v", definition)
	}
	if definition.Status != model.TaskStatusDisabled ||
		definition.AllowManual != model.TaskManualNotAllowed ||
		definition.AllowRetry != model.TaskRetryAllowed ||
		definition.RecordSuccessMode != model.TaskRecordSuccessNone ||
		definition.RecordSuccessRate != 7 ||
		definition.RecordSuccessIntervalSeconds != 123 ||
		definition.RecordDetailOnManual != 0 ||
		definition.RecordDetailOnFailure != 0 {
		t.Fatalf("operational config should be preserved: %#v", definition)
	}
}

func assertTaskDefinitionCount(t *testing.T, db *gorm.DB, expected int64) {
	t.Helper()

	var count int64
	if err := db.Model(&model.TaskDefinition{}).Count(&count).Error; err != nil {
		t.Fatalf("count task definitions failed: %v", err)
	}
	if count != expected {
		t.Fatalf("unexpected task definition count: got=%d want=%d", count, expected)
	}
}

func assertTaskStatus(t *testing.T, db *gorm.DB, code string, expected uint8) {
	t.Helper()

	var definition model.TaskDefinition
	if err := db.Where("code = ?", code).First(&definition).Error; err != nil {
		t.Fatalf("query task definition(%s) failed: %v", code, err)
	}
	if definition.Status != expected {
		t.Fatalf("unexpected status for %s: got=%d want=%d", code, definition.Status, expected)
	}
}

func assertTaskAllowManual(t *testing.T, db *gorm.DB, code string, expected uint8) {
	t.Helper()

	var definition model.TaskDefinition
	if err := db.Where("code = ?", code).First(&definition).Error; err != nil {
		t.Fatalf("query task definition(%s) failed: %v", code, err)
	}
	if definition.AllowManual != expected {
		t.Fatalf("unexpected allow_manual for %s: got=%d want=%d", code, definition.AllowManual, expected)
	}
}

func assertTaskQueue(t *testing.T, db *gorm.DB, code string, expected string) {
	t.Helper()

	var definition model.TaskDefinition
	if err := db.Where("code = ?", code).First(&definition).Error; err != nil {
		t.Fatalf("query task definition(%s) failed: %v", code, err)
	}
	if definition.Queue != expected {
		t.Fatalf("unexpected queue for %s: got=%s want=%s", code, definition.Queue, expected)
	}
}

func assertTaskRecordSuccessMode(t *testing.T, db *gorm.DB, code string, expected string) {
	t.Helper()

	var definition model.TaskDefinition
	if err := db.Where("code = ?", code).First(&definition).Error; err != nil {
		t.Fatalf("query task definition(%s) failed: %v", code, err)
	}
	if definition.RecordSuccessMode != expected {
		t.Fatalf("unexpected record_success_mode for %s: got=%s want=%s", code, definition.RecordSuccessMode, expected)
	}
}

func newTaskDefinitionSyncTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	statement := `
CREATE TABLE task_definitions (
    id integer primary key autoincrement,
    code text NOT NULL DEFAULT '',
    name text NOT NULL DEFAULT '',
    kind text NOT NULL DEFAULT '',
    queue text NOT NULL DEFAULT '',
    cron_spec text NOT NULL DEFAULT '',
    handler text NOT NULL DEFAULT '',
    status integer NOT NULL DEFAULT 1,
    allow_manual integer NOT NULL DEFAULT 0,
    allow_retry integer NOT NULL DEFAULT 1,
    is_high_risk integer NOT NULL DEFAULT 0,
    record_success_mode text NOT NULL DEFAULT 'all',
    record_success_rate integer NOT NULL DEFAULT 100,
    record_success_interval_seconds integer NOT NULL DEFAULT 0,
    record_detail_on_manual integer NOT NULL DEFAULT 1,
    record_detail_on_failure integer NOT NULL DEFAULT 1,
    remark text NOT NULL DEFAULT '',
    created_at datetime,
    updated_at datetime,
    deleted_at integer NOT NULL DEFAULT 0,
    UNIQUE(code, deleted_at)
)`
	if err := db.Exec(statement).Error; err != nil {
		t.Fatalf("create task_definitions table failed: %v", err)
	}
	return db
}
