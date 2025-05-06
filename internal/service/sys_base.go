package service

type Base struct {
	adminUserId *uint
}

// SetAdminUserId 设置管理员ID
func (b Base) SetAdminUserId(userId *uint) {
	b.adminUserId = userId
}

// GetAdminUserId 获取管理员ID
func (b Base) GetAdminUserId() *uint {
	return b.adminUserId
}
