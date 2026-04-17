package admin_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

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
