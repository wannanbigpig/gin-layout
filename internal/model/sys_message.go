package model

import "github.com/wannanbigpig/gin-layout/internal/pkg/utils"

const (
	MessageCategoryExport = "export"

	MessageTopicExportFinished = "export.finished"
	MessageTopicManual         = "manual.notification"

	MessageRecipientTypeAll  = "all"
	MessageRecipientTypeUser = "user"

	MessageUnread uint8 = 0
	MessageRead   uint8 = 1

	MessageSendScheduled uint8 = 0
	MessageSendNow       uint8 = 1
)

// SysMessage 表示面向单个后台用户的系统消息。
type SysMessage struct {
	BaseModel
	UserID          uint              `json:"user_id"`
	SenderID        uint              `json:"sender_id"`
	BatchID         string            `json:"batch_id"`
	Category        string            `json:"category"`
	Topic           string            `json:"topic"`
	Title           string            `json:"title"`
	Content         string            `json:"content"`
	Payload         string            `json:"payload"`
	ActionURL       string            `json:"action_url"`
	ActionLabel     string            `json:"action_label"`
	RecipientType   string            `json:"recipient_type"`
	RecipientIDs    string            `json:"recipient_ids"`
	SendImmediately uint8             `json:"send_immediately"`
	SendAt          *utils.FormatDate `json:"send_at"`
	SentAt          *utils.FormatDate `json:"sent_at"`
	IsRead          uint8             `json:"is_read"`
	ReadAt          *utils.FormatDate `json:"read_at"`
}

func NewSysMessage() *SysMessage {
	return BindModel(&SysMessage{})
}

func (m *SysMessage) TableName() string {
	return "sys_messages"
}
