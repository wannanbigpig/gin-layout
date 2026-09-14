package auth

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/config/autoload"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/testkit"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

var refreshTokenTestDBSequence atomic.Uint64

func TestRefreshByTokenRotatesRefreshToken(t *testing.T) {
	secretKey := testkit.SecretKey("auth-refresh-rotate")
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Jwt.SecretKey = secretKey
		cfg.Jwt.TTL = 1800
		cfg.Jwt.RefreshTokenTTL = 604800
	})
	defer restoreConfig()

	db := newRefreshTokenTestDB(t)
	insertRefreshTestUser(t, db, model.AdminUserStatusEnabled)
	service := NewLoginServiceWithDeps(LoginServiceDeps{
		ConfigProvider: func() *config.Conf {
			return &config.Conf{Jwt: autoload.JwtConfig{SecretKey: secretKey, TTL: 1800, RefreshTokenTTL: 604800}}
		},
		LoginLogDB: db,
	})

	user := &model.AdminUser{
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 1}},
		Username:                "admin",
		Status:                  model.AdminUserStatusEnabled,
		IsSuperAdmin:            global.Yes,
	}
	claims := service.newAdminCustomClaims(user)
	accessToken, err := service.generateAccessToken(claims)
	if err != nil {
		t.Fatalf("generate access token failed: %v", err)
	}
	loginLog, refreshToken, err := service.issueRefreshToken(user, claims, accessToken, LoginLogInfo{IP: "127.0.0.1"}, model.LoginTypeLogin)
	if err != nil {
		t.Fatalf("issue refresh token failed: %v", err)
	}
	loginLog.SetDB(db)
	if err := loginLog.Create(); err != nil {
		t.Fatalf("create login log failed: %v", err)
	}

	result, newRefreshToken, err := service.RefreshByToken(refreshToken, LoginLogInfo{IP: "127.0.0.2"})
	if err != nil {
		t.Fatalf("RefreshByToken returned error: %v", err)
	}
	if result == nil || result.AccessToken == "" || result.TokenType != tokenTypeBearer || result.ExpiresAt == 0 {
		t.Fatalf("unexpected token response: %#v", result)
	}
	if newRefreshToken == "" || newRefreshToken == refreshToken {
		t.Fatalf("expected rotated refresh token, got %q", newRefreshToken)
	}

	var oldLog model.AdminLoginLogs
	if err := db.First(&oldLog, loginLog.ID).Error; err != nil {
		t.Fatalf("query old login log failed: %v", err)
	}
	if oldLog.IsRevoked != model.IsRevokedYes || oldLog.RevokedCode != model.RevokedCodeTokenRefresh {
		t.Fatalf("expected old refresh token revoked, got %#v", oldLog)
	}

	var newLog model.AdminLoginLogs
	if err := db.Where("refresh_token_hash = ?", service.calculateTokenHash(newRefreshToken)).First(&newLog).Error; err != nil {
		t.Fatalf("query new login log failed: %v", err)
	}
	if newLog.IsRevoked != model.IsRevokedNo || newLog.RefreshExpires == nil || newLog.Type != model.LoginTypeRefresh {
		t.Fatalf("unexpected new refresh log: %#v", newLog)
	}

	if _, _, err := service.RefreshByToken(refreshToken, LoginLogInfo{}); err == nil {
		t.Fatal("expected old refresh token to be rejected after rotation")
	}
}

func TestRefreshByTokenRejectsDisabledUser(t *testing.T) {
	secretKey := testkit.SecretKey("auth-refresh-disabled")
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Jwt.SecretKey = secretKey
		cfg.Jwt.TTL = 1800
		cfg.Jwt.RefreshTokenTTL = 604800
	})
	defer restoreConfig()

	db := newRefreshTokenTestDB(t)
	insertRefreshTestUser(t, db, model.AdminUserStatusDisabled)
	service := NewLoginServiceWithDeps(LoginServiceDeps{
		ConfigProvider: func() *config.Conf {
			return &config.Conf{Jwt: autoload.JwtConfig{SecretKey: secretKey, TTL: 1800, RefreshTokenTTL: 604800}}
		},
		LoginLogDB: db,
	})

	user := &model.AdminUser{
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 1}},
		Username:                "admin",
		Status:                  model.AdminUserStatusEnabled,
	}
	claims := service.newAdminCustomClaims(user)
	accessToken, err := service.generateAccessToken(claims)
	if err != nil {
		t.Fatalf("generate access token failed: %v", err)
	}
	loginLog, refreshToken, err := service.issueRefreshToken(user, claims, accessToken, LoginLogInfo{}, model.LoginTypeLogin)
	if err != nil {
		t.Fatalf("issue refresh token failed: %v", err)
	}
	loginLog.SetDB(db)
	if err := loginLog.Create(); err != nil {
		t.Fatalf("create login log failed: %v", err)
	}

	_, _, err = service.RefreshByToken(refreshToken, LoginLogInfo{})
	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) || businessErr.GetCode() != e.RefreshTokenInvalid {
		t.Fatalf("expected RefreshTokenInvalid, got %v", err)
	}
}

