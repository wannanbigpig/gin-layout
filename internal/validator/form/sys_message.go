package form

// SysMessageList 表示系统消息列表查询参数。
type SysMessageList struct {
	Paginate
	Category string `form:"category" json:"category" binding:"omitempty"`
	IsRead   *uint8 `form:"is_read" json:"is_read" binding:"omitempty,oneof=0 1"`
}

func NewSysMessageListQuery() *SysMessageList {
	return &SysMessageList{}
}

// SysMessageRead 表示单条已读请求。
type SysMessageRead struct {
	ID uint `form:"id" json:"id" binding:"required,gt=0"`
}

func NewSysMessageReadForm() *SysMessageRead {
	return &SysMessageRead{}
}

// SysMessageReadAll 表示全部已读请求。
type SysMessageReadAll struct {
	Category string `form:"category" json:"category" binding:"omitempty"`
}

func NewSysMessageReadAllForm() *SysMessageReadAll {
	return &SysMessageReadAll{}
}

// SysMessageManageList 表示管理员通知管理列表查询参数。
type SysMessageManageList struct {
	Paginate
	Category      string `form:"category" json:"category" binding:"omitempty"`
	RecipientType string `form:"recipient_type" json:"recipient_type" binding:"omitempty,oneof=all user"`
	Keyword       string `form:"keyword" json:"keyword" binding:"omitempty"`
}

func NewSysMessageManageListQuery() *SysMessageManageList {
	return &SysMessageManageList{}
}

// SysMessageSend 表示管理员手动发送通知请求。
type SysMessageSend struct {
	Title        string `form:"title" json:"title" binding:"required" label:"消息标题"`
	Content      string `form:"message" json:"message" binding:"required" label:"消息内容"`
	Category     string `form:"category" json:"category" label:"消息分类" binding:"omitempty"`
	ActionURL    string `form:"action_url" json:"action_url" label:"操作链接" binding:"omitempty,max=500"`
	ActionLabel  string `form:"action_label" json:"action_label" label:"操作按钮文案" binding:"omitempty,max=100"`
	AudienceType string `form:"audience_type" json:"audience_type" label:"接收人类型" binding:"required,oneof=all user_ids"`
	UserIDs      []uint `form:"user_ids" json:"user_ids" label:"接收人ID列表" binding:"omitempty,dive,gt=0"`
}

func NewSysMessageSendForm() *SysMessageSend {
	return &SysMessageSend{}
}
