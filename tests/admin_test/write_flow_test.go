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

	bindBytes, _ := json.Marshal(map[string]any{"id": dept.ID, "role_ids": []uint{firstActiveRoleID(t)}})
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

func TestMenuWriteFlow(t *testing.T) {
	requireWritableDB(t)

	title := uniqueTestName("menu")
	cleanupMenus(t, testResourcePrefix+"menu")

	createBody := map[string]any{
		"title":     title,
		"name":      title,
		"path":      "/" + title,
		"component": "test/component",
		"sort":      10,
		"type":      2,
		"status":    1,
		"is_show":   1,
		"is_auth":   1,
	}
	createBytes, _ := json.Marshal(createBody)
	createPayload := string(createBytes)
	createResp := postRequest("/admin/v1/menu/create", &createPayload)
	createResult := decodeResult(t, createResp)
	assert.Equal(t, e.SUCCESS, createResult.Code)

	menu := findMenuByTitle(t, title)

	detailResp := getRequest("/admin/v1/menu/detail", &url.Values{"id": {strconv.FormatUint(uint64(menu.ID), 10)}})
	detailResult := decodeResult(t, detailResp)
	assert.Equal(t, e.SUCCESS, detailResult.Code)

	updateBody := map[string]any{
		"id":        menu.ID,
		"title":     title + "-u",
		"name":      title + "-u",
		"path":      "/" + title + "-u",
		"component": "test/component",
		"sort":      20,
		"type":      2,
		"status":    1,
		"is_show":   1,
		"is_auth":   1,
	}
	updateBytes, _ := json.Marshal(updateBody)
	updatePayload := string(updateBytes)
	updateResp := postRequest("/admin/v1/menu/update", &updatePayload)
	updateResult := decodeResult(t, updateResp)
	assert.Equal(t, e.SUCCESS, updateResult.Code, updateResult.Msg)

	updatedMenu := findMenuByTitle(t, title+"-u")

	refreshBody := `{}`
	refreshResp := postRequest("/admin/v1/menu/update-all-menu-permissions", &refreshBody)
	refreshResult := decodeResult(t, refreshResp)
	assert.Equal(t, e.SUCCESS, refreshResult.Code, refreshResult.Msg)

	deleteBytes, _ := json.Marshal(map[string]any{"id": updatedMenu.ID})
	deletePayload := string(deleteBytes)
	deleteResp := postRequest("/admin/v1/menu/delete", &deletePayload)
	deleteResult := decodeResult(t, deleteResp)
	assert.Equal(t, e.SUCCESS, deleteResult.Code, deleteResult.Msg)
}
