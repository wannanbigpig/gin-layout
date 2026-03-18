package auth

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
)

// TestExtractErrorMessage 验证错误消息提取逻辑。
func TestExtractErrorMessage(t *testing.T) {
	service := NewLoginService()

	businessErr := e.NewBusinessError(e.CaptchaErr)
	if got := service.extractErrorMessage(businessErr); got != businessErr.GetMessage() {
		t.Fatalf("expected business message %q, got %q", businessErr.GetMessage(), got)
	}

	plainErr := errors.New("plain error")
	if got := service.extractErrorMessage(plainErr); got != plainErr.Error() {
		t.Fatalf("expected plain error message, got %q", got)
	}
}

// TestCalculateTokenHash 验证 token 哈希值计算结果稳定。
func TestCalculateTokenHash(t *testing.T) {
	service := NewLoginService()
	const tokenValue = "token-value"
	const expected = "e6c02a5742ea9d4de588eb9b9de7bed43dc17011552186bed3e98b2c5958ff4a"

	if got := service.calculateTokenHash(tokenValue); got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

// TestGetBlacklistKey 验证黑名单 key 前缀拼接。
func TestGetBlacklistKey(t *testing.T) {
	service := NewLoginService()

	if got := service.getBlacklistKey("jwt-id"); got != "blacklist:jwt-id" {
		t.Fatalf("unexpected blacklist key: %s", got)
	}
}

// TestCalculateRemainingTime 验证剩余时间计算逻辑。
func TestCalculateRemainingTime(t *testing.T) {
	service := NewLoginService()
	now := time.Now()
	expires := &utils.FormatDate{Time: now.Add(2 * time.Minute)}

	if got := service.calculateRemainingTime(expires, now); got < time.Minute || got > 2*time.Minute {
		t.Fatalf("unexpected remaining time: %v", got)
	}

	expired := &utils.FormatDate{Time: now.Add(-time.Minute)}
	if got := service.calculateRemainingTime(expired, now); got != 0 {
		t.Fatalf("expected 0 for expired token, got %v", got)
	}

	if got := service.calculateRemainingTime(nil, now); got != defaultTokenTTL {
		t.Fatalf("expected default ttl %v, got %v", defaultTokenTTL, got)
	}
}

// TestBuildRefreshLockKey 验证刷新锁 key 拼接。
func TestBuildRefreshLockKey(t *testing.T) {
	service := NewLoginService()
	claims := &token.AdminCustomClaims{AdminUserInfo: token.AdminUserInfo{UserID: 12}, RegisteredClaims: jwt.RegisteredClaims{ID: "jwt-id"}}

	if got := service.buildRefreshLockKey(claims); got != "refresh_token_lock:12:jwt-id" {
		t.Fatalf("unexpected refresh lock key: %s", got)
	}
}

// TestShouldRefreshToken 验证刷新条件判断。
func TestShouldRefreshToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := NewLoginService()
	originalRefreshTTL := config.Config.Jwt.RefreshTTL
	defer func() {
		config.Config.Jwt.RefreshTTL = originalRefreshTTL
	}()

	config.Config.Jwt.RefreshTTL = 30
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	service.SetCtx(ctx)
	claims := &token.AdminCustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Second)),
		},
	}
	if !service.shouldRefreshToken(claims) {
		t.Fatal("expected token to require refresh")
	}

	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(2 * time.Minute))
	if service.shouldRefreshToken(claims) {
		t.Fatal("expected token with long remaining ttl to skip refresh")
	}
}

// TestIsUserValid 验证用户状态优先检查。
func TestIsUserValid(t *testing.T) {
	service := NewLoginService()
	user := &model.AdminUser{Status: model.AdminUserStatusDisabled}

	if service.isUserValid(user, "jwt-id") {
		t.Fatal("expected disabled user to be invalid")
	}
}

func TestValidateUserReturnsBusinessErrorWhenUserNotFound(t *testing.T) {
	service := NewLoginService()

	user, err := service.validateUser("missing-user", "password")
	if user != nil {
		t.Fatalf("expected nil user, got %#v", user)
	}

	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) {
		t.Fatalf("expected business error, got %v", err)
	}
	if businessErr.GetCode() != e.UserDoesNotExist {
		t.Fatalf("expected code %d, got %d", e.UserDoesNotExist, businessErr.GetCode())
	}
}

// TestAcquireMemoryLock 验证内存锁可以正常获取并释放。
func TestAcquireMemoryLock(t *testing.T) {
	service := NewLoginService()
	unlock := service.acquireMemoryLock("lock-key")
	if unlock == nil {
		t.Fatal("expected unlock function")
	}
	unlock()
}
