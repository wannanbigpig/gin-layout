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

func TestSystemConfigWriteFlow(t *testing.T) {
	requireWritableDB(t)
	requireSystemModuleTables(t)

	configKeyPrefix := uniqueCompactTestName("cfg")
	configKey := configKeyPrefix + ".key"
	configNameZh := configKeyPrefix + "-zh"
	configNameEn := configKeyPrefix + "-en"
	updatedConfigValue := configKeyPrefix + "-updated"

	cleanupSysConfigsByKeyPrefix(t, configKeyPrefix)
	t.Cleanup(func() {
		cleanupSysConfigsByKeyPrefix(t, configKeyPrefix)
	})

	createBody := map[string]any{
		"config_key": configKey,
		"config_name_i18n": map[string]string{
			"zh-CN": configNameZh,
			"en-US": configNameEn,
		},
		"config_value": "init-value",
		"value_type":   "string",
		"group_code":   "test",
		"status":       1,
		"sort":         10,
	}
	createBytes, _ := json.Marshal(createBody)
	createPayload := string(createBytes)
	createResp := postRequest("/admin/v1/system/config/create", &createPayload)
	createResult := decodeResult(t, createResp)
	assert.Equal(t, e.SUCCESS, createResult.Code, createResult.Msg)

	config := findSysConfigByKey(t, configKey)

	listResp := getRequest("/admin/v1/system/config/list", &url.Values{
		"page":       {"1"},
		"per_page":   {"20"},
		"config_key": {configKey},
	})
	listResult := decodeResult(t, listResp)
	assert.Equal(t, e.SUCCESS, listResult.Code, listResult.Msg)
	assert.True(t, collectionContainsFieldValue(listResult.Data, "config_key", configKey))

	detailResp := getRequest("/admin/v1/system/config/detail", &url.Values{
		"id": {strconv.FormatUint(uint64(config.ID), 10)},
	})
	detailResult := decodeResult(t, detailResp)
	assert.Equal(t, e.SUCCESS, detailResult.Code, detailResult.Msg)
	detailData, ok := detailResult.Data.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, configKey, detailData["config_key"])
	assert.NotEmpty(t, detailData["config_name"])
	configNameI18n, ok := detailData["config_name_i18n"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, configNameZh, configNameI18n["zh-CN"])
	assert.Equal(t, configNameEn, configNameI18n["en-US"])

	valueResp := getRequest("/admin/v1/system/config/value", &url.Values{
		"config_key": {configKey},
	})
	valueResult := decodeResult(t, valueResp)
	assert.Equal(t, e.SUCCESS, valueResult.Code, valueResult.Msg)
	valueData, ok := valueResult.Data.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, configKey, valueData["config_key"])
	assert.Equal(t, "init-value", valueData["config_value"])

	updateBody := map[string]any{
		"id":         config.ID,
		"config_key": configKey,
		"config_name_i18n": map[string]string{
			"en-US": configNameEn + "-u",
		},
		"config_value": updatedConfigValue,
		"value_type":   "string",
		"group_code":   "test",
		"status":       1,
		"sort":         20,
	}
	updateBytes, _ := json.Marshal(updateBody)
	updatePayload := string(updateBytes)
	updateResp := postRequest("/admin/v1/system/config/update", &updatePayload)
	updateResult := decodeResult(t, updateResp)
	assert.Equal(t, e.SUCCESS, updateResult.Code, updateResult.Msg)

	refeshBody := `{}`
	refreshResp := postRequest("/admin/v1/system/config/refresh", &refeshBody)
	refreshResult := decodeResult(t, refreshResp)
	assert.Equal(t, e.SUCCESS, refreshResult.Code, refreshResult.Msg)

	updatedDetailResp := getRequest("/admin/v1/system/config/detail", &url.Values{
		"id": {strconv.FormatUint(uint64(config.ID), 10)},
	})
	updatedDetailResult := decodeResult(t, updatedDetailResp)
	assert.Equal(t, e.SUCCESS, updatedDetailResult.Code, updatedDetailResult.Msg)
	updatedDetailData, ok := updatedDetailResult.Data.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, updatedConfigValue, updatedDetailData["config_value"])
	updatedNameI18n, ok := updatedDetailData["config_name_i18n"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, configNameZh, updatedNameI18n["zh-CN"])
	assert.Equal(t, configNameEn+"-u", updatedNameI18n["en-US"])

	deleteBytes, _ := json.Marshal(map[string]any{"id": config.ID})
	deletePayload := string(deleteBytes)
	deleteResp := postRequest("/admin/v1/system/config/delete", &deletePayload)
	deleteResult := decodeResult(t, deleteResp)
	assert.Equal(t, e.SUCCESS, deleteResult.Code, deleteResult.Msg)

	deletedDetailResp := getRequest("/admin/v1/system/config/detail", &url.Values{
		"id": {strconv.FormatUint(uint64(config.ID), 10)},
	})
	deletedDetailResult := decodeResult(t, deletedDetailResp)
	assert.Equal(t, e.NotFound, deletedDetailResult.Code)
}

