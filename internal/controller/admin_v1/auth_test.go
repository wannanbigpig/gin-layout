package admin_v1

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/config/autoload"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	req "github.com/wannanbigpig/gin-layout/internal/pkg/request"
	authservice "github.com/wannanbigpig/gin-layout/internal/service/auth"
	"github.com/wannanbigpig/gin-layout/internal/validator"
)

// TestExtractAccessToken 验证请求头中的访问令牌提取逻辑。
func TestExtractAccessToken(t *testing.T) {
	initControllerAuthTest(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/v1/auth/check-token", nil)
	ctx.Request.Header.Set("Authorization", "Bearer token-value")

	accessToken, err := req.GetAccessToken(ctx)
	if err != nil {
		t.Fatalf("expected token to be extracted, got error %v", err)
	}
	if accessToken != "token-value" {
		t.Fatalf("unexpected token value %s", accessToken)
	}
}

// TestCheckTokenWithoutAuthorization 验证缺少 token 时返回错误响应。
func TestCheckTokenWithoutAuthorization(t *testing.T) {
	initControllerAuthTest(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/v1/auth/check-token", nil)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "test-request-id")

	NewLoginController().CheckToken(ctx)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected http status 500, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"code":500`) {
		t.Fatalf("expected server error response, got %s", recorder.Body.String())
	}
}

// TestLoginWithoutRequiredParams 验证登录参数校验失败路径。
func TestLoginWithoutRequiredParams(t *testing.T) {
	initControllerAuthTest(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/v1/login", nil)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "test-request-id")

	NewLoginController().Login(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected http status 400, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"code":10000`) {
		t.Fatalf("expected validation error response, got %s", recorder.Body.String())
	}
}

// TestBuildLoginLogInfo 验证登录日志上下文构造。
func TestBuildLoginLogInfo(t *testing.T) {
	initControllerAuthTest(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/v1/login", nil)
	ctx.Request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/122.0.0.0 Safari/537.36")
	ctx.Request.RemoteAddr = "192.0.2.1:1234"

	logInfo := authservice.NewLoginService().BuildLoginLogInfo(ctx)
	if logInfo.IP == "" {
		t.Fatal("expected client ip in login log info")
	}
	if logInfo.UserAgent == "" {
		t.Fatal("expected user agent in login log info")
	}
	if logInfo.OS == "" || logInfo.OS == "Unknown" {
		t.Fatalf("expected parsed os in login log info, got %q", logInfo.OS)
	}
	if logInfo.Browser != "Chrome" {
		t.Fatalf("expected Chrome browser in login log info, got %q", logInfo.Browser)
	}
}

func TestBuildLoginLogInfoFallbacksUnknownForMissingUserAgent(t *testing.T) {
	initControllerAuthTest(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/v1/login", nil)
	ctx.Request.RemoteAddr = "192.0.2.1:1234"

	logInfo := authservice.NewLoginService().BuildLoginLogInfo(ctx)
	if logInfo.OS != "Unknown" {
		t.Fatalf("expected unknown os fallback, got %q", logInfo.OS)
	}
	if logInfo.Browser != "Unknown" {
		t.Fatalf("expected unknown browser fallback, got %q", logInfo.Browser)
	}
}

func TestLogoutClearsCookieWhenAccessTokenInvalid(t *testing.T) {
	initControllerAuthTest(t)
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Jwt.RefreshCookieName = "refresh_token"
		cfg.Jwt.RefreshCookiePath = "/admin/v1/auth"
		cfg.Jwt.RefreshCookieSameSite = "Lax"
		cfg.Mysql = autoload.MysqlConfig{Enable: false}
	})
	defer restoreConfig()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/v1/auth/logout", nil)
	ctx.Request.Header.Set("Authorization", "Bearer invalid-token")
	ctx.Request.AddCookie(&http.Cookie{Name: "refresh_token", Value: "invalid-refresh"})
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "test-request-id")

	NewLoginController().Logout(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected http status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected clear refresh cookie")
	}
	clearCookie := cookies[0]
	if clearCookie.Name != "refresh_token" || clearCookie.MaxAge != -1 || clearCookie.Path != "/admin/v1/auth" || !clearCookie.HttpOnly {
		t.Fatalf("unexpected clear cookie: %#v", clearCookie)
	}
}

func initControllerAuthTest(t *testing.T) {
	t.Helper()

	cfg := config.CloneConf(config.GetConfig())
	cfg.BasePath = t.TempDir()
	cfg.AppEnv = "local"
	cfg.Jwt.SecretKey = "controller-test-jwt-key"
	cfg.Security.CredentialEncryptionKey = "controller-test-credential-key"
	cfg.Mysql = autoload.MysqlConfig{Enable: false}
	cfg.Redis = autoload.RedisConfig{Enable: false}
	cfg.Queue = autoload.QueueConfig{Enable: false}
	cfg.Logger.Output = "stderr"
	t.Cleanup(config.ReplaceConfigForTesting(cfg))
	if err := logger.InitLogger(); err != nil {
		t.Fatalf("init logger failed: %v", err)
	}
	if err := validator.InitValidatorTrans("zh"); err != nil {
		t.Fatalf("init validator failed: %v", err)
	}
}
