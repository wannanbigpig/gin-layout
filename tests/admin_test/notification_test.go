package admin_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestNotificationSendReadAndReadAllFlowWithConfiguredMySQL(t *testing.T) {
	requireWritableDB(t)

	db, err := model.GetDB()
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}
	category := uniqueCompactTestName("notice")
	defer func() {
		if err := db.Unscoped().Where("category = ? AND sender_id = ?", category, 1).Delete(&model.SysMessage{}).Error; err != nil {
			t.Errorf("清理通知测试数据失败: %v", err)
		}
	}()

	first := sendNotificationToSuperAdmin(t, category, "第一条接口测试通知")

	listResp := getRequest("/admin/v1/system/notification/list", &url.Values{
		"category": {category},
		"page":     {"1"},
		"per_page": {"20"},
	})
	assert.Equal(t, http.StatusOK, listResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, listResp).Code)

	unreadResp := getRequest("/admin/v1/system/notification/unread-count", nil)
	assert.Equal(t, http.StatusOK, unreadResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, unreadResp).Code)

	readPayload := mustJSONBody(t, map[string]any{"id": first.ID})
	readResp := postRequest("/admin/v1/system/notification/read", &readPayload)
	assert.Equal(t, http.StatusOK, readResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, readResp).Code)

	if err := db.First(first, first.ID).Error; err != nil {
		t.Fatalf("读取已读通知失败: %v", err)
	}
	assert.Equal(t, model.MessageRead, first.IsRead)

	second := sendNotificationToSuperAdmin(t, category, "第二条接口测试通知")
	readAllPayload := mustJSONBody(t, map[string]any{"category": category})
	readAllResp := postRequest("/admin/v1/system/notification/read-all", &readAllPayload)
	assert.Equal(t, http.StatusOK, readAllResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, readAllResp).Code)

	if err := db.First(second, second.ID).Error; err != nil {
		t.Fatalf("读取全部已读后的通知失败: %v", err)
	}
	assert.Equal(t, model.MessageRead, second.IsRead)
	assert.NotZero(t, second.ID)
}

func sendNotificationToSuperAdmin(t *testing.T, category, title string) *model.SysMessage {
	t.Helper()
	payload := mustJSONBody(t, map[string]any{
		"title":         title,
		"message":       "由真实 MySQL 接口测试创建",
		"category":      category,
		"audience_type": "user_ids",
		"user_ids":      []uint{1},
	})
	resp := postRequest("/admin/v1/system/notification/send", &payload)
	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.SUCCESS, result.Code)

	db, err := model.GetDB()
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}
	message := model.NewSysMessage()
	if err := db.Where("category = ? AND sender_id = ? AND title = ?", category, 1, title).
		Order("id DESC").First(message).Error; err != nil {
		t.Fatalf("查询已发送通知失败: %v", err)
	}
	return message
}
