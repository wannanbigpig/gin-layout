package admin_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestGetAdminUserRequiresLogin(t *testing.T) {
	queryParams := &url.Values{}
	queryParams.Set("id", "1")
	resp := anonymousGetRequest("/admin/v1/admin-user/get", queryParams)

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.NotLogin, result.Code)
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

func TestAdminUserListWithAuthorization(t *testing.T) {
	requireMySQL(t)

	resp := getRequest("/admin/v1/admin-user/list", &url.Values{"page": {"1"}, "per_page": {"5"}})

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.SUCCESS, result.Code)
}

func TestAdminUserListRequiresLogin(t *testing.T) {
	resp := anonymousGetRequest("/admin/v1/admin-user/list", &url.Values{"page": {"1"}, "per_page": {"5"}})

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.NotLogin, result.Code)
}

func TestAdminUserProtectedGetRoutesRequireLogin(t *testing.T) {
	testCases := []struct {
		name  string
		route string
		query *url.Values
	}{
		{name: "管理员详情需要登录", route: "/admin/v1/admin-user/detail", query: &url.Values{"id": {"1"}}},
		{name: "管理员完整手机号需要登录", route: "/admin/v1/admin-user/get-full-phone", query: &url.Values{"id": {"1"}}},
		{name: "管理员完整邮箱需要登录", route: "/admin/v1/admin-user/get-full-email", query: &url.Values{"id": {"1"}}},
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

func TestAdminUserProtectedPostRoutesRequireLogin(t *testing.T) {
	testCases := []struct {
		name  string
		route string
		body  string
	}{
		{name: "管理员创建需要登录", route: "/admin/v1/admin-user/create", body: `{}`},
		{name: "管理员更新需要登录", route: "/admin/v1/admin-user/update", body: `{}`},
		{name: "管理员删除需要登录", route: "/admin/v1/admin-user/delete", body: `{}`},
		{name: "管理员绑定角色需要登录", route: "/admin/v1/admin-user/bind-role", body: `{}`},
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

func TestAdminUserWriteFlow(t *testing.T) {
	requireWritableDB(t)

	username := fmt.Sprintf("ta%d", time.Now().UnixNano()%1e10)
	cleanupAdminUsers(t, "ta")

	createBody := map[string]any{
		"username": username,
		"nickname": "测试管理员",
		"password": "12345678",
		"email":    username + "@example.com",
		"dept_ids": []uint{1},
		"status":   1,
	}
	bodyBytes, _ := json.Marshal(createBody)
	body := string(bodyBytes)

	resp := postRequest("/admin/v1/admin-user/create", &body)
	result := decodeResult(t, resp)
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, e.SUCCESS, result.Code)

	user := findAdminUserByUsername(t, username)

	detailResp := getRequest("/admin/v1/admin-user/detail", &url.Values{"id": {strconv.FormatUint(uint64(user.ID), 10)}})
	detailResult := decodeResult(t, detailResp)
	assert.Equal(t, e.SUCCESS, detailResult.Code)

	updateBody := map[string]any{
		"id":       user.ID,
		"nickname": "测试管理员-更新",
		"email":    username + "-updated@example.com",
		"dept_ids": []uint{1},
	}
	updateBytes, _ := json.Marshal(updateBody)
	updatePayload := string(updateBytes)
	updateResp := postRequest("/admin/v1/admin-user/update", &updatePayload)
	updateResult := decodeResult(t, updateResp)
	assert.Equal(t, e.SUCCESS, updateResult.Code)

	deleteBytes, _ := json.Marshal(map[string]any{"id": user.ID})
	deletePayload := string(deleteBytes)
	deleteResp := postRequest("/admin/v1/admin-user/delete", &deletePayload)
	deleteResult := decodeResult(t, deleteResp)
	assert.Equal(t, e.SUCCESS, deleteResult.Code)
}
