package model

import "github.com/wannanbigpig/gin-layout/internal/pkg/utils"

const (
	TodoSourceManual = "manual"

	TodoPriorityLow    = "low"
	TodoPriorityNormal = "normal"
	TodoPriorityHigh   = "high"
	TodoPriorityUrgent = "urgent"

	TodoStatusPending    = "pending"
	TodoStatusProcessing = "processing"
	TodoStatusCompleted  = "completed"
	TodoStatusCanceled   = "canceled"
)

// UserTodo 表示分配给单个后台用户的待办事项。
type UserTodo struct {
	ContainsDeleteBaseModel
	AssigneeID  uint              `json:"assignee_id" gorm:"column:assignee_id;not null;default:0"`
	CreatorID   uint              `json:"creator_id" gorm:"column:creator_id;not null;default:0"`
	SourceType  string            `json:"source_type" gorm:"column:source_type;type:varchar(40);not null;default:''"`
	SourceKey   string            `json:"source_key" gorm:"column:source_key;type:varchar(128);not null;default:''"`
	Title       string            `json:"title" gorm:"column:title;type:varchar(255);not null;default:''"`
	Description string            `json:"description" gorm:"column:description;type:text"`
	Priority    string            `json:"priority" gorm:"column:priority;type:varchar(20);not null;default:'normal'"`
	Status      string            `json:"status" gorm:"column:status;type:varchar(20);not null;default:'pending'"`
	ActionURL   string            `json:"action_url" gorm:"column:action_url;type:varchar(500);not null;default:''"`
	DueAt       *utils.FormatDate `json:"due_at" gorm:"column:due_at;type:datetime"`
	CompletedAt *utils.FormatDate `json:"completed_at" gorm:"column:completed_at;type:datetime"`
	CanceledAt  *utils.FormatDate `json:"canceled_at" gorm:"column:canceled_at;type:datetime"`
}

func NewUserTodo() *UserTodo {
	return BindModel(&UserTodo{})
}

func (m *UserTodo) TableName() string {
	return "user_todos"
}
