package notification

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestServiceCreateListReadAndReadAll(t *testing.T) {
	db := newMessageTestDB(t)
	var broadcasted []Event
	var published []Event

	svc := NewServiceWithDeps(ServiceDeps{
		DB: db,
		BroadcastFn: func(event Event) {
			broadcasted = append(broadcasted, event)
		},
		PublishFn: func(_ context.Context, event Event) error {
			published = append(published, event)
			return nil
		},
	})

	if _, err := svc.Create(context.Background(), CreateMessageInput{
		UserID:   7,
		Category: model.MessageCategoryExport,
		Topic:    model.MessageTopicExportFinished,
		Title:    "请求日志导出完成",
		Content:  "可下载",
		Payload:  map[string]any{"user_id": uint(7), "task_run_id": uint(101)},
	}); err != nil {
		t.Fatalf("create first message failed: %v", err)
	}
	if _, err := svc.Create(context.Background(), CreateMessageInput{
		UserID:   7,
		Category: model.MessageCategoryExport,
		Topic:    model.MessageTopicExportFinished,
		Title:    "文件资源导出失败",
		Content:  "失败",
		Payload:  map[string]any{"user_id": uint(7), "task_run_id": uint(102)},
	}); err != nil {
		t.Fatalf("create second message failed: %v", err)
	}
	if _, err := svc.Create(context.Background(), CreateMessageInput{
		UserID:   8,
		Category: model.MessageCategoryExport,
		Topic:    model.MessageTopicExportFinished,
		Title:    "其他用户消息",
		Content:  "ignore",
		Payload:  map[string]any{"user_id": uint(8)},
	}); err != nil {
		t.Fatalf("create third message failed: %v", err)
	}

	list, err := svc.List(7, &form.SysMessageList{Paginate: form.Paginate{Page: 1, PerPage: 10}})
	if err != nil {
		t.Fatalf("list messages failed: %v", err)
	}
	if list.Total != 2 || len(list.Data) != 2 {
		t.Fatalf("unexpected list result: %#v", list)
	}

	unreadCount, err := svc.UnreadCount(context.Background(), 7)
	if err != nil {
		t.Fatalf("unread count failed: %v", err)
	}
	if unreadCount != 2 {
		t.Fatalf("unexpected unread count: got=%d want=2", unreadCount)
	}

	firstItem, ok := list.Data[0].(*resources.SysMessageListResources)
	if !ok {
		t.Fatalf("unexpected list item type: %T", list.Data[0])
	}
	updated, unreadCount, err := svc.MarkRead(context.Background(), 7, firstItem.ID)
	if err != nil {
		t.Fatalf("mark read failed: %v", err)
	}
	if !updated || unreadCount != 1 {
		t.Fatalf("unexpected mark read result: updated=%v unread=%d", updated, unreadCount)
	}

	affected, unreadCount, err := svc.MarkAllRead(context.Background(), 7, model.MessageCategoryExport)
	if err != nil {
		t.Fatalf("mark all read failed: %v", err)
	}
	if affected != 1 || unreadCount != 0 {
		t.Fatalf("unexpected mark all read result: affected=%d unread=%d", affected, unreadCount)
	}

	if len(broadcasted) != 5 || len(published) != 5 {
		t.Fatalf("expected create/read/read-all events to dispatch, got broadcast=%d publish=%d", len(broadcasted), len(published))
	}
}

func TestServiceNotifyExportResultBuildsMessage(t *testing.T) {
	db := newMessageTestDB(t)
	svc := NewServiceWithDeps(ServiceDeps{
		DB:          db,
		BroadcastFn: func(Event) {},
		PublishFn:   func(context.Context, Event) error { return nil },
	})

	successMsg, err := svc.NotifyExportResult(context.Background(), ExportResultInput{
		UserID:    9,
		Scene:     model.ExportSceneRequestLog,
		Status:    model.TaskRunStatusSuccess,
		TaskRunID: 501,
		FileID:    11,
		FileUUID:  "file-uuid",
		FileName:  "request_logs.csv",
	})
	if err != nil {
		t.Fatalf("notify success failed: %v", err)
	}
	if successMsg.Title != "请求日志导出完成" {
		t.Fatalf("unexpected success title: %s", successMsg.Title)
	}

	failedMsg, err := svc.NotifyExportResult(context.Background(), ExportResultInput{
		UserID:       9,
		Scene:        model.ExportSceneFileList,
		Status:       model.TaskRunStatusFailed,
		TaskRunID:    502,
		ErrorMessage: "生成失败",
	})
	if err != nil {
		t.Fatalf("notify failed failed: %v", err)
	}
	if failedMsg.Title != "文件资源导出失败" {
		t.Fatalf("unexpected failure title: %s", failedMsg.Title)
	}

	var stored []model.SysMessage
	if err := db.Order("id asc").Find(&stored).Error; err != nil {
		t.Fatalf("load stored messages failed: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("unexpected stored messages count: %d", len(stored))
	}
	if stored[0].Payload == "" || stored[1].Payload == "" {
		t.Fatalf("expected payload to be stored: %#v", stored)
	}
}

func TestDecodeRealtimePayload(t *testing.T) {
	payload, ok := decodeRealtimePayload(`{"source":"node-1","event":{"event":"message_created","user_id":7,"unread_count":2}}`)
	if !ok {
		t.Fatal("expected payload to decode")
	}
	if payload.Source != "node-1" || payload.Event.UserID != 7 || payload.Event.UnreadCount != 2 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestDecodeRealtimePayloadRejectsInvalidJSON(t *testing.T) {
	if _, ok := decodeRealtimePayload("{invalid"); ok {
		t.Fatal("expected invalid payload to be rejected")
	}
}

func newMessageTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.Exec(`CREATE TABLE sys_messages (
		id integer primary key autoincrement,
		user_id integer,
		sender_id integer,
		batch_id text,
		category text,
		topic text,
		title text,
		content text,
		payload text,
		action_url text,
		action_label text,
		recipient_type text,
		recipient_ids text,
		send_immediately integer,
		send_at datetime,
		sent_at datetime,
		is_read integer,
		read_at datetime,
		created_at datetime,
		updated_at datetime
	)`).Error; err != nil {
		t.Fatalf("create sys_messages table failed: %v", err)
	}
	return db
}
