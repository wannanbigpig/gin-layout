package admin_test

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestFileFolderFlowWithConfiguredMySQL(t *testing.T) {
	requireWritableDB(t)

	db, err := model.GetDB()
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}
	namePrefix := uniqueTestName("folder")
	defer func() {
		if err := db.Unscoped().Where("name LIKE ?", namePrefix+"%").Delete(&model.UploadFileFolder{}).Error; err != nil {
			t.Errorf("清理目录测试数据失败: %v", err)
		}
	}()

	parentPayload := mustJSONBody(t, map[string]any{"name": namePrefix + "-parent"})
	parentResp := postRequest("/admin/v1/system/file/folder/create", &parentPayload)
	assert.Equal(t, http.StatusOK, parentResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, parentResp).Code)
	parent := findUploadFolderByName(t, namePrefix+"-parent")

	childPayload := mustJSONBody(t, map[string]any{"name": namePrefix + "-child", "parent_id": parent.ID})
	childResp := postRequest("/admin/v1/system/file/folder/create", &childPayload)
	assert.Equal(t, http.StatusOK, childResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, childResp).Code)
	child := findUploadFolderByName(t, namePrefix+"-child")

	updatedName := namePrefix + "-child-updated"
	updatePayload := mustJSONBody(t, map[string]any{"id": child.ID, "name": updatedName})
	updateResp := postRequest("/admin/v1/system/file/folder/update", &updatePayload)
	assert.Equal(t, http.StatusOK, updateResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, updateResp).Code)

	statsResp := getRequest("/admin/v1/system/file/folder/stats", &url.Values{"id": {strconv.FormatUint(uint64(parent.ID), 10)}})
	assert.Equal(t, http.StatusOK, statsResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, statsResp).Code)

	treeResp := getRequest("/admin/v1/system/file/folder/tree", nil)
	assert.Equal(t, http.StatusOK, treeResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, treeResp).Code)

	movePayload := mustJSONBody(t, map[string]any{"id": child.ID, "target_parent_id": 0})
	moveResp := postRequest("/admin/v1/system/file/folder/move", &movePayload)
	assert.Equal(t, http.StatusOK, moveResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, moveResp).Code)

	for _, folderID := range []uint{child.ID, parent.ID} {
		deletePayload := mustJSONBody(t, map[string]any{"id": folderID, "confirm": 1, "deleted_reason": "接口测试清理"})
		deleteResp := postRequest("/admin/v1/system/file/folder/delete", &deletePayload)
		assert.Equal(t, http.StatusOK, deleteResp.Code)
		assert.Equal(t, e.SUCCESS, decodeResult(t, deleteResp).Code)
	}
}

func TestFileResourceReadRoutesWithConfiguredMySQL(t *testing.T) {
	requireMySQL(t)

	testCases := []struct {
		route string
		query *url.Values
		code  int
		biz   int
	}{
		{route: "/admin/v1/system/file/list", query: &url.Values{"page": {"1"}, "per_page": {"10"}}, code: http.StatusOK, biz: e.SUCCESS},
		{route: "/admin/v1/system/file/trash/list", query: &url.Values{"page": {"1"}, "per_page": {"10"}}, code: http.StatusOK, biz: e.SUCCESS},
		{route: "/admin/v1/system/file/references", query: &url.Values{"page": {"1"}, "per_page": {"10"}}, code: http.StatusOK, biz: e.SUCCESS},
		{route: "/admin/v1/system/file/detail", query: nil, code: http.StatusBadRequest, biz: e.InvalidParameter},
	}
	for _, tc := range testCases {
		t.Run(tc.route, func(t *testing.T) {
			resp := getRequest(tc.route, tc.query)
			assert.Equal(t, tc.code, resp.Code)
			assert.Equal(t, tc.biz, decodeResult(t, resp).Code)
		})
	}
}

func TestFileResourceMutationRoutesReachValidation(t *testing.T) {
	requireMySQL(t)

	testCases := []struct {
		route string
		body  string
	}{
		{route: "/admin/v1/system/file/update", body: `{}`},
		{route: "/admin/v1/system/file/move", body: `{}`},
		{route: "/admin/v1/system/file/upload/local", body: `{}`},
		{route: "/admin/v1/system/file/upload/credential", body: `{}`},
		{route: "/admin/v1/system/file/upload/credential/batch", body: `{}`},
		{route: "/admin/v1/system/file/upload/complete", body: `{}`},
		{route: "/admin/v1/system/file/upload/complete/batch", body: `{}`},
		{route: "/admin/v1/system/file/upload/multipart/init", body: `{}`},
		{route: "/admin/v1/system/file/upload/multipart/complete", body: `{}`},
		{route: "/admin/v1/system/file/upload/multipart/abort", body: `{}`},
		{route: "/admin/v1/system/file/delete", body: `{}`},
		{route: "/admin/v1/system/file/batch-delete", body: `{}`},
		{route: "/admin/v1/system/file/trash/restore", body: `{}`},
		{route: "/admin/v1/system/file/trash/batch-restore", body: `{}`},
		{route: "/admin/v1/system/file/trash/destroy", body: `{}`},
		{route: "/admin/v1/system/file/trash/batch-destroy", body: `{}`},
		{route: "/admin/v1/system/file/export", body: `{"storage_driver":"invalid"}`},
	}
	for _, tc := range testCases {
		t.Run(tc.route, func(t *testing.T) {
			body := tc.body
			resp := postRequest(tc.route, &body)
			assert.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
			assert.Equal(t, e.InvalidParameter, decodeResult(t, resp).Code)
		})
	}
}

func findUploadFolderByName(t *testing.T, name string) *model.UploadFileFolder {
	t.Helper()
	folder := model.NewUploadFileFolder()
	db, err := model.GetDB()
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}
	if err := db.Where("name = ?", name).First(folder).Error; err != nil {
		t.Fatalf("查询目录测试数据失败: %v", err)
	}
	return folder
}