func TestSystemDictWriteFlow(t *testing.T) {
	requireWritableDB(t)
	requireSystemModuleTables(t)

	typeCodePrefix := uniqueCompactTestName("dict")
	typeCode := typeCodePrefix + "_type"
	typeNameZh := typeCodePrefix + "-zh"
	typeNameEn := typeCodePrefix + "-en"
	itemValue := "v1"
	itemLabelZh := typeCodePrefix + "-label-zh"
	itemLabelEn := typeCodePrefix + "-label-en"

	cleanupSysDictByTypeCodePrefix(t, typeCodePrefix)
	t.Cleanup(func() {
		cleanupSysDictByTypeCodePrefix(t, typeCodePrefix)
	})

	createTypeBody := map[string]any{
		"type_code": typeCode,
		"type_name_i18n": map[string]string{
			"zh-CN": typeNameZh,
			"en-US": typeNameEn,
		},
		"status": 1,
		"sort":   10,
	}
	createTypeBytes, _ := json.Marshal(createTypeBody)
	createTypePayload := string(createTypeBytes)
	createTypeResp := postRequest("/admin/v1/system/dict/type/create", &createTypePayload)
	createTypeResult := decodeResult(t, createTypeResp)
	assert.Equal(t, e.SUCCESS, createTypeResult.Code, createTypeResult.Msg)

	dictType := findSysDictTypeByCode(t, typeCode)

	typeListResp := getRequest("/admin/v1/system/dict/type/list", &url.Values{
		"page":      {"1"},
		"per_page":  {"20"},
		"type_code": {typeCode},
	})
	typeListResult := decodeResult(t, typeListResp)
	assert.Equal(t, e.SUCCESS, typeListResult.Code, typeListResult.Msg)
	assert.True(t, collectionContainsFieldValue(typeListResult.Data, "type_code", typeCode))

	typeDetailResp := getRequest("/admin/v1/system/dict/type/detail", &url.Values{
		"id": {strconv.FormatUint(uint64(dictType.ID), 10)},
	})
	typeDetailResult := decodeResult(t, typeDetailResp)
	assert.Equal(t, e.SUCCESS, typeDetailResult.Code, typeDetailResult.Msg)
	typeDetailData, ok := typeDetailResult.Data.(map[string]any)
	assert.True(t, ok)
	typeNameI18n, ok := typeDetailData["type_name_i18n"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, typeNameZh, typeNameI18n["zh-CN"])
	assert.Equal(t, typeNameEn, typeNameI18n["en-US"])

	updateTypeBody := map[string]any{
		"id":        dictType.ID,
		"type_code": typeCode,
		"type_name_i18n": map[string]string{
			"en-US": typeNameEn + "-u",
		},
		"status": 1,
		"sort":   20,
	}
	updateTypeBytes, _ := json.Marshal(updateTypeBody)
	updateTypePayload := string(updateTypeBytes)
	updateTypeResp := postRequest("/admin/v1/system/dict/type/update", &updateTypePayload)
	updateTypeResult := decodeResult(t, updateTypeResp)
	assert.Equal(t, e.SUCCESS, updateTypeResult.Code, updateTypeResult.Msg)

	createItemBody := map[string]any{
		"type_code": typeCode,
		"label_i18n": map[string]string{
			"zh-CN": itemLabelZh,
			"en-US": itemLabelEn,
		},
		"value":      itemValue,
		"color":      "success",
		"tag_type":   "success",
		"is_default": 1,
		"status":     1,
		"sort":       10,
	}
	createItemBytes, _ := json.Marshal(createItemBody)
	createItemPayload := string(createItemBytes)
	createItemResp := postRequest("/admin/v1/system/dict/item/create", &createItemPayload)
	createItemResult := decodeResult(t, createItemResp)
	assert.Equal(t, e.SUCCESS, createItemResult.Code, createItemResult.Msg)

	item := findSysDictItemByTypeAndValue(t, typeCode, itemValue)

	itemListResp := getRequest("/admin/v1/system/dict/item/list", &url.Values{
		"page":      {"1"},
		"per_page":  {"20"},
		"type_code": {typeCode},
	})
	itemListResult := decodeResult(t, itemListResp)
	assert.Equal(t, e.SUCCESS, itemListResult.Code, itemListResult.Msg)
	assert.True(t, collectionContainsFieldValue(itemListResult.Data, "value", itemValue))

	optionsResp := getRequest("/admin/v1/system/dict/options", &url.Values{
		"type_code": {typeCode},
	})
	optionsResult := decodeResult(t, optionsResp)
	assert.Equal(t, e.SUCCESS, optionsResult.Code, optionsResult.Msg)
	assert.True(t, plainListContainsFieldValue(optionsResult.Data, "value", itemValue))

	updateItemBody := map[string]any{
		"id":        item.ID,
		"type_code": typeCode,
		"label_i18n": map[string]string{
			"en-US": itemLabelEn + "-u",
		},
		"value":      itemValue,
		"color":      "info",
		"tag_type":   "warning",
		"is_default": 1,
		"status":     1,
		"sort":       15,
	}
	updateItemBytes, _ := json.Marshal(updateItemBody)
	updateItemPayload := string(updateItemBytes)
	updateItemResp := postRequest("/admin/v1/system/dict/item/update", &updateItemPayload)
	updateItemResult := decodeResult(t, updateItemResp)
	assert.Equal(t, e.SUCCESS, updateItemResult.Code, updateItemResult.Msg)

	deleteItemBytes, _ := json.Marshal(map[string]any{"id": item.ID})
	deleteItemPayload := string(deleteItemBytes)
	deleteItemResp := postRequest("/admin/v1/system/dict/item/delete", &deleteItemPayload)
	deleteItemResult := decodeResult(t, deleteItemResp)
	assert.Equal(t, e.SUCCESS, deleteItemResult.Code, deleteItemResult.Msg)

	deleteTypeBytes, _ := json.Marshal(map[string]any{"id": dictType.ID})
	deleteTypePayload := string(deleteTypeBytes)
	deleteTypeResp := postRequest("/admin/v1/system/dict/type/delete", &deleteTypePayload)
	deleteTypeResult := decodeResult(t, deleteTypeResp)
	assert.Equal(t, e.SUCCESS, deleteTypeResult.Code, deleteTypeResult.Msg)

	deletedTypeDetailResp := getRequest("/admin/v1/system/dict/type/detail", &url.Values{
		"id": {strconv.FormatUint(uint64(dictType.ID), 10)},
	})
	deletedTypeDetailResult := decodeResult(t, deletedTypeDetailResp)
	assert.Equal(t, e.NotFound, deletedTypeDetailResult.Code)
}

