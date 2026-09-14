package admin_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/golang-jwt/jwt/v5"

	"github.com/stretchr/testify/assert"

	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
)

// parseTestToken 解析测试用 token 并返回 claims。
func parseTestToken(accessToken string) (*token.AdminCustomClaims, error) {
	claims := new(token.AdminCustomClaims)
	secret := []byte(c.GetConfig().Jwt.SecretKey)
	parsedToken, err := jwt.ParseWithClaims(accessToken, claims, func(t *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !parsedToken.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func TestLoginCaptcha(t *testing.T) {
	captchaResp := anonymousGetRequest("/admin/v1/login-captcha", nil)
	assert.Equal(t, http.StatusOK, captchaResp.Code)
	captchaResult := decodeResult(t, captchaResp)
	assert.Equal(t, e.SUCCESS, captchaResult.Code)
}

func TestLoginInvalidCaptcha(t *testing.T) {
	loginData := map[string]any{
		"username":   "super_admin",
		"password":   "123456",
		"captcha":    "wrong",
		"captcha_id": "invalid",
	}
	body, err := json.Marshal(loginData)
	assert.Nil(t, err)
	bodyStr := string(body)
	resp := anonymousPostRequest("/admin/v1/login", &bodyStr)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.CaptchaErr, result.Code)
}

func TestCheckTokenWithAuthorization(t *testing.T) {
	requireMySQL(t)

	resp := getRequest("/admin/v1/auth/check-token", nil)

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.SUCCESS, result.Code)
	data, ok := result.Data.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, true, data["result"])
}

func TestLogoutIsIdempotentWithoutLogin(t *testing.T) {
	body := `{}`
	resp := performRequestWithHeaders(http.MethodPost, "/admin/v1/auth/logout", &body, "", map[string]string{
		"X-Requested-With": "XMLHttpRequest",
	})

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.SUCCESS, result.Code)
	if cookies := resp.Result().Cookies(); assert.NotEmpty(t, cookies) {
		assert.Equal(t, "refresh_token", cookies[0].Name)
		assert.Equal(t, -1, cookies[0].MaxAge)
		assert.True(t, cookies[0].HttpOnly)
	}
}

func TestAuthCookieEndpointsRequireCSRFHeader(t *testing.T) {
	body := `{}`
	for _, route := range []string{"/admin/v1/auth/logout", "/admin/v1/auth/refresh"} {
		t.Run(route, func(t *testing.T) {
			resp := anonymousPostRequest(route, &body)
			assert.Equal(t, http.StatusForbidden, resp.Code)
			result := decodeResult(t, resp)
			assert.Equal(t, e.AuthorizationErr, result.Code)
		})
	}
}

func TestConcurrentRefreshTokenUsesMySQLRowLock(t *testing.T) {
	requireWritableDB(t)

	db, err := model.GetDB()
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}
	if db.Dialector.Name() != "mysql" {
		t.Fatalf("接口测试要求 MySQL 驱动，当前数据库驱动为 %s", db.Dialector.Name())
	}

	user := findAdminUserByUsername(t, "super_admin")
	marker := uniqueTestName("refresh-concurrency")
	defer func() {
		if err := db.Unscoped().Where("user_agent = ?", marker).Delete(&model.AdminLoginLogs{}).Error; err != nil {
			t.Errorf("清理并发刷新登录日志失败: %v", err)
		}
	}()

	now := time.Now().UTC()
	refreshExpires := now.Add(time.Hour)
	refreshClaims := auth.RefreshClaims{
		UserID:   user.ID,
		TokenID:  uuid.New().String(),
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshExpires),
			Issuer:    global.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   global.PcAdminRefreshSubject,
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
	}
	refreshToken, err := token.Generate(refreshClaims)
	if err != nil {
		t.Fatalf("生成 refresh token 失败: %v", err)
	}
	refreshHash := sha256.Sum256([]byte(refreshToken))
	originalLog := &model.AdminLoginLogs{
		UID:              user.ID,
		Username:         user.Username,
		JwtID:            uuid.New().String(),
		RefreshTokenHash: hex.EncodeToString(refreshHash[:]),
		UserAgent:        marker,
		LoginStatus:      model.LoginStatusSuccess,
		Type:             model.LoginTypeLogin,
		IsRevoked:        model.IsRevokedNo,
		RefreshExpires:   &utils.FormatDate{Time: refreshExpires},
	}
	if err := db.Create(originalLog).Error; err != nil {
		t.Fatalf("创建待刷新登录日志失败: %v", err)
	}

	cookieName := strings.TrimSpace(c.GetConfig().Jwt.RefreshCookieName)
	if cookieName == "" {
		cookieName = "refresh_token"
	}
	cookieHeader := (&http.Cookie{Name: cookieName, Value: refreshToken}).String()

	const concurrency = 8
	start := make(chan struct{})
	responses := make(chan *httptest.ResponseRecorder, concurrency)
	var wg sync.WaitGroup
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			body := `{}`
			responses <- performRequestWithHeaders(http.MethodPost, "/admin/v1/auth/refresh", &body, "", map[string]string{
				"Cookie":           cookieHeader,
				"User-Agent":       marker,
				"X-Requested-With": "XMLHttpRequest",
			})
		}()
	}
	close(start)
	wg.Wait()
	close(responses)

	successCount := 0
	for recorder := range responses {
		result := decodeResult(t, recorder)
		switch result.Code {
		case e.SUCCESS:
			successCount++
			if recorder.Code != http.StatusOK {
				t.Fatalf("刷新成功时 HTTP 状态码为 %d", recorder.Code)
			}
		case e.RefreshTokenInvalid:
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("刷新令牌失效时 HTTP 状态码为 %d", recorder.Code)
			}
		default:
			t.Fatalf("并发刷新返回非预期结果: http=%d code=%d msg=%s", recorder.Code, result.Code, result.Msg)
		}
	}
	if successCount != 1 {
		t.Fatalf("期望并发刷新仅成功一次，实际成功 %d 次", successCount)
	}

	if err := db.First(originalLog, originalLog.ID).Error; err != nil {
		t.Fatalf("查询原始登录日志失败: %v", err)
	}
	if originalLog.IsRevoked != model.IsRevokedYes || originalLog.RevokedCode != model.RevokedCodeTokenRefresh {
		t.Fatalf("原始 refresh token 未正确撤销: %#v", originalLog)
	}
	var activeRefreshCount int64
	if err := db.Model(&model.AdminLoginLogs{}).
		Where("user_agent = ? AND type = ? AND is_revoked = ?", marker, model.LoginTypeRefresh, model.IsRevokedNo).
		Count(&activeRefreshCount).Error; err != nil {
		t.Fatalf("统计刷新登录日志失败: %v", err)
	}
	if activeRefreshCount != 1 {
		t.Fatalf("期望仅生成一条活跃刷新登录日志，实际为 %d", activeRefreshCount)
	}
}

