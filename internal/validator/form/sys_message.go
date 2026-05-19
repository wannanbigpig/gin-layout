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
