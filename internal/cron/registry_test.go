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
	if err := syncBuiltinDefinitions(db, cfg); err != nil {
		t.Fatalf("sync builtin definitions failed: %v", err)
	}

	assertTaskDefinitionCount(t, db, 3)
	assertTaskStatus(t, db, TaskCodeCronResetSystemData, 0)
	assertTaskStatus(t, db, jobs.AuditLogTaskType, 1)
	assertTaskAllowManual(t, db, TaskCodeCronDemo, 1)
	assertTaskAllowManual(t, db, TaskCodeCronResetSystemData, 1)
	assertTaskAllowManual(t, db, jobs.AuditLogTaskType, 0)

	cfg.EnableResetSystemCron = true
	if err := syncBuiltinDefinitions(db, cfg); err != nil {
		t.Fatalf("sync builtin definitions on second run failed: %v", err)
	}

	assertTaskDefinitionCount(t, db, 3)
	assertTaskStatus(t, db, TaskCodeCronResetSystemData, 1)
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