func TestRefreshCookieUsesPersistentHttpOnlySettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := NewLoginServiceWithDeps(LoginServiceDeps{
		ConfigProvider: func() *config.Conf {
			return &config.Conf{
				Jwt: autoload.JwtConfig{
					RefreshTokenTTL:       604800,
					RefreshCookieName:     "refresh_token",
					RefreshCookiePath:     "/admin/v1/auth",
					RefreshCookieSameSite: "Lax",
					RefreshCookieSecure:   false,
					RefreshCookieDomain:   "",
				},
			}
		},
	})
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	service.SetRefreshCookie(ctx, "refresh-value")
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != "refresh_token" || cookie.Value != "refresh-value" {
		t.Fatalf("unexpected cookie name/value: %#v", cookie)
	}
	if !cookie.HttpOnly || cookie.MaxAge != 604800 || cookie.Path != "/admin/v1/auth" || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected refresh cookie attributes: %#v", cookie)
	}

	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	service.ClearRefreshCookie(ctx)
	clearCookie := recorder.Result().Cookies()[0]
	if clearCookie.Name != "refresh_token" || clearCookie.MaxAge != -1 || clearCookie.Path != "/admin/v1/auth" || !clearCookie.HttpOnly {
		t.Fatalf("unexpected clear cookie attributes: %#v", clearCookie)
	}
}

func TestRefreshByTokenOnlyOneUseSucceedsWithSQLite(t *testing.T) {
	secretKey := testkit.SecretKey("auth-refresh-concurrent")
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Jwt.SecretKey = secretKey
		cfg.Jwt.TTL = 1800
		cfg.Jwt.RefreshTokenTTL = 604800
	})
	defer restoreConfig()

	db := newRefreshTokenTestDB(t)
	insertRefreshTestUser(t, db, model.AdminUserStatusEnabled)
	service := NewLoginServiceWithDeps(LoginServiceDeps{
		ConfigProvider: func() *config.Conf {
			return &config.Conf{Jwt: autoload.JwtConfig{SecretKey: secretKey, TTL: 1800, RefreshTokenTTL: 604800}}
		},
		LoginLogDB: db,
	})
	user := &model.AdminUser{
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 1}},
		Username:                "admin",
		Status:                  model.AdminUserStatusEnabled,
		IsSuperAdmin:            global.Yes,
	}
	claims := service.newAdminCustomClaims(user)
	accessToken, err := service.generateAccessToken(claims)
	if err != nil {
		t.Fatalf("generate access token failed: %v", err)
	}
	loginLog, refreshToken, err := service.issueRefreshToken(user, claims, accessToken, LoginLogInfo{}, model.LoginTypeLogin)
	if err != nil {
		t.Fatalf("issue refresh token failed: %v", err)
	}
	loginLog.SetDB(db)
	if err := loginLog.Create(); err != nil {
		t.Fatalf("create login log failed: %v", err)
	}

	if _, _, err := service.RefreshByToken(refreshToken, LoginLogInfo{}); err != nil {
		t.Fatalf("first RefreshByToken returned error: %v", err)
	}
	for attempt := 2; attempt <= 8; attempt++ {
		if _, _, err := service.RefreshByToken(refreshToken, LoginLogInfo{}); err == nil {
			t.Fatalf("expected refresh attempt %d to reject the consumed token", attempt)
		}
	}
}

func newRefreshTokenTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:refresh-token-%d?mode=memory&cache=shared&_busy_timeout=5000", refreshTokenTestDBSequence.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite connection failed: %v", err)
	}
	// SQLite 不具备 MySQL 的行级 FOR UPDATE 语义；单连接用于验证应用层令牌只能消费一次，
	// 但不能替代 MySQL 集成测试对并发锁行为的验证。
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close sqlite failed: %v", err)
		}
	})
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

