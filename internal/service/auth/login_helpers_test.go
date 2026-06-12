package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/config/autoload"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/testkit"
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

func TestGenerateAccessTokenOmitsPIIClaims(t *testing.T) {
	secretKey := testkit.SecretKey("auth-minimal-jwt")
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Jwt.SecretKey = secretKey
		cfg.Jwt.TTL = 3600
	})
	defer restoreConfig()

	service := NewLoginService()
	claims := service.newAdminCustomClaims(&model.AdminUser{
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{
			BaseModel: model.BaseModel{ID: 12},
		},
		Username:        "tester",
		Nickname:        "测试用户",
		Email:           "tester@example.com",
		FullPhoneNumber: "+8613800138000",
		PhoneNumber:     "13800138000",
		CountryCode:     "+86",
		IsSuperAdmin:    global.Yes,
	})

	accessToken, err := service.generateAccessToken(claims)
	if err != nil {
		t.Fatalf("generate token failed: %v", err)
	}

	parsedClaims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(accessToken, parsedClaims, func(jwtToken *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})
	if err != nil {
		t.Fatalf("parse generated token failed: %v", err)
	}
	if parsed == nil || !parsed.Valid {
		t.Fatalf("expected generated token to be valid")
	}

	for _, key := range []string{"full_phone_number", "phone_number", "country_code", "email", "nickname"} {
		if _, exists := parsedClaims[key]; exists {
			t.Fatalf("expected JWT payload to omit %q, got %#v", key, parsedClaims)
		}
	}
	if parsedClaims["user_id"] == nil || parsedClaims["username"] != "tester" || parsedClaims["is_super_admin"] == nil {
		t.Fatalf("expected minimal identity and permission claims, got %#v", parsedClaims)
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

// TestIsPrincipalValidSkipsFallbackWhenMysqlUnavailable 验证降级模式下不会继续回表。
func TestIsPrincipalValidSkipsFallbackWhenMysqlUnavailable(t *testing.T) {
	service := NewLoginService()
	claims := &token.AdminCustomClaims{
		AdminUserInfo: token.AdminUserInfo{UserID: 12},
		RegisteredClaims: jwt.RegisteredClaims{
			ID: "jwt-id",
		},
	}

	tokenRevokedCalled := false
	service.blacklistLookupFn = func(_ string) (bool, error) {
		return false, errRedisUnavailable
	}
	service.tokenRevokedLookupFn = func(_ string) bool {
		tokenRevokedCalled = true
		return false
	}
	service.mysqlReadyFn = func() bool { return false }

	if service.isPrincipalValid(claims) {
		t.Fatal("expected principal to be rejected when redis and mysql are unavailable")
	}
	if tokenRevokedCalled {
		t.Fatal("expected database revoke lookup to be skipped")
	}
}

func TestIsPrincipalValidFallsBackToDatabaseWhenMysqlReady(t *testing.T) {
	service := NewLoginService()
	claims := &token.AdminCustomClaims{
		AdminUserInfo: token.AdminUserInfo{UserID: 12},
		RegisteredClaims: jwt.RegisteredClaims{
			ID: "jwt-id",
		},
	}

	tokenRevokedCalled := false
	service.blacklistLookupFn = func(_ string) (bool, error) {
		return false, errRedisUnavailable
	}
	service.tokenRevokedLookupFn = func(jwtID string) bool {
		tokenRevokedCalled = true
		return jwtID == "revoked"
	}
	service.mysqlReadyFn = func() bool { return true }

	if !service.isPrincipalValid(claims) {
		t.Fatal("expected principal to stay valid when mysql fallback is available")
	}
	if !tokenRevokedCalled {
		t.Fatal("expected database revoke lookup to be used")
	}
}

func TestIsPrincipalValidChecksDatabaseWhenRedisMisses(t *testing.T) {
	service := NewLoginService()
	claims := &token.AdminCustomClaims{
		AdminUserInfo: token.AdminUserInfo{UserID: 12},
		RegisteredClaims: jwt.RegisteredClaims{
			ID: "jwt-id",
		},
	}

	tokenRevokedCalled := false
	service.blacklistLookupFn = func(_ string) (bool, error) {
		return false, nil
	}
	service.tokenRevokedLookupFn = func(jwtID string) bool {
		tokenRevokedCalled = true
		return jwtID == "jwt-id"
	}
	service.mysqlReadyFn = func() bool { return true }

	if service.isPrincipalValid(claims) {
		t.Fatal("expected redis miss to fall back to revoked token lookup")
	}
	if !tokenRevokedCalled {
		t.Fatal("expected database revoke lookup to be used on redis miss")
	}
}

func TestIsPrincipalValidRejectsRevokedToken(t *testing.T) {
	service := NewLoginService()
	claims := &token.AdminCustomClaims{
		AdminUserInfo: token.AdminUserInfo{UserID: 12},
		RegisteredClaims: jwt.RegisteredClaims{
			ID: "jwt-id",
		},
	}

	service.blacklistLookupFn = func(_ string) (bool, error) {
		return false, errors.New("redis unavailable")
	}
	service.tokenRevokedLookupFn = func(jwtID string) bool {
		return jwtID == "jwt-id"
	}

	if service.isPrincipalValid(claims) {
		t.Fatal("expected revoked token to be rejected")
	}
}

func TestValidateUserReturnsDependencyErrorWhenDBUnavailable(t *testing.T) {
	service := NewLoginService()

	user, err := service.validateUser("missing-user", "password")
	if user != nil {
		t.Fatalf("expected nil user, got %#v", user)
	}

	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) {
		t.Fatalf("expected business error, got %v", err)
	}
	if businessErr.GetCode() != e.ServiceDependencyNotReady {
		t.Fatalf("expected code %d, got %d", e.ServiceDependencyNotReady, businessErr.GetCode())
	}
}

