package admin_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

func TestLogin(t *testing.T) {
	// 获取验证码
	captchaRoute := ts.URL + "/api/v1/admin/login-captcha"
	captchaResp := getRequest(captchaRoute, nil)
	assert.Nil(t, captchaResp.Error)
	assert.Equal(t, http.StatusOK, captchaResp.Response.StatusCode)
	captchaResult := new(response.Result)
	err := captchaResp.ParseJson(captchaResult)
	assert.Nil(t, err)
	assert.Equal(t, e.SUCCESS, captchaResult.Code)

	// 登录
	route := ts.URL + "/api/v1/admin/login"
	h := utils.HttpRequest{}
	captchaData, ok := captchaResult.Data.(map[string]any)
	assert.True(t, ok)
	loginData := map[string]any{
		"username":   "super_admin",
		"password":   "123456",
		"captcha":    captchaData["answer"],
		"captcha_id": captchaData["id"],
	}
	body, err := json.Marshal(loginData)
	assert.Nil(t, err)
	resp := h.JsonRequest("POST", route, strings.NewReader(string(body)))

	assert.Nil(t, resp.Error)
	assert.Equal(t, http.StatusOK, resp.Response.StatusCode)
	result := new(response.Result)
	err = resp.ParseJson(result)
	assert.Nil(t, err)
	assert.Equal(t, e.SUCCESS, result.Code)
}

func TestGetAdminUser(t *testing.T) {
	route := ts.URL + "api/v1/admin/admin-user/get"
	queryParams := &url.Values{}
	queryParams.Set("id", "1")
	resp := getRequest(route, queryParams)

	assert.Nil(t, resp.Error)
	assert.Equal(t, http.StatusOK, resp.Response.StatusCode)
	result := new(response.Result)
	err := resp.ParseJson(result)
	assert.Nil(t, err)
	assert.Equal(t, e.SUCCESS, result.Code)
}
