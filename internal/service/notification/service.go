package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"uuid"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

var ErrMessageTableMissing = errors.New("sys_messages table is missing")

type Event struct {
	Event       string                         `json:"event"`
	UserID      uint                           `json:"user_id"`
	Message     *resources.SysMessageResources `json:"message,omitempty"`
	MessageID   uint                           `json:"message_id,omitempty"`
	Category    string                         `json:"category,omitempty"`
	Topic       string                         `json:"topic,omitempty"`
	Data        map[string]any                 `json:"data,omitempty"`
	UnreadCount int64                          `json:"unread_count"`
}

type ServiceDeps struct {
	DB          *gorm.DB
	BroadcastFn func(Event)
	PublishFn   func(context.Context, Event) error
}

type Service struct {
	db          *gorm.DB
	broadcastFn func(Event)
	publishFn   func(context.Context, Event) error
}

type CreateMessageInput struct {
	UserID      uint
	Category    string
	Topic       string
	Title       string
	Content     string
	ActionURL   string
	ActionLabel string
	Payload     map[string]any
	BatchID     string
}

// SendInput 表示管理员手动发送通知的输入参数。
type SendInput struct {
	Title        string
	Content      string
	Category     string
	ActionURL    string
	ActionLabel  string
	AudienceType string
	UserIDs      []uint
	SenderID     uint
}

// SendResult 表示发送通知的结果。
type SendResult struct {
	BatchID string `json:"batch_id"`
	Total   int    `json:"total"`
	Success int    `json:"success"`
	Failed  int    `json:"failed"`
}

type ExportResultInput struct {
	UserID         uint
	PublicID       string
	Scene          string
	Status         string
	ExportRecordID uint
	TaskRunID      uint
	FileID         uint
	FileUUID       string
	FileName       string
	ErrorMessage   string
}

// ExportProgressInput 表示只用于实时展示的导出进度，不写入站内消息表。
type ExportProgressInput struct {
	UserID         uint
	Scene          string
	PublicID       string
	Status         string
	Progress       uint8
	ProcessedCount uint64
	TotalCount     uint64
}

func NewService() *Service {
	return NewServiceWithDeps(ServiceDeps{})
}

func NewServiceWithDeps(deps ServiceDeps) *Service {
	svc := &Service{
		db:          deps.DB,
		broadcastFn: deps.BroadcastFn,
		publishFn:   deps.PublishFn,
	}
	if svc.broadcastFn == nil {
		svc.broadcastFn = BroadcastEvent
	}
	if svc.publishFn == nil {
		svc.publishFn = PublishEvent
	}
	return svc
}

func (s *Service) Create(ctx context.Context, input CreateMessageInput) (*model.SysMessage, error) {
	message, created, err := s.Persist(ctx, input)
	if err != nil || !created {
		return message, err
	}
	if err := s.DispatchCreated(ctx, message); err != nil {
		return nil, err
	}
	return message, nil
}

// Persist 持久化一条通知但不发送实时事件。调用方可把它放在业务事务中，
// 并在事务成功提交后调用 DispatchCreated，避免业务状态和消息记录分叉。
func (s *Service) Persist(ctx context.Context, input CreateMessageInput) (*model.SysMessage, bool, error) {
	if input.UserID == 0 {
		return nil, false, fmt.Errorf("message user id is required")
	}
	if strings.TrimSpace(input.Title) == "" {
		return nil, false, fmt.Errorf("message title is required")
	}
	actionURL, err := NormalizeActionURL(input.ActionURL)
	if err != nil {
		return nil, false, err
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, false, err
	}
	if err := ensureMessageTable(db); err != nil {
		return nil, false, err
	}

	payload, err := json.Marshal(input.Payload)
	if err != nil {
		return nil, false, err
	}

	message := model.NewSysMessage()
	message.UserID = input.UserID
	message.BatchID = strings.TrimSpace(input.BatchID)
	if message.BatchID == "" {
		message.BatchID = uuid.New().String()
	}
	message.Category = strings.TrimSpace(input.Category)
	message.Topic = strings.TrimSpace(input.Topic)
	message.Title = strings.TrimSpace(input.Title)
	message.Content = strings.TrimSpace(input.Content)
	message.ActionURL = actionURL
	message.ActionLabel = strings.TrimSpace(input.ActionLabel)
	message.Payload = string(payload)
	message.IsRead = model.MessageUnread

	result := db.Clauses(clause.OnConflict{DoNothing: true}).Create(message)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		existing := model.NewSysMessage()
		if err := db.Where("user_id = ? AND batch_id = ?", message.UserID, message.BatchID).First(existing).Error; err != nil {
			return nil, false, err
		}
		return existing, false, nil
	}
	return message, true, nil
}

