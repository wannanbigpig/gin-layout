package admin_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestPublicDemoRoute(t *testing.T) {
	resp := anonymousGetRequest("/admin/v1/demo", nil)

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.SUCCESS, result.Code)
}

func TestProtectedPostRoutesRequireLogin(t *testing.T) {
	testCases := []struct {
		name  string
		route string
		body  string
	}{
		{name: "上传文件需要登录", route: "/admin/v1/common/upload", body: `{}`},
		{name: "退出登录需要登录", route: "/admin/v1/auth/logout", body: `{}`},
		{name: "管理员创建需要登录", route: "/admin/v1/admin-user/create", body: `{}`},
		{name: "管理员更新需要登录", route: "/admin/v1/admin-user/update", body: `{}`},
		{name: "管理员删除需要登录", route: "/admin/v1/admin-user/delete", body: `{}`},
		{name: "管理员绑定角色需要登录", route: "/admin/v1/admin-user/bind-role", body: `{}`},
		{name: "菜单创建需要登录", route: "/admin/v1/menu/create", body: `{}`},
		{name: "菜单更新需要登录", route: "/admin/v1/menu/update", body: `{}`},
		{name: "菜单删除需要登录", route: "/admin/v1/menu/delete", body: `{}`},
		{name: "刷新菜单权限需要登录", route: "/admin/v1/menu/update-all-menu-permissions", body: `{}`},
		{name: "角色创建需要登录", route: "/admin/v1/role/create", body: `{}`},
		{name: "角色更新需要登录", route: "/admin/v1/role/update", body: `{}`},
		{name: "角色删除需要登录", route: "/admin/v1/role/delete", body: `{}`},
		{name: "部门创建需要登录", route: "/admin/v1/department/create", body: `{}`},
		{name: "部门更新需要登录", route: "/admin/v1/department/update", body: `{}`},
		{name: "部门删除需要登录", route: "/admin/v1/department/delete", body: `{}`},
		{name: "部门绑定角色需要登录", route: "/admin/v1/department/bind-role", body: `{}`},
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

func TestProtectedGetRoutesRequireLogin(t *testing.T) {
	testCases := []struct {
		name  string
		route string
		query *url.Values
	}{
		{name: "管理员详情需要登录", route: "/admin/v1/admin-user/detail", query: &url.Values{"id": {"1"}}},
		{name: "管理员完整手机号需要登录", route: "/admin/v1/admin-user/get-full-phone", query: &url.Values{"id": {"1"}}},
		{name: "管理员完整邮箱需要登录", route: "/admin/v1/admin-user/get-full-email", query: &url.Values{"id": {"1"}}},
		{name: "菜单详情需要登录", route: "/admin/v1/menu/detail", query: &url.Values{"id": {"1"}}},
		{name: "角色详情需要登录", route: "/admin/v1/role/detail", query: &url.Values{"id": {"1"}}},
		{name: "部门详情需要登录", route: "/admin/v1/department/detail", query: &url.Values{"id": {"1"}}},
		{name: "请求日志详情需要登录", route: "/admin/v1/log/request/detail", query: &url.Values{"id": {"1"}}},
		{name: "登录日志详情需要登录", route: "/admin/v1/log/login/detail", query: &url.Values{"id": {"1"}}},
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
