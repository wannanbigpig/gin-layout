package taskcron

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestGetAuditLogRetentionDaysUsesMinimumForPositiveSmallValue(t *testing.T) {
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.AuditLogRetentionDays = 3
	})
	defer restoreConfig()

	if got := getAuditLogRetentionDays(); got != auditLogMinRetentionDays {
		t.Fatalf("expected minimum retention %d, got %d", auditLogMinRetentionDays, got)
	}
}

func TestGetAuditLogRetentionDaysUsesDefaultForUnsetValue(t *testing.T) {
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.AuditLogRetentionDays = 0
	})
	defer restoreConfig()

	if got := getAuditLogRetentionDays(); got != auditLogDefaultRetention {
		t.Fatalf("expected default retention %d, got %d", auditLogDefaultRetention, got)
	}
}

func TestGetAuditLogRetentionDaysUsesDefaultWhenConfigNil(t *testing.T) {
	restoreConfig := config.ReplaceConfigForTesting(nil)
	defer restoreConfig()

	if got := getAuditLogRetentionDays(); got != auditLogDefaultRetention {
		t.Fatalf("expected default retention %d, got %d", auditLogDefaultRetention, got)
	}
}

func TestBatchDeleteByCutoffPhysicallyDeletesSoftDeletedLoginLogs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.Exec(`CREATE TABLE admin_login_logs (
		id integer primary key autoincrement,
		created_at datetime,
		updated_at datetime,
		deleted_at integer NOT NULL DEFAULT 0,
		token_expires datetime,
		refresh_expires datetime
	)`).Error; err != nil {
		t.Fatalf("create admin_login_logs failed: %v", err)
	}

	oldTime := time.Now().AddDate(0, 0, -60)
	if err := db.Exec(`INSERT INTO admin_login_logs (id, created_at, updated_at, deleted_at) VALUES
		(1, ?, ?, 0),
		(2, ?, ?, 123456)`, oldTime, oldTime, oldTime, oldTime).Error; err != nil {
		t.Fatalf("seed admin_login_logs failed: %v", err)
	}

	deleted, err := batchDeleteByCutoff(db, &model.AdminLoginLogs{}, "admin_login_logs", time.Now().AddDate(0, 0, -30), context.Background(), true)
	if err != nil {
		t.Fatalf("batch delete failed: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 rows physically deleted, got %d", deleted)
	}

	var count int64
	if err := db.Unscoped().Table("admin_login_logs").Count(&count).Error; err != nil {
		t.Fatalf("count admin_login_logs failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no remaining rows, got %d", count)
	}
}

func TestAuditCleanupRetainsSessionsUntilBothTokensExpire(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE admin_login_logs (id INTEGER PRIMARY KEY, created_at datetime, deleted_at integer DEFAULT 0, token_expires datetime, refresh_expires datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	old, future := time.Now().AddDate(0, 0, -60), time.Now().Add(time.Hour)
	for _, row := range []struct {
		id              int
		access, refresh time.Time
	}{{1, old, future}, {2, future, old}, {3, old, old}} {
		if err := db.Exec("INSERT INTO admin_login_logs (id,created_at,token_expires,refresh_expires) VALUES (?,?,?,?)", row.id, old, row.access, row.refresh).Error; err != nil {
			t.Fatal(err)
		}
	}
	deleted, err := batchDeleteByCutoff(db, &model.AdminLoginLogs{}, "admin_login_logs", time.Now().AddDate(0, 0, -30), context.Background(), true)
	if err != nil || deleted != 1 {
		t.Fatalf("deleted=%d error=%v", deleted, err)
	}
	var ids []int
	if err := db.Table("admin_login_logs").Order("id").Pluck("id", &ids).Error; err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Fatalf("active sessions removed: %v", ids)
	}
}
