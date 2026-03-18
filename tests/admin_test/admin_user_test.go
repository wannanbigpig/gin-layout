package admin_test

import (
	"encoding/json"
	"net/http"
	"net/url"
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

func TestGetAdminUserRequiresLogin(t *testing.T) {
	queryParams := &url.Values{}
	queryParams.Set("id", "1")
	resp := anonymousGetRequest("/admin/v1/admin-user/get", queryParams)

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.NotLogin, result.Code)
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

func TestGetCurrentAdminUserWithAuthorization(t *testing.T) {
	requireMySQL(t)

	resp := getRequest("/admin/v1/admin-user/get", nil)

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.SUCCESS, result.Code)

	data, ok := result.Data.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, float64(1), data["id"])
}

func TestGetUserMenuInfoWithAuthorization(t *testing.T) {
	requireMySQL(t)

	resp := getRequest("/admin/v1/admin-user/user-menu-info", nil)

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.SUCCESS, result.Code)
}

func TestUpdateProfileInvalidEmail(t *testing.T) {
	requireMySQL(t)

	body := `{"email":"invalid-email"}`
	resp := postRequest("/admin/v1/admin-user/update-profile", &body)

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.InvalidParameter, result.Code)
}