// TestLogoutUpdatesDatabase 测试退出登录接口能够正确更新数据库中的 token 撤销状态。
func TestLogoutUpdatesDatabase(t *testing.T) {
	requireMySQL(t)

	// 先获取验证码
	captchaResp := anonymousGetRequest("/admin/v1/login-captcha", nil)
	assert.Equal(t, http.StatusOK, captchaResp.Code)
	captchaResult := decodeResult(t, captchaResp)
	if captchaResult.Code != e.SUCCESS {
		t.Fatalf("获取登录前置验证码失败：%s", captchaResult.Msg)
	}
	captchaData, ok := captchaResult.Data.(map[string]any)
	assert.True(t, ok)
	captchaID, _ := captchaData["id"].(string)
	// 验证码答案在测试环境下会返回
	captchaAnswer, _ := captchaData["answer"].(string)

	// 使用验证码登录
	loginData := map[string]any{
		"username":   "super_admin",
		"password":   "123456",
		"captcha":    captchaAnswer,
		"captcha_id": captchaID,
	}
	body, err := json.Marshal(loginData)
	assert.Nil(t, err)
	bodyStr := string(body)
	loginResp := anonymousPostRequest("/admin/v1/login", &bodyStr)
	assert.Equal(t, http.StatusOK, loginResp.Code)
	loginResult := decodeResult(t, loginResp)
	if loginResult.Code != e.SUCCESS {
		t.Fatalf("使用真实 MySQL 登录失败：%s", loginResult.Msg)
	}

	// 提取 token 中的 jwt_id 用于后续验证
	data, ok := loginResult.Data.(map[string]any)
	assert.True(t, ok)
	accessToken, ok := data["access_token"].(string)
	assert.True(t, ok)

	// 解析 token 获取 jwt_id
	claims, err := parseTestToken(accessToken)
	assert.Nil(t, err)
	jwtID := claims.ID

	// 调用退出登录接口（使用登录后返回的 token）
	logoutHeader := "Bearer " + accessToken
	logoutResp := performRequestWithHeaders(http.MethodPost, "/admin/v1/auth/logout", &bodyStr, logoutHeader, map[string]string{
		"X-Requested-With": "XMLHttpRequest",
	})
	assert.Equal(t, http.StatusOK, logoutResp.Code)
	logoutResult := decodeResult(t, logoutResp)
	assert.Equal(t, e.SUCCESS, logoutResult.Code)

	// 验证数据库中该 jwt_id 的记录被标记为已撤销
	loginLog := model.NewAdminLoginLogs()
	db, err := loginLog.GetDB()
	assert.Nil(t, err)
	err = db.Where("jwt_id = ? AND deleted_at = 0", jwtID).First(loginLog).Error
	assert.Nil(t, err)
	assert.Equal(t, uint8(1), loginLog.IsRevoked, "退出登录后 token 应被标记为已撤销")
	assert.Equal(t, uint8(1), loginLog.RevokedCode, "撤销原因码应为用户主动登出")
}
