package model

import "github.com/wannanbigpig/gin-layout/internal/pkg/utils"

const (
	MessageCategoryExport = "export"

	MessageTopicExportFinished = "export.finished"

	MessageUnread uint8 = 0
	MessageRead   uint8 = 1
)

// SysMessage 表示面向单个后台用户的系统消息。
type SysMessage struct {
	BaseModel
	UserID   uint              `json:"user_id"`
	Category string            `json:"category"`
	Topic    string            `json:"topic"`
	Title    string            `json:"title"`
	Content  string            `json:"content"`
	Payload  string            `json:"payload"`
	IsRead   uint8             `json:"is_read"`
	ReadAt   *utils.FormatDate `json:"read_at"`
}

func NewSysMessage() *SysMessage {
	return BindModel(&SysMessage{})
}

func (m *SysMessage) TableName() string {
	return "sys_messages"
}