// DispatchCreated 在消息事务提交后广播实时事件；持久消息仍是最终交付凭据。
func (s *Service) DispatchCreated(ctx context.Context, message *model.SysMessage) error {
	if message == nil || message.ID == 0 {
		return nil
	}
	unreadCount, err := s.UnreadCount(ctx, message.UserID)
	if err != nil {
		return err
	}
	event := Event{
		Event:       "message_created",
		UserID:      message.UserID,
		Message:     resources.NewSysMessageTransformer().ToStruct(message),
		MessageID:   message.ID,
		Category:    message.Category,
		UnreadCount: unreadCount,
	}
	s.dispatch(ctx, event)
	return nil
}

func (s *Service) NotifyExportResult(ctx context.Context, input ExportResultInput) (*model.SysMessage, error) {
	title, content := buildExportMessageText(input.Scene, input.Status)
	batchID := ""
	if input.TaskRunID > 0 {
		batchID = fmt.Sprintf("export:%d:%s", input.TaskRunID, strings.TrimSpace(input.Status))
	}
	return s.Create(ctx, CreateMessageInput{
		UserID:      input.UserID,
		Category:    model.MessageCategoryExport,
		Topic:       model.MessageTopicExportFinished,
		Title:       title,
		Content:     content,
		ActionURL:   "/my-exports",
		ActionLabel: "查看我的导出",
		BatchID:     batchID,
		Payload: map[string]any{
			"public_id": strings.TrimSpace(input.PublicID),
			"scene":     strings.TrimSpace(input.Scene),
			"status":    strings.TrimSpace(input.Status),
		},
	})
}

// PublishExportProgress 发送轻量实时进度。数据库中的导出记录仍是最终事实来源，
// WebSocket/Redis 不可用时由前端低频轮询兜底，因此推送失败不应中断导出任务。
func (s *Service) PublishExportProgress(ctx context.Context, input ExportProgressInput) {
	if input.UserID == 0 || strings.TrimSpace(input.PublicID) == "" {
		return
	}
	s.dispatch(ctx, Event{
		Event:    "export_progress",
		UserID:   input.UserID,
		Category: model.MessageCategoryExport,
		Topic:    model.MessageTopicExportProgress,
		Data: map[string]any{
			"scene":           strings.TrimSpace(input.Scene),
			"public_id":       strings.TrimSpace(input.PublicID),
			"status":          strings.TrimSpace(input.Status),
			"progress":        input.Progress,
			"processed_count": input.ProcessedCount,
			"total_count":     input.TotalCount,
			"stage":           "processing",
		},
	})
}

// Send 管理员手动发送系统通知，支持全员或指定用户。
func (s *Service) Send(ctx context.Context, input SendInput) (*SendResult, error) {
	if strings.TrimSpace(input.Title) == "" {
		return nil, fmt.Errorf("message title is required")
	}
	if strings.TrimSpace(input.Content) == "" {
		return nil, fmt.Errorf("message content is required")
	}
	actionURL, err := NormalizeActionURL(input.ActionURL)
	if err != nil {
		return nil, err
	}
	input.ActionURL = actionURL
	input.Category = strings.TrimSpace(input.Category)
	if input.Category == "" {
		input.Category = model.MessageCategorySystem
	}
	if input.Category == model.MessageCategoryTodo {
		return nil, fmt.Errorf("message category %q is reserved for todo reminders", input.Category)
	}

	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	if err := ensureMessageTable(db); err != nil {
		return nil, err
	}

	// 确定接收人列表：去重并过滤为真实存在且启用的用户，避免向无效/禁用账号发消息
	userIDs, err := resolveRecipients(db, input.AudienceType, input.UserIDs)
	if err != nil {
		return nil, err
	}

	batchID := uuid.New().String()
	result := &SendResult{BatchID: batchID, Total: len(userIDs)}
	if len(userIDs) == 0 {
		return result, nil
	}

	payload, err := json.Marshal(buildSendPayload(input))
	if err != nil {
		return nil, err
	}

	// 一次性构建并批量插入，sender_id/batch_id 在插入时即写入，避免逐条 insert + update 的 N+1
	messages := make([]*model.SysMessage, 0, len(userIDs))
	for _, uid := range userIDs {
		msg := model.NewSysMessage()
		msg.UserID = uid
		msg.SenderID = input.SenderID
		msg.BatchID = batchID
		msg.Category = strings.TrimSpace(input.Category)
		msg.Topic = model.MessageTopicManual
		msg.Title = strings.TrimSpace(input.Title)
		msg.Content = strings.TrimSpace(input.Content)
		msg.ActionURL = strings.TrimSpace(input.ActionURL)
		msg.ActionLabel = strings.TrimSpace(input.ActionLabel)
		msg.Payload = string(payload)
		msg.IsRead = model.MessageUnread
		messages = append(messages, msg)
	}
	if err := db.CreateInBatches(messages, 200).Error; err != nil {
		return nil, err
	}
	result.Success = len(messages)

	// 一次分组统计各接收人未读数，再逐个分发实时事件（广播/Redis publish 为内存或轻量操作）
	unreadCounts := s.unreadCountsByUsers(db, userIDs)
	transformer := resources.NewSysMessageTransformer()
	for _, msg := range messages {
		s.dispatch(ctx, Event{
			Event:       "message_created",
			UserID:      msg.UserID,
			Message:     transformer.ToStruct(msg),
			MessageID:   msg.ID,
			Category:    msg.Category,
			UnreadCount: unreadCounts[msg.UserID],
		})
	}

	return result, nil
}

