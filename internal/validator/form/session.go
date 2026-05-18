package form

// SessionList 在线会话列表查询参数。
type SessionList struct {
	Paginate
	UID       uint   `form:"uid" json:"uid" binding:"omitempty,gt=0"`
	Username  string `form:"username" json:"username" binding:"omitempty"`
	IP        string `form:"ip" json:"ip" binding:"omitempty"`
	IsRevoked *uint8 `form:"is_revoked" json:"is_revoked" binding:"omitempty,oneof=0 1"`
	StartTime string `form:"start_time" json:"start_time" binding:"omitempty"`
	EndTime   string `form:"end_time" json:"end_time" binding:"omitempty"`
}

func NewSessionListQuery() *SessionList {
	return &SessionList{}
}

// SessionRevoke 撤销在线会话参数。
type SessionRevoke struct {
	ID     uint   `form:"id" json:"id" binding:"required,gt=0"`
	Reason string `form:"reason" json:"reason" binding:"omitempty,max=255"`
}

func NewSessionRevokeForm() *SessionRevoke {
	return &SessionRevoke{}
}
