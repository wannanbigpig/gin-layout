package admin_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestRoleListWithAuthorization(t *testing.T) {
	requireMySQL(t)

	resp := getRequest("/admin/v1/role/list", &url.Values{"page": {"1"}, "per_page": {"5"}})

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.SUCCESS, result.Code)
}

func TestRoleOptionsWithAuthorization(t *testing.T) {
	requireMySQL(t)

	resp := getRequest("/admin/v1/role/options", &url.Values{"keyword": {"超级"}})
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, resp).Code)
}

func TestRoleListRequiresLogin(t *testing.T) {
	resp := anonymousGetRequest("/admin/v1/role/list", &url.Values{"page": {"1"}, "per_page": {"5"}})

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
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

			assert.Equal(t, http.StatusUnauthorized, resp.Code)
			result := decodeResult(t, resp)
			assert.Equal(t, e.NotLogin, result.Code)
		})
	}

	resp := anonymousGetRequest("/admin/v1/role/detail", &url.Values{"id": {"1"}})
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.NotLogin, result.Code)
}

func TestRoleWriteFlow(t *testing.T) {
	requireWritableDB(t)

	name := uniqueTestName("role")
	roleCode := uniqueCompactTestName("rolecode")
	cleanupRoles(t, testResourcePrefix+"role")
	homeMenu := findMenuByTitle(t, "首页")
	permissionMenu := findMenuByTitle(t, "权限管理")
	adminUserMenu := findMenuByTitle(t, "管理员")
	adminUserListMenu := findMenuByCode(t, "adminUser:list")
	dashboardOverviewMenu := findMenuByCode(t, "dashboard:overview")
	dashboardStatisticsMenu := findMenuByCode(t, "dashboard:statistics")

	createBody := map[string]any{
		"code":      roleCode,
		"name":      name,
		"status":    1,
		"sort":      10,
		"menu_list": []uint{homeMenu.ID, permissionMenu.ID, adminUserMenu.ID, adminUserListMenu.ID},
	}
	createBytes, _ := json.Marshal(createBody)
	createPayload := string(createBytes)
	createResp := postRequest("/admin/v1/role/create", &createPayload)
	createResult := decodeResult(t, createResp)
	assert.Equal(t, e.SUCCESS, createResult.Code)

	role := findRoleByName(t, name)
	assert.Equal(t, roleCode, role.Code)

	detailResp := getRequest("/admin/v1/role/detail", &url.Values{"id": {strconv.FormatUint(uint64(role.ID), 10)}})
	detailResult := decodeResult(t, detailResp)
	assert.Equal(t, e.SUCCESS, detailResult.Code)
	detailData := detailResult.Data.(map[string]any)
	assert.ElementsMatch(t, []any{float64(adminUserListMenu.ID)}, detailData["menu_list"])

	storedMenuIDs, err := model.NewRoleMenuMap().MenuIdsByRoleIds([]uint{role.ID})
	assert.NoError(t, err)
	assert.ElementsMatch(t, []uint{adminUserListMenu.ID}, storedMenuIDs)

	rejectCodeUpdateBody := map[string]any{
		"id":        role.ID,
		"code":      roleCode + "_new",
		"name":      name,
		"status":    1,
		"sort":      20,
		"menu_list": []uint{dashboardOverviewMenu.ID},
	}
	rejectCodeUpdateBytes, _ := json.Marshal(rejectCodeUpdateBody)
	rejectCodeUpdatePayload := string(rejectCodeUpdateBytes)
	rejectCodeUpdateResp := postRequest("/admin/v1/role/update", &rejectCodeUpdatePayload)
	rejectCodeUpdateResult := decodeResult(t, rejectCodeUpdateResp)
	assert.Equal(t, e.InvalidParameter, rejectCodeUpdateResult.Code)

	role = findRoleByName(t, name)
	assert.Equal(t, roleCode, role.Code)

	updateBody := map[string]any{
		"id":          role.ID,
		"name":        name,
		"description": "测试角色-更新",
		"status":      1,
		"sort":        20,
		"menu_list":   []uint{homeMenu.ID, dashboardOverviewMenu.ID, dashboardStatisticsMenu.ID},
	}
	updateBytes, _ := json.Marshal(updateBody)
	updatePayload := string(updateBytes)
	updateResp := postRequest("/admin/v1/role/update", &updatePayload)
	updateResult := decodeResult(t, updateResp)
	assert.Equal(t, e.SUCCESS, updateResult.Code)

	role = findRoleByName(t, name)
	assert.Equal(t, roleCode, role.Code)

	detailResp = getRequest("/admin/v1/role/detail", &url.Values{"id": {strconv.FormatUint(uint64(role.ID), 10)}})
	detailResult = decodeResult(t, detailResp)
	assert.Equal(t, e.SUCCESS, detailResult.Code)
	detailData = detailResult.Data.(map[string]any)
	assert.ElementsMatch(t, []any{float64(dashboardOverviewMenu.ID), float64(dashboardStatisticsMenu.ID)}, detailData["menu_list"])

	storedMenuIDs, err = model.NewRoleMenuMap().MenuIdsByRoleIds([]uint{role.ID})
	assert.NoError(t, err)
	assert.ElementsMatch(t, []uint{dashboardOverviewMenu.ID, dashboardStatisticsMenu.ID}, storedMenuIDs)

	deleteBytes, _ := json.Marshal(map[string]any{"id": role.ID})
	deletePayload := string(deleteBytes)
	deleteResp := postRequest("/admin/v1/role/delete", &deletePayload)
	deleteResult := decodeResult(t, deleteResp)
	assert.Equal(t, e.SUCCESS, deleteResult.Code)
}
