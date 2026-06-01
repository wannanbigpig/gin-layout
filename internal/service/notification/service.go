package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

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
	Scene          string
	Status         string
	ExportRecordID uint
	TaskRunID      uint
	FileID         uint
	FileUUID       string
	FileName       string
	ErrorMessage   string
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
	if input.UserID == 0 {
		return nil, fmt.Errorf("message user id is required")
	}
	if strings.TrimSpace(input.Title) == "" {
		return nil, fmt.Errorf("message title is required")
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	if err := ensureMessageTable(db); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(input.Payload)
	if err != nil {
		return nil, err
	}

	message := model.NewSysMessage()
	message.UserID = input.UserID
	message.Category = strings.TrimSpace(input.Category)
	message.Topic = strings.TrimSpace(input.Topic)
	message.Title = strings.TrimSpace(input.Title)
	message.Content = strings.TrimSpace(input.Content)
	message.ActionURL = strings.TrimSpace(input.ActionURL)
	message.ActionLabel = strings.TrimSpace(input.ActionLabel)
	message.Payload = string(payload)
	message.IsRead = model.MessageUnread

	if err := db.Create(message).Error; err != nil {
		return nil, err
	}

	unreadCount, err := s.UnreadCount(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	event := Event{
		Event:       "message_created",
		UserID:      input.UserID,
		Message:     resources.NewSysMessageTransformer().ToStruct(message),
		MessageID:   message.ID,
		Category:    message.Category,
		UnreadCount: unreadCount,
	}
	s.dispatch(ctx, event)

	return message, nil
}

func (s *Service) NotifyExportResult(ctx context.Context, input ExportResultInput) (*model.SysMessage, error) {
	title, content := buildExportMessageText(input.Scene, input.Status, input.ErrorMessage)
	return s.Create(ctx, CreateMessageInput{
		UserID:   input.UserID,
		Category: model.MessageCategoryExport,
		Topic:    model.MessageTopicExportFinished,
		Title:    title,
		Content:  content,
		Payload: map[string]any{
			"user_id":          input.UserID,
			"scene":            strings.TrimSpace(input.Scene),
			"status":           strings.TrimSpace(input.Status),
			"export_record_id": input.ExportRecordID,
			"task_run_id":      input.TaskRunID,
			"file_id":          input.FileID,
			"file_uuid":        strings.TrimSpace(input.FileUUID),
			"file_name":        strings.TrimSpace(input.FileName),
			"download_url":     buildExportDownloadURL(strings.TrimSpace(input.FileUUID)),
			"error_message":    strings.TrimSpace(input.ErrorMessage),
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

	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	if err := ensureMessageTable(db); err != nil {
		return nil, err
	}

	// 确定接收人列表
	var userIDs []uint
	switch input.AudienceType {
	case "all":
		userModel := model.NewAdminUsers()
		userDB, err := userModel.GetDB(userModel)
		if err != nil {
			return nil, err
		}
		if err := userDB.Where("status = ? AND deleted_at = 0", model.AdminUserStatusEnabled).Pluck("id", &userIDs).Error; err != nil {
			return nil, err
		}
	case "user_ids":
		userIDs = input.UserIDs
	default:
		return nil, fmt.Errorf("invalid audience_type: %s", input.AudienceType)
	}

	batchID := uuid.New().String()
	payload := map[string]any{}
	if strings.TrimSpace(input.ActionURL) != "" {
		payload["action_url"] = strings.TrimSpace(input.ActionURL)
	}
	if strings.TrimSpace(input.ActionLabel) != "" {
		payload["action_label"] = strings.TrimSpace(input.ActionLabel)
	}

	result := &SendResult{BatchID: batchID, Total: len(userIDs)}
	for _, uid := range userIDs {
		msg, err := s.Create(ctx, CreateMessageInput{
			UserID:      uid,
			Category:    strings.TrimSpace(input.Category),
			Topic:       model.MessageTopicManual,
			Title:       input.Title,
			Content:     input.Content,
			ActionURL:   input.ActionURL,
			ActionLabel: input.ActionLabel,
			Payload:     payload,
		})
		if err != nil || msg == nil {
			result.Failed++
			if log.Logger != nil {
				log.Logger.Warn("send notification to user failed",
					zap.Uint("user_id", uid), zap.String("batch_id", batchID), zap.Error(err))
			}
			continue
		}
		// 更新 sender_id 和 batch_id
		updates := map[string]any{"sender_id": input.SenderID, "batch_id": batchID}
		if err := db.Model(&model.SysMessage{}).Where("id = ?", msg.ID).Updates(updates).Error; err != nil {
			if log.Logger != nil {
				log.Logger.Warn("更新通知 sender_id/batch_id 失败",
					zap.Uint("message_id", msg.ID), zap.Error(err))
			}
		}
		result.Success++
	}

	return result, nil
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
		"payload", "is_read", "read_at", "created_at", "updated_at",
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

func buildExportMessageText(scene, status, errorMessage string) (string, string) {
	sceneName := exportSceneName(scene)
	switch strings.TrimSpace(status) {
	case model.TaskRunStatusSuccess:
		return sceneName + "导出完成", sceneName + "导出已完成，可前往任务中心下载。"
	case model.TaskRunStatusFailed:
		reason := strings.TrimSpace(errorMessage)
		if reason == "" {
			reason = "任务执行失败"
		}
		return sceneName + "导出失败", sceneName + "导出失败：" + reason
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

func buildExportDownloadURL(fileUUID string) string {
	if strings.TrimSpace(fileUUID) == "" {
		return ""
	}
	return "/admin/v1/file/" + strings.TrimSpace(fileUUID)
}