func TestSystemConfigProtectedRoutesRequireLogin(t *testing.T) {
	postCases := []struct {
		name  string
		route string
		body  string
	}{
		{name: "系统参数创建需要登录", route: "/admin/v1/system/config/create", body: `{}`},
		{name: "系统参数更新需要登录", route: "/admin/v1/system/config/update", body: `{}`},
		{name: "系统参数删除需要登录", route: "/admin/v1/system/config/delete", body: `{}`},
		{name: "系统参数刷新缓存需要登录", route: "/admin/v1/system/config/refresh", body: `{}`},
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

	getCases := []struct {
		name  string
		route string
		query *url.Values
	}{
		{name: "系统参数列表需要登录", route: "/admin/v1/system/config/list", query: &url.Values{"page": {"1"}}},
		{name: "系统参数详情需要登录", route: "/admin/v1/system/config/detail", query: &url.Values{"id": {"1"}}},
		{name: "系统参数值需要登录", route: "/admin/v1/system/config/value", query: &url.Values{"config_key": {"system.site_name"}}},
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

func TestSystemDictProtectedRoutesRequireLogin(t *testing.T) {
	postCases := []struct {
		name  string
		route string
		body  string
	}{
		{name: "字典类型创建需要登录", route: "/admin/v1/system/dict/type/create", body: `{}`},
		{name: "字典类型更新需要登录", route: "/admin/v1/system/dict/type/update", body: `{}`},
		{name: "字典类型删除需要登录", route: "/admin/v1/system/dict/type/delete", body: `{}`},
		{name: "字典项创建需要登录", route: "/admin/v1/system/dict/item/create", body: `{}`},
		{name: "字典项更新需要登录", route: "/admin/v1/system/dict/item/update", body: `{}`},
		{name: "字典项删除需要登录", route: "/admin/v1/system/dict/item/delete", body: `{}`},
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

	getCases := []struct {
		name  string
		route string
		query *url.Values
	}{
		{name: "字典类型列表需要登录", route: "/admin/v1/system/dict/type/list", query: &url.Values{"page": {"1"}}},
		{name: "字典类型详情需要登录", route: "/admin/v1/system/dict/type/detail", query: &url.Values{"id": {"1"}}},
		{name: "字典项列表需要登录", route: "/admin/v1/system/dict/item/list", query: &url.Values{"type_code": {"common_status"}}},
		{name: "字典选项需要登录", route: "/admin/v1/system/dict/options", query: &url.Values{"type_code": {"common_status"}}},
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

func collectionContainsFieldValue(collectionData any, field string, target any) bool {
	collectionMap, ok := collectionData.(map[string]any)
	if !ok {
		return false
	}
	rows, ok := collectionMap["data"].([]any)
	if !ok {
		return false
	}
	for _, row := range rows {
		rowMap, ok := row.(map[string]any)
		if !ok {
			continue
		}
		if rowMap[field] == target {
			return true
		}
	}
	return false
}

func plainListContainsFieldValue(data any, field string, target any) bool {
	rows, ok := data.([]any)
	if !ok {
		return false
	}
	for _, row := range rows {
		rowMap, ok := row.(map[string]any)
		if !ok {
			continue
		}
		if rowMap[field] == target {
			return true
		}
	}
	return false
}

// requireSystemModuleTables 确认系统参数/字典相关表已迁移。
func requireSystemModuleTables(t *testing.T) {
	t.Helper()
	db, err := model.GetDB()
	if err != nil {
		t.Skipf("数据库不可用，跳过 system 成功路径测试: %v", err)
	}
	requiredTables := []string{
		"sys_config",
		"sys_config_i18n",
		"sys_dict_type",
		"sys_dict_type_i18n",
		"sys_dict_item",
		"sys_dict_item_i18n",
	}
	for _, table := range requiredTables {
		if !db.Migrator().HasTable(table) {
			t.Skipf("测试库缺少表 %s，跳过 system 成功路径测试", table)
		}
	}
}
