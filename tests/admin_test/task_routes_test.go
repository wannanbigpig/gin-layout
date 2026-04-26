package admin_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestTaskCenterListRoutesWithAuthorization(t *testing.T) {
	requireMySQL(t)

	testCases := []struct {
		name  string
		route string
		query *url.Values
	}{
		{
			name:  "任务定义列表",
			route: "/admin/v1/task/list",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
		{
			name:  "任务执行记录列表",
			route: "/admin/v1/task/run/list",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
		{
			name:  "定时任务最近状态列表",
			route: "/admin/v1/task/cron/state",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := getRequest(tc.route, tc.query)
			assert.Equal(t, http.StatusOK, resp.Code)
			result := decodeResult(t, resp)
			assert.Equal(t, e.SUCCESS, result.Code)
		})
	}
}

func TestTaskCenterRunDetailNotFound(t *testing.T) {
	requireMySQL(t)

	resp := getRequest("/admin/v1/task/run/detail", &url.Values{"id": {"99999999"}})
	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.NotFound, result.Code)
}

func TestTaskCenterRoutesRequireLogin(t *testing.T) {
	getCases := []struct {
		name  string
		route string
		query *url.Values
	}{
		{
			name:  "任务定义列表需要登录",
			route: "/admin/v1/task/list",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
		{
			name:  "任务执行记录列表需要登录",
			route: "/admin/v1/task/run/list",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
		{name: "任务执行记录详情需要登录", route: "/admin/v1/task/run/detail", query: &url.Values{"id": {"1"}}},
		{
			name:  "定时任务最近状态需要登录",
			route: "/admin/v1/task/cron/state",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
	}

	for _, tc := range getCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := anonymousGetRequest(tc.route, tc.query)
			assert.Equal(t, http.StatusOK, resp.Code)
			result := decodeResult(t, resp)
			assert.Equal(t, e.NotLogin, result.Code)
		})
	}

	postCases := []struct {
		name  string
		route string
		body  string
	}{
		{name: "手动触发任务需要登录", route: "/admin/v1/task/trigger", body: `{"task_code":"demo:send"}`},
		{name: "重试任务需要登录", route: "/admin/v1/task/run/retry", body: `{"run_id":1}`},
		{name: "取消任务需要登录", route: "/admin/v1/task/run/cancel", body: `{"run_id":1}`},
	}
	for _, tc := range postCases {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.body
			resp := anonymousPostRequest(tc.route, &body)
			assert.Equal(t, http.StatusOK, resp.Code)
			result := decodeResult(t, resp)
			assert.Equal(t, e.NotLogin, result.Code)
		})
	}
}

func TestTaskCenterOperateRouteValidation(t *testing.T) {
	requireMySQL(t)

	triggerBody := `{}`
	triggerResp := postRequest("/admin/v1/task/trigger", &triggerBody)
	assert.Equal(t, http.StatusOK, triggerResp.Code)
	triggerResult := decodeResult(t, triggerResp)
	assert.Equal(t, e.InvalidParameter, triggerResult.Code)

	retryBody := `{}`
	retryResp := postRequest("/admin/v1/task/run/retry", &retryBody)
	assert.Equal(t, http.StatusOK, retryResp.Code)
	retryResult := decodeResult(t, retryResp)
	assert.Equal(t, e.InvalidParameter, retryResult.Code)

	cancelBody := `{}`
	cancelResp := postRequest("/admin/v1/task/run/cancel", &cancelBody)
	assert.Equal(t, http.StatusOK, cancelResp.Code)
	cancelResult := decodeResult(t, cancelResp)
	assert.Equal(t, e.InvalidParameter, cancelResult.Code)
}
