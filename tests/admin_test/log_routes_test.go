package admin_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestLogListRoutesWithAuthorization(t *testing.T) {
	requireMySQL(t)

	testCases := []struct {
		name  string
		route string
		query *url.Values
	}{
		{
			name:  "请求日志列表",
			route: "/admin/v1/log/request/list",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
		{
			name:  "个人请求日志列表",
			route: "/admin/v1/log/request/self-list",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}, "operator_id": {"4294967295"}},
		},
		{
			name:  "登录日志列表",
			route: "/admin/v1/log/login/list",
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

func TestSelfRequestLogDetailNotFound(t *testing.T) {
	requireMySQL(t)

	resp := getRequest("/admin/v1/log/request/self-detail", &url.Values{"id": {"4294967295"}})
	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Equal(t, e.NotFound, decodeResult(t, resp).Code)
}

func TestRequestLogExportRouteWithAuthorization(t *testing.T) {
	requireMySQL(t)

	body := `{"limit":10}`
	resp := postRequest("/admin/v1/log/request/export", &body)
	result := decodeResult(t, resp)
	// 队列未启用时返回 ServiceDependencyNotReady（HTTP 503），启用时返回 SUCCESS（HTTP 200）
	assert.True(t, result.Code == e.SUCCESS || result.Code == e.ServiceDependencyNotReady,
		"expected SUCCESS or ServiceDependencyNotReady, got %d", result.Code)
	expectedHTTP := http.StatusOK
	if result.Code == e.ServiceDependencyNotReady {
		expectedHTTP = http.StatusServiceUnavailable
	}
	assert.Equal(t, expectedHTTP, resp.Code)
}

func TestExportRecordListRouteWithAuthorization(t *testing.T) {
	requireMySQL(t)

	resp := getRequest("/admin/v1/system/export/list", &url.Values{"page": {"1"}, "per_page": {"5"}})
	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.SUCCESS, result.Code)
}

func TestExportRecordDetailNotFound(t *testing.T) {
	requireMySQL(t)

	resp := getRequest("/admin/v1/system/export/detail", &url.Values{"id": {"4294967295"}})
	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Equal(t, e.NotFound, decodeResult(t, resp).Code)
}

func TestRequestLogMaskConfigRoutesWithAuthorization(t *testing.T) {
	resp := getRequest("/admin/v1/log/request/mask-config", nil)
	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.SUCCESS, result.Code)

	body := `{"common":["password","token"],"request_body":["phone"]}`
	updateResp := postRequest("/admin/v1/log/request/mask-config", &body)
	assert.Equal(t, http.StatusOK, updateResp.Code)
	updateResult := decodeResult(t, updateResp)
	assert.Equal(t, e.SUCCESS, updateResult.Code)
}

func TestLogRoutesRequireLogin(t *testing.T) {
	getCases := []struct {
		name  string
		route string
		query *url.Values
	}{
		{
			name:  "请求日志列表需要登录",
			route: "/admin/v1/log/request/list",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
		{
			name:  "请求日志详情需要登录",
			route: "/admin/v1/log/request/detail",
			query: &url.Values{"id": {"1"}},
		},
		{
			name:  "个人请求日志列表需要登录",
			route: "/admin/v1/log/request/self-list",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
		{
			name:  "个人请求日志详情需要登录",
			route: "/admin/v1/log/request/self-detail",
			query: &url.Values{"id": {"1"}},
		},
		{
			name:  "请求日志脱敏配置需要登录",
			route: "/admin/v1/log/request/mask-config",
			query: nil,
		},
		{
			name:  "登录日志列表需要登录",
			route: "/admin/v1/log/login/list",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
		{
			name:  "登录日志详情需要登录",
			route: "/admin/v1/log/login/detail",
			query: &url.Values{"id": {"1"}},
		},
		{
			name:  "导出记录列表需要登录",
			route: "/admin/v1/system/export/list",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
	}

	for _, tc := range getCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := anonymousGetRequest(tc.route, tc.query)

			assert.Equal(t, http.StatusUnauthorized, resp.Code)
			result := decodeResult(t, resp)
			assert.Equal(t, e.NotLogin, result.Code)
		})
	}

	postCases := []struct {
		name  string
		route string
		body  string
	}{
		{
			name:  "请求日志导出需要登录",
			route: "/admin/v1/log/request/export",
			body:  `{"limit":10}`,
		},
		{
			name:  "请求日志脱敏配置更新需要登录",
			route: "/admin/v1/log/request/mask-config",
			body:  `{"common":["password"]}`,
		},
	}

	for _, tc := range postCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := anonymousPostRequest(tc.route, &tc.body)
			assert.Equal(t, http.StatusUnauthorized, resp.Code)
			result := decodeResult(t, resp)
			assert.Equal(t, e.NotLogin, result.Code)
		})
	}
}
