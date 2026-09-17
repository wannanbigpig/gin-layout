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

func TestProductWriteFlowWithConfiguredMySQL(t *testing.T) {
	requireWritableDB(t)

	db, err := model.GetDB()
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}
	namePrefix := uniqueTestName("product")
	defer func() {
		if err := db.Unscoped().Where("name LIKE ?", namePrefix+"%").Delete(&model.Product{}).Error; err != nil {
			t.Errorf("清理产品测试数据失败: %v", err)
		}
		if err := db.Unscoped().Where("source_type = ? AND title LIKE ?", "product_activation", "启用产品："+namePrefix+"%").Delete(&model.UserTodo{}).Error; err != nil {
			t.Errorf("清理产品待办测试数据失败: %v", err)
		}
	}()

	var department model.Department
	if err := db.Where("deleted_at = 0").Order("id ASC").First(&department).Error; err != nil {
		t.Fatalf("产品流程缺少可用部门前置数据: %v", err)
	}

	createPayload := mustJSONBody(t, map[string]any{
		"name":        namePrefix,
		"description": "接口测试产品",
		"price":       1999,
		"status":      0,
		"dept_id":     department.ID,
	})
	createResp := postRequest("/admin/v1/product/create", &createPayload)
	assert.Equal(t, http.StatusOK, createResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, createResp).Code)

	var product model.Product
	if err := db.Where("name = ? AND deleted_at = 0", namePrefix).First(&product).Error; err != nil {
		t.Fatalf("查询已创建产品失败: %v", err)
	}
	var firstTodo model.UserTodo
	if err := db.Where("assignee_id = ? AND source_type = ? AND source_key = ? AND status = ?",
		product.CreatedBy, "product_activation", "product:"+strconv.FormatUint(uint64(product.ID), 10), model.TodoStatusPending).
		First(&firstTodo).Error; err != nil {
		t.Fatalf("禁用产品应创建待启用事项: %v", err)
	}

	listResp := getRequest("/admin/v1/product/list", &url.Values{"name": {namePrefix}, "page": {"1"}, "per_page": {"10"}})
	assert.Equal(t, http.StatusOK, listResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, listResp).Code)

	detailResp := getRequest("/admin/v1/product/detail", &url.Values{"id": {strconv.FormatUint(uint64(product.ID), 10)}})
	assert.Equal(t, http.StatusOK, detailResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, detailResp).Code)

	updatedName := namePrefix + "-updated"
	updatePayload := mustJSONBody(t, map[string]any{
		"id":          product.ID,
		"name":        updatedName,
		"description": "接口测试产品-更新",
		"price":       2999,
		"status":      0,
		"dept_id":     department.ID,
	})
	updateResp := postRequest("/admin/v1/product/update", &updatePayload)
	assert.Equal(t, http.StatusOK, updateResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, updateResp).Code)
	var updatedTodo model.UserTodo
	if err := db.Where("id = ?", firstTodo.ID).First(&updatedTodo).Error; err != nil {
		t.Fatalf("查询已更新产品待办失败: %v", err)
	}
	assert.Equal(t, "启用产品："+updatedName, updatedTodo.Title)

	enablePayload := mustJSONBody(t, map[string]any{
		"id": product.ID, "name": updatedName, "description": "接口测试产品-启用",
		"price": 2999, "status": 1, "dept_id": department.ID,
	})
	enableResp := postRequest("/admin/v1/product/update", &enablePayload)
	assert.Equal(t, http.StatusOK, enableResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, enableResp).Code)
	if err := db.Where("id = ?", firstTodo.ID).First(&updatedTodo).Error; err != nil {
		t.Fatalf("查询已完成产品待办失败: %v", err)
	}
	assert.Equal(t, model.TodoStatusCompleted, updatedTodo.Status)
	assert.NotNil(t, updatedTodo.CompletedAt)

	disableAgainPayload := mustJSONBody(t, map[string]any{
		"id": product.ID, "name": updatedName, "description": "接口测试产品-再次禁用",
		"price": 2999, "status": 0, "dept_id": department.ID,
	})
	disableAgainResp := postRequest("/admin/v1/product/update", &disableAgainPayload)
	assert.Equal(t, http.StatusOK, disableAgainResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, disableAgainResp).Code)
	var secondTodo model.UserTodo
	if err := db.Where("assignee_id = ? AND source_type = ? AND source_key = ? AND status = ?",
		product.CreatedBy, "product_activation", "product:"+strconv.FormatUint(uint64(product.ID), 10), model.TodoStatusPending).
		Order("id DESC").First(&secondTodo).Error; err != nil {
		t.Fatalf("再次禁用产品应创建新周期待办: %v", err)
	}
	assert.NotEqual(t, firstTodo.ID, secondTodo.ID)

	deletePayload := mustJSONBody(t, map[string]any{"id": product.ID})
	deleteResp := postRequest("/admin/v1/product/delete", &deletePayload)
	assert.Equal(t, http.StatusOK, deleteResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, deleteResp).Code)
	if err := db.Where("id = ?", secondTodo.ID).First(&secondTodo).Error; err != nil {
		t.Fatalf("查询已取消产品待办失败: %v", err)
	}
	assert.Equal(t, model.TodoStatusCanceled, secondTodo.Status)
	assert.NotNil(t, secondTodo.CanceledAt)

	var activeCount int64
	if err := db.Model(&model.Product{}).Where("id = ?", product.ID).Count(&activeCount).Error; err != nil {
		t.Fatalf("验证产品删除状态失败: %v", err)
	}
	assert.Zero(t, activeCount)
}

func mustJSONBody(t *testing.T, value any) string {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("序列化请求体失败: %v", err)
	}
	return string(payload)
}
