package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
)

func TestRecordLoginLogLoginCommitsUserAndLogTogether(t *testing.T) {
	db := newLoginRecordTestDB(t, true)
	if err := db.Exec(`INSERT INTO admin_user(id, deleted_at, username, last_ip, last_login) VALUES (?, ?, ?, ?, ?)`,
		1, 0, "admin", "", nil).Error; err != nil {
		t.Fatalf("insert admin_user failed: %v", err)
	}

	service := NewLoginServiceWithDeps(LoginServiceDeps{LoginLogDB: db})
	adminUser := &model.AdminUser{ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 1}}, Username: "admin"}
	claims := token.AdminCustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "jwt-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	if err := service.recordLoginLog(adminUser, claims, "access-token", "", LoginLogInfo{IP: "127.0.0.1"}, model.LoginTypeLogin); err != nil {
		t.Fatalf("recordLoginLog returned error: %v", err)
	}

	var logCount int64
	if err := db.Table("admin_login_logs").Count(&logCount).Error; err != nil {
		t.Fatalf("count admin_login_logs failed: %v", err)
	}
	if logCount != 1 {
		t.Fatalf("expected one login log, got %d", logCount)
	}

	var stored struct {
		LastIP string
	}
	if err := db.Table("admin_user").Select("last_ip").Where("id = ?", 1).Take(&stored).Error; err != nil {
		t.Fatalf("query admin_user failed: %v", err)
	}
	if stored.LastIP != "127.0.0.1" {
		t.Fatalf("expected last_ip updated, got %q", stored.LastIP)
	}
}

func TestRecordLoginLogLoginRollsBackWhenUserUpdateFails(t *testing.T) {
	db := newLoginRecordTestDB(t, false)
	if err := db.Exec(`INSERT INTO admin_user(id, deleted_at, username, last_ip) VALUES (?, ?, ?, ?)`,
		1, 0, "admin", "").Error; err != nil {
		t.Fatalf("insert admin_user failed: %v", err)
	}

	service := NewLoginServiceWithDeps(LoginServiceDeps{LoginLogDB: db})
	adminUser := &model.AdminUser{ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 1}}, Username: "admin"}
	claims := token.AdminCustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "jwt-rollback",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	if err := service.recordLoginLog(adminUser, claims, "access-token", "", LoginLogInfo{IP: "127.0.0.1"}, model.LoginTypeLogin); err == nil {
		t.Fatal("expected recordLoginLog to fail when admin_user update fails")
	}

	var logCount int64
	if err := db.Table("admin_login_logs").Count(&logCount).Error; err != nil {
		t.Fatalf("count admin_login_logs failed: %v", err)
	}
	if logCount != 0 {
		t.Fatalf("expected login log insert rolled back, got %d", logCount)
	}
}

func newLoginRecordTestDB(t *testing.T, withLastLoginColumn bool) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	adminUserDDL := `CREATE TABLE admin_user (
		id integer primary key,
		deleted_at integer not null default 0,
		username text,
		last_ip text,
		updated_at datetime`
	if withLastLoginColumn {
		adminUserDDL += `,
		last_login datetime`
	}
	adminUserDDL += `)`
	if err := db.Exec(adminUserDDL).Error; err != nil {
		t.Fatalf("create admin_user failed: %v", err)
	}

	loginLogDDL := `CREATE TABLE admin_login_logs (
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
	if err := db.Exec(loginLogDDL).Error; err != nil {
		t.Fatalf("create admin_login_logs failed: %v", err)
	}
	return db
}
