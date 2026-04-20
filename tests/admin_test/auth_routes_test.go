package admin_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/stretchr/testify/assert"

	c "github.com/wannanbigpig/gin-layout/config"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
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

	assert.Equal(t, http.StatusOK, resp.Code)
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

func TestProtectedAuthRoutesRequireLogin(t *testing.T) {
	testCases := []struct {
		name  string
		route string
		body  string
	}{
		{name: "退出登录需要登录", route: "/admin/v1/auth/logout", body: `{}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.body
			resp := anonymousPostRequest(tc.route, &body)

			assert.Equal(t, http.StatusOK, resp.Code)
			result := decodeResult(t, resp)
			assert.Equal(t, e.NotLogin, result.Code)
		})
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
		t.Skipf("获取验证码失败，跳过测试：%s", captchaResult.Msg)
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
		t.Skipf("登录失败，跳过测试：%s", loginResult.Msg)
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
	logoutResp := performRequest(http.MethodPost, "/admin/v1/auth/logout", &bodyStr, logoutHeader)
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
