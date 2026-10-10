package model

const NotificationSceneTodoDue = "todo_due"

// NotificationPreference 保存用户对一个业务场景的渠道偏好。
type NotificationPreference struct {
	BaseModel
	UserID uint   `json:"user_id" gorm:"column:user_id;not null;default:0;uniqueIndex:np_user_scene"`
	Scene  string `json:"scene" gorm:"column:scene;type:varchar(40);not null;default:'';uniqueIndex:np_user_scene"`
	// 默认值由数据库迁移负责。模型层不能声明 default，否则 GORM Create 会把用户显式提交的 0
	// 当作零值替换为默认值，导致“关闭通知”实际仍保存为开启。
	InAppEnabled uint8  `json:"in_app_enabled" gorm:"column:in_app_enabled;not null"`
	SMSEnabled   uint8  `json:"sms_enabled" gorm:"column:sms_enabled;not null"`
	LeadMinutes  uint16 `json:"lead_minutes" gorm:"column:lead_minutes;not null"`
}

func NewNotificationPreference() *NotificationPreference {
	return BindModel(&NotificationPreference{})
}

func (m *NotificationPreference) TableName() string {
	return "notification_preferences"
}
