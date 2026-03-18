package admin_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestPublicFileRouteWithoutAuthorization(t *testing.T) {
	resp := anonymousGetRequest("/admin/v1/file/not-found-uuid", nil)

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.NotEqual(t, e.NotLogin, result.Code)
}

func TestProtectedApiFlowsWithAuthorization(t *testing.T) {
	requireMySQL(t)

	testCases := []struct {
		name  string
		route string
		query *url.Values
	}{
		{
			name:  "管理员列表",
			route: "/admin/v1/admin-user/list",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
		{
			name:  "菜单列表",
			route: "/admin/v1/menu/list",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
		{
			name:  "角色列表",
			route: "/admin/v1/role/list",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
		{
			name:  "部门列表",
			route: "/admin/v1/department/list",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
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

func TestProtectedApiRequiresLogin(t *testing.T) {
	testCases := []struct {
		name  string
		route string
		query *url.Values
	}{
		{
			name:  "管理员列表需要登录",
			route: "/admin/v1/admin-user/list",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
		{
			name:  "菜单列表需要登录",
			route: "/admin/v1/menu/list",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
		{
			name:  "角色列表需要登录",
			route: "/admin/v1/role/list",
			query: &url.Values{"page": {"1"}, "per_page": {"5"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := anonymousGetRequest(tc.route, tc.query)

			assert.Equal(t, http.StatusOK, resp.Code)
			result := decodeResult(t, resp)
			assert.Equal(t, e.NotLogin, result.Code)
		})
	}
}
