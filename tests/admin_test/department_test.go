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

func TestDepartmentListWithAuthorization(t *testing.T) {
	requireMySQL(t)

	resp := getRequest("/admin/v1/department/list", &url.Values{"page": {"1"}, "per_page": {"5"}})

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.SUCCESS, result.Code)
}

func TestDepartmentListRequiresLogin(t *testing.T) {
	resp := anonymousGetRequest("/admin/v1/department/list", &url.Values{"page": {"1"}, "per_page": {"5"}})

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.NotLogin, result.Code)
}

func TestDepartmentProtectedRoutesRequireLogin(t *testing.T) {
	postCases := []struct {
		name  string
		route string
		body  string
	}{
		{name: "部门创建需要登录", route: "/admin/v1/department/create", body: `{}`},
		{name: "部门更新需要登录", route: "/admin/v1/department/update", body: `{}`},
		{name: "部门删除需要登录", route: "/admin/v1/department/delete", body: `{}`},
		{name: "部门绑定角色需要登录", route: "/admin/v1/department/bind-role", body: `{}`},
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

	resp := anonymousGetRequest("/admin/v1/department/detail", &url.Values{"id": {"1"}})
	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.NotLogin, result.Code)
}

func TestDepartmentWriteFlow(t *testing.T) {
	requireWritableDB(t)

	name := uniqueTestName("dept")
	cleanupDepartments(t, testResourcePrefix+"dept")

	createBody := map[string]any{
		"name":        name,
		"description": "测试部门",
		"sort":        10,
	}
	createBytes, _ := json.Marshal(createBody)
	createPayload := string(createBytes)
	createResp := postRequest("/admin/v1/department/create", &createPayload)
	createResult := decodeResult(t, createResp)
	assert.Equal(t, e.SUCCESS, createResult.Code)

	dept := findDepartmentByName(t, name)

	detailResp := getRequest("/admin/v1/department/detail", &url.Values{"id": {strconv.FormatUint(uint64(dept.ID), 10)}})
	detailResult := decodeResult(t, detailResp)
	assert.Equal(t, e.SUCCESS, detailResult.Code)

	updateBody := map[string]any{
		"id":          dept.ID,
		"name":        name,
		"description": "测试部门-更新",
		"sort":        20,
	}
	updateBytes, _ := json.Marshal(updateBody)
	updatePayload := string(updateBytes)
	updateResp := postRequest("/admin/v1/department/update", &updatePayload)
	updateResult := decodeResult(t, updateResp)
	assert.Equal(t, e.SUCCESS, updateResult.Code)

	bindBytes, _ := json.Marshal(map[string]any{"dept_id": dept.ID, "role_ids": []uint{firstActiveRoleID(t)}})
	bindPayload := string(bindBytes)
	bindResp := postRequest("/admin/v1/department/bind-role", &bindPayload)
	bindResult := decodeResult(t, bindResp)
	assert.Equal(t, e.SUCCESS, bindResult.Code)

	deleteBytes, _ := json.Marshal(map[string]any{"id": dept.ID})
	deletePayload := string(deleteBytes)
	deleteResp := postRequest("/admin/v1/department/delete", &deletePayload)
	deleteResult := decodeResult(t, deleteResp)
	assert.Equal(t, e.SUCCESS, deleteResult.Code)
}