func insertRefreshTestUser(t *testing.T, db *gorm.DB, status uint8) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE admin_user (
		id integer primary key,
		created_at datetime,
		updated_at datetime,
		deleted_at integer not null default 0,
		username text,
		password text,
		status integer,
		is_super_admin integer,
		last_ip text,
		last_login datetime
	)`).Error; err != nil {
		t.Fatalf("create admin_user failed: %v", err)
	}
	if err := db.Exec(`INSERT INTO admin_user(id, deleted_at, username, status, is_super_admin) VALUES (?, ?, ?, ?, ?)`,
		1, 0, "admin", status, global.Yes).Error; err != nil {
		t.Fatalf("insert admin_user failed: %v", err)
	}
}

func TestRevokeRefreshTokenRevokesActiveToken(t *testing.T) {
	db := newRefreshTokenTestDB(t)
	service := NewLoginServiceWithDeps(LoginServiceDeps{LoginLogDB: db})
	refreshToken := "refresh-token"
	refreshExpires := utils.FormatDate{Time: time.Now().Add(time.Hour)}
	loginLog := model.AdminLoginLogs{
		UID:              1,
		Username:         "admin",
		RefreshTokenHash: service.calculateTokenHash(refreshToken),
		LoginStatus:      model.LoginStatusSuccess,
		IsRevoked:        model.IsRevokedNo,
		RefreshExpires:   &refreshExpires,
	}
	if err := db.Create(&loginLog).Error; err != nil {
		t.Fatalf("create login log failed: %v", err)
	}

	if err := service.RevokeRefreshToken(refreshToken, model.RevokedCodeUserLogout, "logout"); err != nil {
		t.Fatalf("RevokeRefreshToken returned error: %v", err)
	}

	var stored model.AdminLoginLogs
	if err := db.First(&stored, loginLog.ID).Error; err != nil {
		t.Fatalf("query login log failed: %v", err)
	}
	if stored.IsRevoked != model.IsRevokedYes || stored.RevokedCode != model.RevokedCodeUserLogout {
		t.Fatalf("expected refresh token revoked, got %#v", stored)
	}
}

func TestRevokeUserTokensRevokesRefreshOnlySession(t *testing.T) {
	db := newRefreshTokenTestDB(t)
	service := NewLoginServiceWithDeps(LoginServiceDeps{
		LoginLogDB: db,
		WriteTokenToBlacklist: func(_ string, _ time.Duration) error {
			return nil
		},
	})
	tokenExpires := utils.FormatDate{Time: time.Now().UTC().Add(-time.Minute)}
	refreshExpires := utils.FormatDate{Time: time.Now().UTC().Add(time.Hour)}
	loginLog := model.AdminLoginLogs{
		UID:              1,
		Username:         "admin",
		JwtID:            "expired-access-jwt",
		LoginStatus:      model.LoginStatusSuccess,
		IsRevoked:        model.IsRevokedNo,
		TokenExpires:     &tokenExpires,
		RefreshExpires:   &refreshExpires,
		RefreshTokenHash: "refresh-hash",
	}
	if err := db.Create(&loginLog).Error; err != nil {
		t.Fatalf("create login log failed: %v", err)
	}

	if err := service.RevokeUserTokens(1, model.RevokedCodeUserDisable, "disable"); err != nil {
		t.Fatalf("RevokeUserTokens returned error: %v", err)
	}

	var stored model.AdminLoginLogs
	if err := db.First(&stored, loginLog.ID).Error; err != nil {
		t.Fatalf("query login log failed: %v", err)
	}
	if stored.IsRevoked != model.IsRevokedYes || stored.RevokedCode != model.RevokedCodeUserDisable {
		t.Fatalf("expected refresh-only session revoked, got %#v", stored)
	}
}

func TestRevokeUserTokensRevokesSessionWithoutJwtID(t *testing.T) {
	db := newRefreshTokenTestDB(t)
	service := NewLoginServiceWithDeps(LoginServiceDeps{
		LoginLogDB: db,
		WriteTokenToBlacklist: func(_ string, _ time.Duration) error {
			return nil
		},
	})
	refreshExpires := utils.FormatDate{Time: time.Now().UTC().Add(time.Hour)}
	loginLog := model.AdminLoginLogs{
		UID:              1,
		Username:         "admin",
		LoginStatus:      model.LoginStatusSuccess,
		IsRevoked:        model.IsRevokedNo,
		RefreshExpires:   &refreshExpires,
		RefreshTokenHash: "refresh-hash-without-jwt",
	}
	if err := db.Create(&loginLog).Error; err != nil {
		t.Fatalf("create login log failed: %v", err)
	}

	if err := service.RevokeUserTokens(1, model.RevokedCodeUserDisable, "disable"); err != nil {
		t.Fatalf("RevokeUserTokens returned error: %v", err)
	}

	var stored model.AdminLoginLogs
	if err := db.First(&stored, loginLog.ID).Error; err != nil {
		t.Fatalf("query login log failed: %v", err)
	}
	if stored.IsRevoked != model.IsRevokedYes || stored.RevokedCode != model.RevokedCodeUserDisable {
		t.Fatalf("expected session without jwt_id revoked by row id, got %#v", stored)
	}
}
