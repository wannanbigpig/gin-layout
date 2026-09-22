package admin_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestPersonalTodoWriteFlowWithConfiguredMySQL(t *testing.T) {
	requireWritableDB(t)

	db, err := model.GetDB()
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}
	sourceKey := uniqueCompactTestName("todo")
	defer func() {
		if err := db.Unscoped().Where("assignee_id = ? AND source_type = ? AND source_key = ?", 1, model.TodoSourceManual, sourceKey).
			Delete(&model.UserTodo{}).Error; err != nil {
			t.Errorf("清理个人待办测试数据失败: %v", err)
		}
	}()

	createPayload := mustJSONBody(t, map[string]any{
		"source_key": sourceKey,
		"title":      "接口测试待办",
		"priority":   model.TodoPriorityHigh,
	})
	createResp := postRequest("/admin/v1/todo/create", &createPayload)
	assert.Equal(t, http.StatusOK, createResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, createResp).Code)

	item := model.NewUserTodo()
	if err := db.Where("assignee_id = ? AND source_type = ? AND source_key = ?", 1, model.TodoSourceManual, sourceKey).
		First(item).Error; err != nil {
		t.Fatalf("查询已创建个人待办失败: %v", err)
	}
	assert.Equal(t, uint(1), item.CreatorID)

	listResp := getRequest("/admin/v1/todo/list", &url.Values{
		"keyword":  {"接口测试待办"},
		"priority": {model.TodoPriorityHigh},
		"page":     {"1"},
		"per_page": {"20"},
	})
	assert.Equal(t, http.StatusOK, listResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, listResp).Code)

	updatedTitle := "接口测试待办-已更新"
	updatePayload := mustJSONBody(t, map[string]any{"id": item.ID, "title": updatedTitle})
	updateResp := postRequest("/admin/v1/todo/update", &updatePayload)
	assert.Equal(t, http.StatusOK, updateResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, updateResp).Code)

	statusPayload := mustJSONBody(t, map[string]any{"id": item.ID, "status": model.TodoStatusCompleted})
	statusResp := postRequest("/admin/v1/todo/status", &statusPayload)
	assert.Equal(t, http.StatusOK, statusResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, statusResp).Code)

	if err := db.First(item, item.ID).Error; err != nil {
		t.Fatalf("查询已更新个人待办失败: %v", err)
	}
	assert.Equal(t, updatedTitle, item.Title)
	assert.Equal(t, model.TodoStatusCompleted, item.Status)
	assert.NotNil(t, item.CompletedAt)

	summaryResp := getRequest("/admin/v1/todo/summary", nil)
	assert.Equal(t, http.StatusOK, summaryResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, summaryResp).Code)

	deletePayload := mustJSONBody(t, map[string]any{"id": item.ID})
	deleteResp := postRequest("/admin/v1/todo/delete", &deletePayload)
	assert.Equal(t, http.StatusOK, deleteResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, deleteResp).Code)

	var activeCount int64
	if err := db.Model(&model.UserTodo{}).Where("id = ?", item.ID).Count(&activeCount).Error; err != nil {
		t.Fatalf("验证个人待办删除状态失败: %v", err)
	}
	assert.Zero(t, activeCount)
}

func TestTodoReminderPreferenceFlowWithConfiguredMySQL(t *testing.T) {
	requireWritableDB(t)
	db, err := model.GetDB()
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}
	cleanup := func() {
		if err := db.Where("user_id = ? AND scene = ?", 1, model.NotificationSceneTodoDue).
			Delete(&model.NotificationPreference{}).Error; err != nil {
			t.Errorf("清理待办提醒偏好失败: %v", err)
		}
	}
	cleanup()
	defer cleanup()

	getResp := getRequest("/admin/v1/todo/reminder-preference", nil)
	assert.Equal(t, http.StatusOK, getResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, getResp).Code)

	payload := mustJSONBody(t, map[string]any{"in_app_enabled": false, "sms_enabled": false})
	updateResp := postRequest("/admin/v1/todo/reminder-preference", &payload)
	assert.Equal(t, http.StatusOK, updateResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, updateResp).Code)

	var preference model.NotificationPreference
	if err := db.Where("user_id = ? AND scene = ?", 1, model.NotificationSceneTodoDue).First(&preference).Error; err != nil {
		t.Fatalf("回读待办提醒偏好失败: %v", err)
	}
	assert.Equal(t, uint8(0), preference.InAppEnabled)
	assert.Equal(t, uint8(0), preference.SMSEnabled)
}
