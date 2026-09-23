package notification

import (
	"context"
	"strings"
	"sync"
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
	var captureMu sync.Mutex

	svc := NewServiceWithDeps(ServiceDeps{
		DB: db,
		BroadcastFn: func(event Event) {
			captureMu.Lock()
			broadcasted = append(broadcasted, event)
			captureMu.Unlock()
		},
		PublishFn: func(_ context.Context, event Event) error {
			captureMu.Lock()
			published = append(published, event)
			captureMu.Unlock()
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

	list, err := svc.List(context.Background(), 7, &form.SysMessageList{Paginate: form.Paginate{Page: 1, PerPage: 10}})
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

	captureMu.Lock()
	bcLen := len(broadcasted)
	pubLen := len(published)
	captureMu.Unlock()
	if bcLen != 5 || pubLen != 5 {
		t.Fatalf("expected create/read/read-all events to dispatch, got broadcast=%d publish=%d", bcLen, pubLen)
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
		PublicID:  "export-501",
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
	if successMsg.ActionURL != "/my-exports" || strings.Contains(successMsg.Payload, "task_run_id") || strings.Contains(successMsg.Payload, "file_id") {
		t.Fatalf("export notification exposed diagnostics: %#v", successMsg)
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
	if strings.Contains(failedMsg.Content, "生成失败") || strings.Contains(failedMsg.Payload, "生成失败") {
		t.Fatalf("raw execution error leaked to user notification: %#v", failedMsg)
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

func TestServicePublishExportProgressDispatchesEphemeralEvent(t *testing.T) {
	var broadcasted []Event
	var published []Event
	svc := NewServiceWithDeps(ServiceDeps{
		BroadcastFn: func(event Event) { broadcasted = append(broadcasted, event) },
		PublishFn: func(_ context.Context, event Event) error {
			published = append(published, event)
			return nil
		},
	})

	svc.PublishExportProgress(context.Background(), ExportProgressInput{
		UserID:         7,
		Scene:          model.ExportSceneRequestLog,
		PublicID:       "export-101",
		Status:         model.TaskRunStatusRunning,
		Progress:       40,
		ProcessedCount: 20,
		TotalCount:     50,
	})

	if len(broadcasted) != 1 || len(published) != 1 {
		t.Fatalf("expected one realtime event, got broadcast=%d publish=%d", len(broadcasted), len(published))
	}
	event := broadcasted[0]
	if event.Event != "export_progress" || event.Topic != model.MessageTopicExportProgress || event.UserID != 7 {
		t.Fatalf("unexpected export progress event: %#v", event)
	}
	if event.Data["public_id"] != "export-101" || event.Data["progress"] != uint8(40) {
		t.Fatalf("unexpected export progress payload: %#v", event.Data)
	}
}

func TestNotifyExportResultIsIdempotentPerTaskStatus(t *testing.T) {
	db := newMessageTestDB(t)
	dispatched := 0
	svc := NewServiceWithDeps(ServiceDeps{
		DB: db,
		BroadcastFn: func(Event) {
			dispatched++
		},
	})
	input := ExportResultInput{
		UserID:    7,
		Scene:     model.ExportSceneRequestLog,
		Status:    model.TaskRunStatusSuccess,
		TaskRunID: 101,
		FileID:    9,
		FileUUID:  "file-9",
		FileName:  "request_logs.csv",
	}
	first, err := svc.NotifyExportResult(context.Background(), input)
	if err != nil {
		t.Fatalf("first export notification failed: %v", err)
	}
	second, err := svc.NotifyExportResult(context.Background(), input)
	if err != nil {
		t.Fatalf("duplicate export notification failed: %v", err)
	}
	if first.ID == 0 || second.ID != first.ID {
		t.Fatalf("duplicate notification should reuse one row: first=%d second=%d", first.ID, second.ID)
	}
	var count int64
	if err := db.Model(&model.SysMessage{}).Count(&count).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if count != 1 || dispatched != 1 {
		t.Fatalf("expected one row and one dispatch, rows=%d dispatched=%d", count, dispatched)
	}
}

func TestNotificationActionURLMustBeInternalPath(t *testing.T) {
	db := newMessageTestDB(t)
	svc := NewServiceWithDeps(ServiceDeps{DB: db, BroadcastFn: func(Event) {}})
	for _, actionURL := range []string{"https://example.com", "//example.com", `\\example.com`, "javascript:alert(1)", "/safe\npath"} {
		if _, err := svc.Create(context.Background(), CreateMessageInput{UserID: 7, Title: "unsafe", ActionURL: actionURL}); err == nil {
			t.Fatalf("expected unsafe action URL %q to fail", actionURL)
		}
	}
	if _, err := svc.Create(context.Background(), CreateMessageInput{UserID: 7, Title: "safe", ActionURL: "/todo/list?scope=overdue"}); err != nil {
		t.Fatalf("expected internal action URL to pass: %v", err)
	}
}

func TestManualNotificationCannotForgeTodoReminderCategory(t *testing.T) {
	db := newMessageTestDB(t)
	svc := NewServiceWithDeps(ServiceDeps{DB: db, BroadcastFn: func(Event) {}})
	if _, err := svc.Send(context.Background(), SendInput{
		Title: "manual", Content: "manual", Category: model.MessageCategoryTodo, AudienceType: "user_ids", UserIDs: []uint{7},
	}); err == nil {
		t.Fatal("manual notification must not use the reserved todo category")
	}
}

func TestServiceSendUsesInjectedDBForRecipients(t *testing.T) {
	db := newMessageTestDB(t)
	if err := db.Exec(`INSERT INTO admin_user (id, username, status, deleted_at) VALUES
		(7, 'enabled', 1, 0),
		(8, 'disabled', 0, 0)`).Error; err != nil {
		t.Fatalf("seed admin_user failed: %v", err)
	}

	var broadcasted []Event
	svc := NewServiceWithDeps(ServiceDeps{
		DB: db,
		BroadcastFn: func(event Event) {
			broadcasted = append(broadcasted, event)
		},
		PublishFn: func(context.Context, Event) error { return nil },
	})

	result, err := svc.Send(context.Background(), SendInput{
		Title:        "系统通知",
		Content:      "请查看",
		AudienceType: "user_ids",
		UserIDs:      []uint{7, 8, 7},
		SenderID:     1,
	})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if result.Total != 1 || result.Success != 1 {
		t.Fatalf("unexpected send result: %#v", result)
	}

	var messages []model.SysMessage
	if err := db.Find(&messages).Error; err != nil {
		t.Fatalf("query sys_messages failed: %v", err)
	}
	if len(messages) != 1 || messages[0].UserID != 7 {
		t.Fatalf("expected one message for enabled user 7, got %#v", messages)
	}
	if len(broadcasted) != 1 || broadcasted[0].UserID != 7 {
		t.Fatalf("expected one broadcast for user 7, got %#v", broadcasted)
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
		batch_id text not null,
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
		updated_at datetime,
		unique(user_id, batch_id)
	)`).Error; err != nil {
		t.Fatalf("create sys_messages table failed: %v", err)
	}
	if err := db.Exec(`CREATE TABLE admin_user (
		id integer primary key autoincrement,
		username text NOT NULL DEFAULT '',
		status integer NOT NULL DEFAULT 1,
		deleted_at integer NOT NULL DEFAULT 0
	)`).Error; err != nil {
		t.Fatalf("create admin_user table failed: %v", err)
	}
	return db
}

func TestServiceListWithCanceledContext(t *testing.T) {
	db := newMessageTestDB(t)
	svc := NewServiceWithDeps(ServiceDeps{DB: db})
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 预先取消

	_, err := svc.List(ctx, 1, &form.SysMessageList{})
	if err == nil {
		t.Fatal("expected error on canceled context, got nil")
	}
}
