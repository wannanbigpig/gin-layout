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
			name:  "登录日志列表需要登录",
			route: "/admin/v1/log/login/list",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
		{
			name:  "登录日志详情需要登录",
			route: "/admin/v1/log/login/detail",
			query: &url.Values{"id": {"1"}},
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
}
