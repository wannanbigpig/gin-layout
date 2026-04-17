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

func TestMenuListWithAuthorization(t *testing.T) {
	requireMySQL(t)

	resp := getRequest("/admin/v1/menu/list", &url.Values{"page": {"1"}, "per_page": {"5"}})

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.SUCCESS, result.Code)
}

func TestMenuListRequiresLogin(t *testing.T) {
	resp := anonymousGetRequest("/admin/v1/menu/list", &url.Values{"page": {"1"}, "per_page": {"5"}})

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.NotLogin, result.Code)
}

func TestMenuProtectedRoutesRequireLogin(t *testing.T) {
	postCases := []struct {
		name  string
		route string
		body  string
	}{
		{name: "菜单创建需要登录", route: "/admin/v1/menu/create", body: `{}`},
		{name: "菜单更新需要登录", route: "/admin/v1/menu/update", body: `{}`},
		{name: "菜单删除需要登录", route: "/admin/v1/menu/delete", body: `{}`},
		{name: "刷新菜单权限需要登录", route: "/admin/v1/menu/update-all-menu-permissions", body: `{}`},
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

	resp := anonymousGetRequest("/admin/v1/menu/detail", &url.Values{"id": {"1"}})
	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.NotLogin, result.Code)
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
