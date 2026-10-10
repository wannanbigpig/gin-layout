package form

// UserTodoList 表示个人待办列表查询。
type UserTodoList struct {
	Paginate
	ListSort
	Status     string `form:"status" json:"status" binding:"omitempty,oneof=pending processing completed canceled"`
	Priority   string `form:"priority" json:"priority" binding:"omitempty,oneof=low normal high urgent"`
	SourceType string `form:"source_type" json:"source_type" binding:"omitempty,max=40"`
	DueScope   string `form:"due_scope" json:"due_scope" binding:"omitempty,oneof=overdue today upcoming no_due"`
	Keyword    string `form:"keyword" json:"keyword" binding:"omitempty,max=100"`
	GroupID    *uint  `form:"group_id" json:"group_id" binding:"omitempty"`
}

func NewUserTodoListQuery() *UserTodoList {
	return &UserTodoList{}
}

type UserTodoSummaryQuery struct {
	GroupID *uint `form:"group_id" json:"group_id" binding:"omitempty"`
}

// UserTodoCreate 创建当前用户的手工待办。source_type 固定为 manual。
type UserTodoCreate struct {
	SourceKey   string `form:"source_key" json:"source_key" binding:"omitempty,max=128"`
	Title       string `form:"title" json:"title" binding:"required,max=255"`
	Description string `form:"description" json:"description" binding:"omitempty,max=5000"`
	Priority    string `form:"priority" json:"priority" binding:"omitempty,oneof=low normal high urgent"`
	ActionURL   string `form:"action_url" json:"action_url" binding:"omitempty,max=500"`
	DueAt       string `form:"due_at" json:"due_at" binding:"omitempty,max=35"`
	GroupID     uint   `form:"group_id" json:"group_id"`
}

func NewUserTodoCreateForm() *UserTodoCreate {
	return &UserTodoCreate{}
}

// UserTodoUpdate 只用于编辑当前用户自己的 manual 待办核心信息。
type UserTodoUpdate struct {
	ID          uint    `form:"id" json:"id" binding:"required,gt=0"`
	Title       *string `form:"title" json:"title" binding:"omitempty,min=1,max=255"`
	Description *string `form:"description" json:"description" binding:"omitempty,max=5000"`
	Priority    *string `form:"priority" json:"priority" binding:"omitempty,oneof=low normal high urgent"`
	ActionURL   *string `form:"action_url" json:"action_url" binding:"omitempty,max=500"`
	DueAt       *string `form:"due_at" json:"due_at" binding:"omitempty,max=35"`
	GroupID     *uint   `form:"group_id" json:"group_id"`
}

type UserTodoGroupCreate struct {
	Name string `form:"name" json:"name" binding:"required,max=40"`
}

type UserTodoGroupRename struct {
	ID   uint   `form:"id" json:"id" binding:"required,gt=0"`
	Name string `form:"name" json:"name" binding:"required,max=40"`
}

type UserTodoGroupID struct {
	ID uint `form:"id" json:"id" binding:"required,gt=0"`
}

type UserTodoMoveGroup struct {
	ID      uint `form:"id" json:"id" binding:"required,gt=0"`
	GroupID uint `form:"group_id" json:"group_id"`
}

func NewUserTodoUpdateForm() *UserTodoUpdate {
	return &UserTodoUpdate{}
}

type UserTodoStatus struct {
	ID     uint   `form:"id" json:"id" binding:"required,gt=0"`
	Status string `form:"status" json:"status" binding:"required,oneof=pending processing completed canceled"`
}

func NewUserTodoStatusForm() *UserTodoStatus {
	return &UserTodoStatus{}
}

type UserTodoDelete struct {
	ID uint `form:"id" json:"id" binding:"required,gt=0"`
}

func NewUserTodoDeleteForm() *UserTodoDelete {
	return &UserTodoDelete{}
}

// UserTodoReminderPreference 更新当前用户的待办提醒偏好。
type UserTodoReminderPreference struct {
	InAppEnabled *bool `form:"in_app_enabled" json:"in_app_enabled" binding:"required"`
	SMSEnabled   *bool `form:"sms_enabled" json:"sms_enabled" binding:"required"`
	LeadMinutes  *int  `form:"lead_minutes" json:"lead_minutes" binding:"omitempty,min=0,max=43200"`
}

func NewUserTodoReminderPreferenceForm() *UserTodoReminderPreference {
	return &UserTodoReminderPreference{}
}
