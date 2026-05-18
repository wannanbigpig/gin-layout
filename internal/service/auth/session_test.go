package auth

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

func TestRevokeSessionUpdatesDatabaseWhenRedisUnavailable(t *testing.T) {
	db := newSessionTestDB(t)
	expires := utils.FormatDate{Time: time.Now().Add(time.Hour)}
	loginLog := model.AdminLoginLogs{
		UID:          1,
		Username:     "admin",
		JwtID:        "jwt-id",
		LoginStatus:  model.LoginStatusSuccess,
		IsRevoked:    model.IsRevokedNo,
		TokenExpires: &expires,
	}
	if err := db.Create(&loginLog).Error; err != nil {
		t.Fatalf("create login log failed: %v", err)
	}

	service := NewLoginServiceWithDeps(LoginServiceDeps{
		LoginLogDB: db,
		WriteTokenToBlacklist: func(_ string, _ time.Duration) error {
			return errRedisUnavailable
		},
	})
	if err := service.RevokeSession(context.Background(), loginLog.ID, "force offline"); err != nil {
		t.Fatalf("RevokeSession returned error: %v", err)
	}

	var stored model.AdminLoginLogs
	if err := db.First(&stored, loginLog.ID).Error; err != nil {
		t.Fatalf("query login log failed: %v", err)
	}
	if stored.IsRevoked != model.IsRevokedYes {
		t.Fatalf("expected session to be revoked, got %d", stored.IsRevoked)
	}
	if stored.RevokedReason != "force offline" || stored.RevokedAt == nil {
		t.Fatalf("unexpected revoke fields: %#v", stored)
	}
}

func TestRevokeSessionRejectsExpiredSession(t *testing.T) {
	db := newSessionTestDB(t)
	expires := utils.FormatDate{Time: time.Now().Add(-time.Minute)}
	loginLog := model.AdminLoginLogs{
		UID:          1,
		Username:     "admin",
		JwtID:        "jwt-id",
		LoginStatus:  model.LoginStatusSuccess,
		IsRevoked:    model.IsRevokedNo,
		TokenExpires: &expires,
	}
	if err := db.Create(&loginLog).Error; err != nil {
		t.Fatalf("create login log failed: %v", err)
	}

	service := NewLoginServiceWithDeps(LoginServiceDeps{LoginLogDB: db})
	if err := service.RevokeSession(context.Background(), loginLog.ID, "force offline"); err == nil {
		t.Fatal("expected expired session revoke to fail")
	}
}

func newSessionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	statement := `CREATE TABLE admin_login_logs (
		id integer primary key autoincrement,
		created_at datetime,
		updated_at datetime,
		deleted_at integer not null default 0,
		uid integer,
		username text,
		jwt_id text,
		access_token text,
		refresh_token text,
		token_hash text,
		refresh_token_hash text,
		ip text,
		user_agent text,
		os text,
		browser text,
		execution_time integer,
		login_status integer,
		login_fail_reason text,
		type integer,
		is_revoked integer,
		revoked_code integer,
		revoked_reason text,
		revoked_at datetime,
		token_expires datetime,
		refresh_expires datetime
	)`
	if err := db.Exec(statement).Error; err != nil {
		t.Fatalf("create login logs table failed: %v", err)
	}
	return db
}
