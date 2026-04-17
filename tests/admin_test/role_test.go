package admin_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestRoleListWithAuthorization(t *testing.T) {
	requireMySQL(t)

	resp := getRequest("/admin/v1/role/list", &url.Values{"page": {"1"}, "per_page": {"5"}})

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.SUCCESS, result.Code)
}

func TestRoleListRequiresLogin(t *testing.T) {
	resp := anonymousGetRequest("/admin/v1/role/list", &url.Values{"page": {"1"}, "per_page": {"5"}})

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.NotLogin, result.Code)
}

func TestRoleProtectedRoutesRequireLogin(t *testing.T) {
	postCases := []struct {
		name  string
		route string
		body  string
	}{
		{name: "角色创建需要登录", route: "/admin/v1/role/create", body: `{}`},
		{name: "角色更新需要登录", route: "/admin/v1/role/update", body: `{}`},
		{name: "角色删除需要登录", route: "/admin/v1/role/delete", body: `{}`},
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

	resp := anonymousGetRequest("/admin/v1/role/detail", &url.Values{"id": {"1"}})
	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.NotLogin, result.Code)
}

func TestRoleWriteFlow(t *testing.T) {
	requireWritableDB(t)

	name := uniqueTestName("role")
	cleanupRoles(t, testResourcePrefix+"role")

	createBody := map[string]any{
		"name":      name,
		"status":    1,
		"sort":      10,
		"menu_list": []uint{firstActiveMenuID(t)},
	}
	createBytes, _ := json.Marshal(createBody)
	createPayload := string(createBytes)
	createResp := postRequest("/admin/v1/role/create", &createPayload)
	createResult := decodeResult(t, createResp)
	assert.Equal(t, e.SUCCESS, createResult.Code)

	role := findRoleByName(t, name)

	detailResp := getRequest("/admin/v1/role/detail", &url.Values{"id": {strconv.FormatUint(uint64(role.ID), 10)}})
	detailResult := decodeResult(t, detailResp)
	assert.Equal(t, e.SUCCESS, detailResult.Code)

	updateBody := map[string]any{
		"id":          role.ID,
		"name":        name,
		"description": "测试角色-更新",
		"status":      1,
		"sort":        20,
		"menu_list":   []uint{firstActiveMenuID(t)},
	}
	updateBytes, _ := json.Marshal(updateBody)
	updatePayload := string(updateBytes)
	updateResp := postRequest("/admin/v1/role/update", &updatePayload)
	updateResult := decodeResult(t, updateResp)
	assert.Equal(t, e.SUCCESS, updateResult.Code)

	deleteBytes, _ := json.Marshal(map[string]any{"id": role.ID})
	deletePayload := string(deleteBytes)
	deleteResp := postRequest("/admin/v1/role/delete", &deletePayload)
	deleteResult := decodeResult(t, deleteResp)
	assert.Equal(t, e.SUCCESS, deleteResult.Code)
}
