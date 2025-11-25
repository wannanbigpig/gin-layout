package form

// LoginLogList 登录日志列表查询表单
type AdminLoginLogList struct {
	Paginate
	Username    string `form:"username" json:"username" binding:"omitempty"`                   // 登录账号
	LoginStatus *int8  `form:"login_status" json:"login_status" binding:"omitempty,oneof=0 1"` // 登录状态：1=成功, 0=失败
	IP          string `form:"ip" json:"ip" binding:"omitempty"`                               // 登录IP
	StartTime   string `form:"start_time" json:"start_time" binding:"omitempty"`               // 开始时间
	EndTime     string `form:"end_time" json:"end_time" binding:"omitempty"`                   // 结束时间
}

// NewAdminLoginLogListQuery 创建登录日志列表查询表单
func NewAdminLoginLogListQuery() *AdminLoginLogList {
	return &AdminLoginLogList{}
}