// resolveRecipients 根据接收人类型返回去重后的有效接收人 ID 列表。
func resolveRecipients(db *gorm.DB, audienceType string, requested []uint) ([]uint, error) {
	if db == nil {
		return nil, model.ErrDBUninitialized
	}
	userDB := db.Model(&model.AdminUser{})
	var userIDs []uint
	switch audienceType {
	case "all":
		if err := userDB.Where("status = ? AND deleted_at = 0", model.AdminUserStatusEnabled).Pluck("id", &userIDs).Error; err != nil {
			return nil, err
		}
	case "user_ids":
		ids := dedupIDs(requested)
		if len(ids) == 0 {
			return nil, nil
		}
		if err := userDB.Where("id IN ? AND status = ? AND deleted_at = 0", ids, model.AdminUserStatusEnabled).Pluck("id", &userIDs).Error; err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid audience_type: %s", audienceType)
	}
	return userIDs, nil
}

// buildSendPayload 构建手动通知的 payload，仅包含非空的操作字段。
func buildSendPayload(input SendInput) map[string]any {
	payload := map[string]any{}
	if v := strings.TrimSpace(input.ActionURL); v != "" {
		payload["action_url"] = v
	}
	if v := strings.TrimSpace(input.ActionLabel); v != "" {
		payload["action_label"] = v
	}
	return payload
}

