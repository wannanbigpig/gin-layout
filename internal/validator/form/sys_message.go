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
	Category        string `form:"category" json:"category" binding:"required"`
	Title           string `form:"title" json:"title" binding:"required"`
	Content         string `form:"content" json:"content" binding:"omitempty"`
	ActionURL       string `form:"action_url" json:"action_url" binding:"omitempty"`
	ActionLabel     string `form:"action_label" json:"action_label" binding:"omitempty"`
	RecipientType   string `form:"recipient_type" json:"recipient_type" binding:"required,oneof=all user"`
	RecipientIDs    []uint `form:"recipient_ids" json:"recipient_ids" binding:"omitempty,dive,gt=0"`
	SendAt          string `form:"send_at" json:"send_at" binding:"omitempty"`
	SendImmediately uint8  `form:"send_immediately" json:"send_immediately" binding:"required,oneof=0 1"`
}

func NewSysMessageSendForm() *SysMessageSend {
	return &SysMessageSend{
		SendImmediately: 1,
	}
}