func TestLogoutFallsBackToDatabaseRevocationWhenRedisUnavailable(t *testing.T) {
	secretKey := testkit.SecretKey("auth-logout")
	service := NewLoginServiceWithDeps(LoginServiceDeps{
		ConfigProvider: func() *config.Conf {
			return &config.Conf{
				Jwt: autoload.JwtConfig{
					SecretKey: secretKey,
				},
			}
		},
	})

	revoked := false
	service.markTokensRevokedFn = func(_ context.Context, jwtIDs []string, revokedCode uint8, revokedReason string) error {
		revoked = true
		if len(jwtIDs) != 1 || jwtIDs[0] == "" {
			t.Fatalf("unexpected jwt ids: %#v", jwtIDs)
		}
		if revokedCode != model.RevokedCodeUserLogout || revokedReason == "" {
			t.Fatalf("unexpected revoke payload: code=%d reason=%s", revokedCode, revokedReason)
		}
		return nil
	}

	claims := &token.AdminCustomClaims{
		AdminUserInfo: token.AdminUserInfo{UserID: 1, Username: "tester"},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
			Issuer:    global.Issuer,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   global.PcAdminSubject,
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        "jwt-logout-test",
		},
	}
	accessToken, err := signTokenForTest(claims, secretKey)
	if err != nil {
		t.Fatalf("generate token failed: %v", err)
	}

	if err := service.Logout(accessToken); err != nil {
		t.Fatalf("expected logout to succeed without redis, got %v", err)
	}
	if !revoked {
		t.Fatal("expected database revocation to be invoked")
	}
}

func TestLogoutTreatsRedisWriteFailureAsSuccessAfterDatabaseRevocation(t *testing.T) {
	secretKey := testkit.SecretKey("auth-logout")
	service := NewLoginServiceWithDeps(LoginServiceDeps{
		ConfigProvider: func() *config.Conf {
			return &config.Conf{
				Jwt: autoload.JwtConfig{
					SecretKey: secretKey,
				},
			}
		},
	})

	revoked := false
	blacklistWriteCalled := false
	service.markTokensRevokedFn = func(_ context.Context, jwtIDs []string, revokedCode uint8, revokedReason string) error {
		revoked = true
		if len(jwtIDs) != 1 || jwtIDs[0] == "" {
			t.Fatalf("unexpected jwt ids: %#v", jwtIDs)
		}
		if revokedCode != model.RevokedCodeUserLogout || revokedReason == "" {
			t.Fatalf("unexpected revoke payload: code=%d reason=%s", revokedCode, revokedReason)
		}
		return nil
	}
	service.writeTokenToBlacklistFn = func(jwtID string, remainingTime time.Duration) error {
		blacklistWriteCalled = true
		if jwtID == "" || remainingTime <= 0 {
			t.Fatalf("unexpected blacklist write args: jwtID=%q remaining=%v", jwtID, remainingTime)
		}
		return errors.New("redis write timeout")
	}

	claims := &token.AdminCustomClaims{
		AdminUserInfo: token.AdminUserInfo{UserID: 1, Username: "tester"},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
			Issuer:    global.Issuer,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   global.PcAdminSubject,
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        "jwt-logout-blacklist-fail",
		},
	}
	accessToken, err := signTokenForTest(claims, secretKey)
	if err != nil {
		t.Fatalf("generate token failed: %v", err)
	}

	if err := service.Logout(accessToken); err != nil {
		t.Fatalf("expected logout to degrade to success, got %v", err)
	}
	if !revoked {
		t.Fatal("expected database revocation to be invoked")
	}
	if !blacklistWriteCalled {
		t.Fatal("expected redis blacklist write to be attempted")
	}
}

func signTokenForTest(claims jwt.Claims, secret string) (string, error) {
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return jwtToken.SignedString([]byte(secret))
}
