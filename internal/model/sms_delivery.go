package model

import "github.com/wannanbigpig/gin-layout/internal/pkg/utils"

const (
	SMSDeliveryStatusPending   = "pending"
	SMSDeliveryStatusSending   = "sending"
	SMSDeliveryStatusRetryWait = "retry_wait"
	SMSDeliveryStatusAccepted  = "accepted"
	SMSDeliveryStatusFailed    = "failed"
	SMSDeliveryStatusUnknown   = "unknown"
	SMSDeliveryStatusCanceled  = "canceled"
)

// SMSDelivery 既是待发 Outbox，也是计费短信的最终状态记录。
// unknown 表示供应商可能已接受，不得自动重试。
type SMSDelivery struct {
	BaseModel
	IdempotencyKey    string            `json:"idempotency_key" gorm:"column:idempotency_key;type:varchar(160);not null;uniqueIndex"`
	Scene             string            `json:"scene" gorm:"column:scene;type:varchar(40);not null;index:smsd_due"`
	UserID            uint              `json:"user_id" gorm:"column:user_id;not null;default:0;index:smsd_user_created"`
	BusinessType      string            `json:"business_type" gorm:"column:business_type;type:varchar(40);not null;default:''"`
	BusinessID        uint              `json:"business_id" gorm:"column:business_id;not null;default:0"`
	BusinessVersion   string            `json:"business_version" gorm:"column:business_version;type:varchar(80);not null;default:''"`
	Variables         string            `json:"variables" gorm:"column:variables;type:text"`
	RecipientMasked   string            `json:"recipient_masked" gorm:"column:recipient_masked;type:varchar(32);not null;default:''"`
	Status            string            `json:"status" gorm:"column:status;type:varchar(20);not null;default:'pending';index:smsd_due"`
	AttemptCount      uint              `json:"attempt_count" gorm:"column:attempt_count;not null;default:0"`
	MaxAttempts       uint              `json:"max_attempts" gorm:"column:max_attempts;not null;default:3"`
	NextAttemptAt     *utils.FormatDate `json:"next_attempt_at" gorm:"column:next_attempt_at;type:datetime;index:smsd_due"`
	LastAttemptAt     *utils.FormatDate `json:"last_attempt_at" gorm:"column:last_attempt_at;type:datetime"`
	AcceptedAt        *utils.FormatDate `json:"accepted_at" gorm:"column:accepted_at;type:datetime"`
	Provider          string            `json:"provider" gorm:"column:provider;type:varchar(40);not null;default:''"`
	ProviderRequestID string            `json:"provider_request_id" gorm:"column:provider_request_id;type:varchar(128);not null;default:''"`
	ProviderMessageID string            `json:"provider_message_id" gorm:"column:provider_message_id;type:varchar(128);not null;default:''"`
	ProviderCode      string            `json:"provider_code" gorm:"column:provider_code;type:varchar(80);not null;default:''"`
	LastError         string            `json:"last_error" gorm:"column:last_error;type:varchar(500);not null;default:''"`
}

func NewSMSDelivery() *SMSDelivery {
	return BindModel(&SMSDelivery{})
}

func (m *SMSDelivery) TableName() string {
	return "sms_deliveries"
}