// dedupIDs 去重并剔除 0 值 ID。
func dedupIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// NormalizeActionURL 只接受站内相对路径，并返回可安全存储的标准值。
func NormalizeActionURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.Contains(value, `\`) || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", e.NewBusinessError(e.InvalidParameter, "action_url 仅支持站内相对路径")
	}
	return value, nil
}

// unreadCountsByUsers 一次查询批量统计指定用户的未读消息数。
func (s *Service) unreadCountsByUsers(db *gorm.DB, userIDs []uint) map[uint]int64 {
	counts := make(map[uint]int64, len(userIDs))
	if len(userIDs) == 0 {
		return counts
	}
	type row struct {
		UserID uint
		Cnt    int64
	}
	var rows []row
	if err := db.Model(&model.SysMessage{}).
		Select("user_id, COUNT(*) AS cnt").
		Where("user_id IN ? AND is_read = ?", userIDs, model.MessageUnread).
		Group("user_id").
		Scan(&rows).Error; err != nil {
		log.Warn("query unread message counts failed", zap.Error(err))
		return counts
	}
	for _, r := range rows {
		counts[r.UserID] = r.Cnt
	}
	return counts
}

func (s *Service) List(userID uint, params *form.SysMessageList) (*resources.Collection, error) {
	if params == nil {
		params = form.NewSysMessageListQuery()
	}
	db, err := s.dbOrDefault(context.Background())
	if err != nil {
		return nil, err
	}
	if err := ensureMessageTable(db); err != nil {
		return nil, err
	}
	query := query_builder.New().
		AddEq("user_id", userID).
		AddEq("category", strings.TrimSpace(params.Category))
	if params.IsRead != nil {
		query.AddEq("is_read", *params.IsRead)
	}
	condition, args := query.Build()
	normalizedPage := params.Page
	if normalizedPage < 1 {
		normalizedPage = 1
	}
	normalizedPerPage := params.PerPage
	if normalizedPerPage < 1 {
		normalizedPerPage = 10
	}

	base := db.Model(&model.SysMessage{})
	if strings.TrimSpace(condition) != "" {
		base = base.Where(condition, args...)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		log.Logger.Warn("count sys messages failed", zap.Uint("user_id", userID), zap.Error(err))
		return nil, err
	}

	list := make([]*model.SysMessage, 0)
	err = base.Select([]string{
		"id", "user_id", "category", "topic", "title", "content",
		"payload", "action_url", "action_label", "is_read", "read_at", "created_at", "updated_at",
	}).Order("id desc").Offset((normalizedPage - 1) * normalizedPerPage).Limit(normalizedPerPage).Find(&list).Error
	if err != nil {
		log.Logger.Warn("list sys messages failed", zap.Uint("user_id", userID), zap.Error(err))
		return nil, err
	}
	return resources.NewSysMessageTransformer().ToCollection(normalizedPage, normalizedPerPage, total, list), nil
}

func (s *Service) UnreadCount(ctx context.Context, userID uint) (int64, error) {
	if userID == 0 {
		return 0, nil
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return 0, err
	}
	if err := ensureMessageTable(db); err != nil {
		return 0, err
	}
	var count int64
	err = db.Model(&model.SysMessage{}).Where("user_id = ? AND is_read = ?", userID, model.MessageUnread).Count(&count).Error
	return count, err
}

func (s *Service) MarkRead(ctx context.Context, userID, id uint) (bool, int64, error) {
	if userID == 0 || id == 0 {
		return false, 0, nil
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return false, 0, err
	}
	if err := ensureMessageTable(db); err != nil {
		return false, 0, err
	}
	now := utils.FormatDate{Time: time.Now()}
	result := db.Model(&model.SysMessage{}).
		Where("id = ? AND user_id = ? AND is_read = ?", id, userID, model.MessageUnread).
		Updates(map[string]any{"is_read": model.MessageRead, "read_at": &now})
	if result.Error != nil {
		return false, 0, result.Error
	}
	unreadCount, err := s.UnreadCount(ctx, userID)
	if err != nil {
		return result.RowsAffected > 0, 0, err
	}
	if result.RowsAffected > 0 {
		s.dispatch(ctx, Event{
			Event:       "message_read",
			UserID:      userID,
			MessageID:   id,
			UnreadCount: unreadCount,
		})
	}
	return result.RowsAffected > 0, unreadCount, nil
}

func (s *Service) MarkAllRead(ctx context.Context, userID uint, category string) (int64, int64, error) {
	if userID == 0 {
		return 0, 0, nil
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return 0, 0, err
	}
	if err := ensureMessageTable(db); err != nil {
		return 0, 0, err
	}
	now := utils.FormatDate{Time: time.Now()}
	query := db.Model(&model.SysMessage{}).Where("user_id = ? AND is_read = ?", userID, model.MessageUnread)
	if strings.TrimSpace(category) != "" {
		query = query.Where("category = ?", strings.TrimSpace(category))
	}
	result := query.Updates(map[string]any{"is_read": model.MessageRead, "read_at": &now})
	if result.Error != nil {
		return 0, 0, result.Error
	}
	unreadCount, err := s.UnreadCount(ctx, userID)
	if err != nil {
		return result.RowsAffected, 0, err
	}
	if result.RowsAffected > 0 {
		s.dispatch(ctx, Event{
			Event:       "message_read_all",
			UserID:      userID,
			Category:    strings.TrimSpace(category),
			UnreadCount: unreadCount,
		})
	}
	return result.RowsAffected, unreadCount, nil
}

func (s *Service) dbOrDefault(ctx context.Context) (*gorm.DB, error) {
	if s.db != nil {
		if ctx != nil {
			return s.db.WithContext(ctx), nil
		}
		return s.db, nil
	}
	db, err := model.GetDB()
	if err != nil {
		return nil, err
	}
	if ctx != nil {
		return db.WithContext(ctx), nil
	}
	return db, nil
}

func (s *Service) dispatch(ctx context.Context, event Event) {
	if s.broadcastFn != nil {
		s.broadcastFn(event)
	}
	if s.publishFn == nil {
		return
	}
	if err := s.publishFn(ctx, event); err != nil && log.Logger != nil {
		log.Logger.Warn("publish message realtime event failed", zap.Error(err), zap.String("event", event.Event))
	}
}

func ensureMessageTable(db *gorm.DB) error {
	if db == nil {
		return model.ErrDBUninitialized
	}
	if db.Migrator().HasTable(&model.SysMessage{}) {
		return nil
	}
	return e.NewDependencyNotReadyError(ErrMessageTableMissing.Error())
}

func buildExportMessageText(scene, status string) (string, string) {
	sceneName := exportSceneName(scene)
	switch strings.TrimSpace(status) {
	case model.TaskRunStatusSuccess:
		return sceneName + "导出完成", sceneName + "导出已完成，可前往我的导出下载。"
	case model.TaskRunStatusFailed:
		return sceneName + "导出失败", sceneName + "导出失败，请重试；如持续失败请联系管理员。"
	default:
		return sceneName + "导出状态更新", sceneName + "导出状态已更新。"
	}
}

func exportSceneName(scene string) string {
	switch strings.TrimSpace(scene) {
	case model.ExportSceneRequestLog:
		return "请求日志"
	case model.ExportSceneFileList:
		return "文件资源"
	default:
		return "导出任务"
	}
}
